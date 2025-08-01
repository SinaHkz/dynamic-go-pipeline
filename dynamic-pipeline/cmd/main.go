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
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg, err := pipelineconfig.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	pc := cfg.Pipeline

	supCfg := supervisor.Config{
		MinWorkers:      pc.MinWorkers,
		MaxWorkers:      pc.MaxWorkers,
		GrowThreshold:   pc.GrowThreshold,
		ShrinkThreshold: pc.ShrinkThreshold,
		CheckInterval:   pc.CheckInterval,
		FailureRate:     pc.FailureRate,
	}

	ctx, cancel := shutdown.WithSignal(context.Background())
	defer cancel()

	jobs    := make(chan types.Job, 200)
	results := make(chan types.Result, 200)
	rateCh  := make(chan time.Duration, 1)
	var wg sync.WaitGroup
	var counter backlog.Counter

	producer.Start(ctx, jobs, rateCh, &counter)
	collector.Start(ctx, results, &wg)
	sup := supervisor.New(supCfg, ctx, jobs, results, &wg, &counter)
	sup.Start()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				rateCh <- 100 * time.Millisecond
				time.Sleep(10 * time.Second)

				rateCh <- 1000 * time.Millisecond
				time.Sleep(30 * time.Second)
			}
		}
	}()

	log.Println("🚀 pipeline running — press Ctrl+C to stop")
	<-ctx.Done()
	log.Println("🛑 shutdown signal received")
	sup.Stop()
	wg.Wait()
	log.Println("✅ graceful exit complete")
}
