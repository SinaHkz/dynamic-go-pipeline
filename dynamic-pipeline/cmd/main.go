package main

import (
	"context"
	"log"
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
	// Timestamped logs everywhere
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Load config
	cfg, err := pipelineconfig.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	pc := cfg.Pipeline

	// Supervisor config
	supCfg := supervisor.Config{
		MinWorkers:      pc.MinWorkers,
		MaxWorkers:      pc.MaxWorkers,
		GrowThreshold:   pc.GrowThreshold,
		ShrinkThreshold: pc.ShrinkThreshold,
		CheckInterval:   pc.CheckInterval,
		FailureRate:     pc.FailureRate,
	}

	root := context.Background()

	// Signal context: fires on Ctrl+C
	sigCtx, _ := shutdown.WithSignal(root)

	// Separate contexts:
	// - prodCtx: cancels producer & load generator ONLY
	// - sup/collector use their own lifecycle; they are stopped after drain
	prodCtx, cancelProd := context.WithCancel(root)

	jobs := make(chan types.Job, 200)
	results := make(chan types.Result, 200)
	rateCh := make(chan time.Duration, 5) // buffered so sends won't block

	var wg sync.WaitGroup
	var counter backlog.Counter

	// Start components
	producer.Start(prodCtx, jobs, rateCh, &counter)
	collector.Start(root, results, &wg) // will exit when results channel closes

	sup := supervisor.New(supCfg, root, jobs, results, &wg, &counter)
	sup.Start()

	// Dynamic load: alternate fast/slow rates (prime with a fast rate)
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

	// Wait for Ctrl+C
	<-sigCtx.Done()
	log.Println("🛑 signal received — stopping producer, beginning drain")

	// 1) Stop producer & load generator (no new jobs)
	cancelProd()

	// 2) Drain: wait until all queued/in-flight jobs finish
	//    (backlog counts both queued and in-progress)
	for {
		b := counter.Load()
		if b == 0 {
			break
		}
		log.Printf("…draining backlog=%d", b)
		time.Sleep(500 * time.Millisecond)
	}

	// 3) Now that no job can be added or is running, close jobs channel
	//    (safe for requeue logic too—nothing is in progress)
	close(jobs)
	log.Println("✅ jobs channel closed — stopping workers & supervisor")

	// 4) Ask supervisor to stop (it will close results when done)
	sup.Stop()

	// 5) Wait for collector (and any last result) to complete
	wg.Wait()
	log.Println("🎉 graceful exit complete")
}
