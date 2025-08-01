package main

import (
	"context"
	"log"
	"sync"

	pipelineconfig "pipeline-config"

	"dynamic-pipeline/internal/collector"
	"dynamic-pipeline/internal/producer"
	"dynamic-pipeline/internal/shutdown"
	"dynamic-pipeline/internal/supervisor"
	"dynamic-pipeline/pkg/types"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Load parameters from pipeline-config module.
	cfg, err := pipelineconfig.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	pc := cfg.Pipeline

	// Map YAML to supervisor.Config.
	supCfg := supervisor.Config{
		MinWorkers:      pc.MinWorkers,
		MaxWorkers:      pc.MaxWorkers,
		GrowThreshold:   pc.GrowThreshold,
		ShrinkThreshold: pc.ShrinkThreshold,
		CheckInterval:   pc.CheckInterval,
		FailureRate:     pc.FailureRate,
	}

	root := context.Background()
	ctx, cancel := shutdown.WithSignal(root)
	defer cancel()

	jobs    := make(chan types.Job, 100)
	results := make(chan types.Result, 100)

	var wg sync.WaitGroup
	producer.Start(ctx, jobs)
	collector.Start(ctx, results, &wg)

	sup := supervisor.New(supCfg, ctx, jobs, results, &wg)
	sup.Start()

	log.Println("🚀 pipeline running — press Ctrl+C to stop")
	<-ctx.Done()               // wait for interrupt
	log.Println("🛑 shutdown signal received")

	sup.Stop()                 // ask supervisor to halt workers
	wg.Wait()                  // wait for collector & workers to drain
	log.Println("✅ graceful exit complete")
}
