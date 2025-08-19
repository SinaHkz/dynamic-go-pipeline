package collector

import (
	"context"
	"log"
	"sync"

	"dynamic-pipeline/pkg/types"
)

// Start prints/logs results. When a result has an error, it emits a signal
// to errSig so the Supervisor can track error rate.
//
// errSig should be a buffered channel (e.g., size 1024) to avoid blocking.
func Start(ctx context.Context, results <-chan types.Result, errSig chan<- struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case res, ok := <-results:
				if !ok {
					return
				}
				if res.Error != nil {
					log.Printf("✗ job %d handled by worker %02d, ERROR: %v",
						res.JobID, res.WorkerID, res.Error)
					// Non-blocking notify; drop if channel is full.
					select {
					case errSig <- struct{}{}:
					default:
					}
				} else {
					log.Printf("✓ job %d handled by worker %02d",
						res.JobID, res.WorkerID)
				}
			}
		}
	}()
}
