package main

import (
	"context"
	"log"
	"sync"

	pipelineconfig "github.com/SinaHkz/pipeline-config"
	"github.com/SinaHkz/dynamic-pipeline/internal/collector"
	"github.com/SinaHkz/dynamic-pipeline/internal/producer"
	"github.com/SinaHkz/dynamic-pipeline/internal/shutdown"
	"github.com/SinaHkz/dynamic-pipeline/internal/supervisor"
	"github.com/SinaHkz/dynamic-pipeline/pkg/types"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// ── load YAML from the helper module ────────────────────────────────
	cfgFile, err := pipelineconfig.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	pcfg := cfgFile.Pipeline                                    // shorthand

	// Map YAML → supervisor.Config
	supCfg := supervisor.Config{
		MinWorkers:      pcfg.MinWorkers,
		MaxWorkers:      pcfg.MaxWorkers,
		GrowThreshold:   pcfg.GrowThreshold,
		ShrinkThreshold: pcfg.ShrinkThreshold,
		CheckInterval:   pcfg.CheckInterval,
		FailureRate:     pcfg.FailureRate,
	}

	// ── normal pipeline bootstrapping ───────────────────────────────────
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
	<-ctx.Done()
	log.Println("🛑 shutdown signal received")

	sup.Stop()
	wg.Wait()
	log.Println("✅ graceful exit complete")
}
