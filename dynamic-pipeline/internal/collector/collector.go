package collector

import (
	"context"
	"fmt"
	"sync"

	"dynamic-pipeline/pkg/types"
)

// Start prints every result; exits when results channel closes or ctx cancels.
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
					return // channel closed
				}
				if res.Error != nil {
					fmt.Printf("✗ job %d handled by worker %d, ERROR: %v\n",
						res.JobID, res.WorkerID, res.Error)
				} else {
					fmt.Printf("✓ job %d handled by worker %d\n",
						res.JobID, res.WorkerID)
				}
			}
		}
	}()
}
