package collector

import (
	"context"
	"fmt"
	"sync"

	"github.com/SinaHkz/dynamic-pipeline/pkg/types"
)

// Start prints every result.  When the results channel is closed and drained
// it signals through wg that it is finished.
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
					return // workers closed channel
				}

				if res.Error != nil {
					fmt.Printf("✗ job %d ⟶ handled by worker %02d, ERROR: %v\n", res.JobID, res.WorkerID, res.Error)
				} else {
					fmt.Printf("✓ job %d ⟶ handled by worker %02d\n", res.JobID, res.WorkerID)
				}
			}
		}
	}()
}
