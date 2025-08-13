package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	pipelineconfig "pipeline-config"

	"dynamic-pipeline/internal/collector"
	"dynamic-pipeline/internal/producer"
	"dynamic-pipeline/internal/shutdown"
	"dynamic-pipeline/internal/supervisor"
	"dynamic-pipeline/pkg/backlog"
	"dynamic-pipeline/pkg/types"
)

func main() {
	// 1) Prepare logging: create logs/ dir, open file, and tee logs to console + file.
	if err := os.MkdirAll("logs", 0o755); err != nil {
		log.Fatalf("failed to create logs directory: %v", err)
	}
	logPath := filepath.Join("logs", "pipeline.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Fatalf("failed to open log file %s: %v", logPath, err)
	}
	defer logFile.Close()

	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("logging to %s", logPath)

	// 2) Load config (from pipeline-config module)
	cfg, err := pipelineconfig.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	pc := cfg.Pipeline

	// 3) Supervisor config
	supCfg := supervisor.Config{
		MinWorkers:      pc.MinWorkers,
		MaxWorkers:      pc.MaxWorkers,
		GrowThreshold:   pc.GrowThreshold,
		ShrinkThreshold: pc.ShrinkThreshold,
		CheckInterval:   pc.CheckInterval,
		FailureRate:     pc.FailureRate,
	}

	// 4) Contexts: signal-driven shutdown, and producer-only cancel for graceful drain
	root := context.Background()
	sigCtx, _ := shutdown.WithSignal(root)     // fires on Ctrl+C (SIGINT) / SIGTERM
	prodCtx, cancelProd := context.WithCancel(root) // stops producer + load generator only

	// 5) Channels and shared state
	jobs := make(chan types.Job, 200)
	results := make(chan types.Result, 200)
	rateCh := make(chan time.Duration, 5) // buffered so rate changes don't block
	var wg sync.WaitGroup
	var counter backlog.Counter            // atomic backlog counter

	// 6) Start components
	producer.Start(prodCtx, jobs, rateCh, &counter)
	collector.Start(root, results, &wg)
	sup := supervisor.New(supCfg, root, jobs, results, &wg, &counter)
	sup.Start()

	// 7) Dynamic load: alternate fast/slow rates (prime with fast)
	rateCh <- 20 * time.Millisecond
	go func() {
		for {
			select {
			case <-prodCtx.Done():
				return
			default:
				rateCh <- 20 * time.Millisecond  // burst
				time.Sleep(30 * time.Second)
				rateCh <- 600 * time.Millisecond // idle
				time.Sleep(30 * time.Second)
			}
		}
	}()

	log.Println("🚀 pipeline running — press Ctrl+C to start graceful drain")

	// 8) Wait for Ctrl+C, then graceful drain:
	<-sigCtx.Done()
	log.Println("🛑 signal received — stopping producer, beginning drain")

	// Stop producer & load generator (no new jobs will be added)
	cancelProd()

	// Drain: wait until backlog == 0 (queued + in-flight jobs)
	for {
		b := counter.Load()
		if b == 0 {
			break
		}
		log.Printf("…draining backlog=%d", b)
		time.Sleep(500 * time.Millisecond)
	}

	// Close jobs channel now that nothing is in progress or being added
	close(jobs)
	log.Println("--------------------------------------------------   jobs channel closed — stopping workers & supervisor")

	// Stop supervisor (it will stop workers and then close results)
	sup.Stop()

	// Wait for collector to flush and exit
	wg.Wait()
	log.Println("--------------------------------------------------   graceful exit complete
}
