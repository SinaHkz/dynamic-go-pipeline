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
	// ----- logging to console + file -----
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

	// ----- load config -----
	cfg, err := pipelineconfig.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	pc := cfg.Pipeline

	// ----- supervisor config -----
	supCfg := supervisor.Config{
		MinWorkers:      pc.MinWorkers,
		MaxWorkers:      pc.MaxWorkers,
		GrowThreshold:   pc.GrowThreshold,
		ShrinkThreshold: pc.ShrinkThreshold,
		CheckInterval:   pc.CheckInterval,
		FailureRate:     pc.FailureRate,
		ErrorWindow:     pc.ErrorWindow,
		ErrorThreshold:  pc.ErrorThreshold,
	}

	// ----- contexts -----
	root := context.Background()
	sigCtx, _ := shutdown.WithSignal(root)         // listens for Ctrl+C / SIGTERM
	prodCtx, cancelProd := context.WithCancel(root) // producer & load-generator only

	// ----- channels and shared state -----
	jobs := make(chan types.Job, 200)
	results := make(chan types.Result, 200)
	rateCh := make(chan time.Duration, 5) // buffered so rate changes don't block
	errSig := make(chan struct{}, 1024)   // collector → supervisor: each error
	notify := make(chan struct{}, 1)      // supervisor → main: request shutdown

	var wg sync.WaitGroup
	var counter backlog.Counter

	// ----- start components -----
	producer.Start(prodCtx, jobs, rateCh, &counter)
	collector.Start(root, results, errSig, &wg)
	sup := supervisor.New(supCfg, root, jobs, results, &wg, &counter, errSig, notify)
	sup.Start()

	// ----- dynamic load: alternate fast/slow rates -----
	rateCh <- 20 * time.Millisecond // prime with burst
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

	log.Println("pipeline running — press Ctrl+C to start graceful drain")

	// ----- wait for either OS signal or error-threshold trip -----
	var reason string
	select {
	case <-sigCtx.Done():
		reason = "signal"
	case <-notify:
		reason = "error-threshold"
	}
	log.Printf("shutdown requested by %s — stopping producer, beginning drain", reason)

	// ----- graceful drain sequence -----

	// 1) stop producer & load generator (no new jobs will be added)
	cancelProd()

	// Optional: overall drain timeout (set to >0 to enable)
	drainTimeout := 0 * time.Second // e.g., 30 * time.Second to enforce a cap
	var timeoutC <-chan time.Time
	if drainTimeout > 0 {
		t := time.NewTimer(drainTimeout)
		defer t.Stop()
		timeoutC = t.C
	}

	// 2) wait until backlog == 0, but remain responsive to Ctrl+C
	for {
		if counter.Load() == 0 {
			break
		}
		select {
		case <-sigCtx.Done():
			// second Ctrl+C: force shutdown
			log.Println("second signal received — forcing shutdown")
			goto FORCE
		case <-time.After(500 * time.Millisecond):
			log.Printf("draining backlog=%d", counter.Load())
		case <-timeoutC:
			log.Println("drain timeout reached — forcing shutdown")
			goto FORCE
		}
	}

	// 3) graceful close of jobs, then stop supervisor, then wait for collector
	log.Println("backlog is zero — closing jobs, stopping workers and supervisor")
	close(jobs)
	sup.Stop()
	wg.Wait()
	log.Println("graceful exit complete")
	return

FORCE:
	// forced path: close jobs and stop right away
	close(jobs)
	sup.Stop()
	wg.Wait()
	log.Println("forced exit complete")
}
