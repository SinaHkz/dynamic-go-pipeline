package collector

import (
	"context"
	"log"
	"sync"

	"dynamic-pipeline/pkg/types"
)

func Start(ctx context.Context, results <-chan types.Result, wg *sync.WaitGroup) {
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
					log.Printf("✗ job %d handled by worker %d, ERROR: %v",
						res.JobID, res.WorkerID, res.Error)
				} else {
					log.Printf("✓ job %d handled by worker %d",
						res.JobID, res.WorkerID)
				}
			}
		}
	}()
}
