package worker

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/yourusername/dynamic-pipeline/pkg/types"
)

// startOne runs a single worker goroutine.  It ends when ctx or the worker-specific
// done channel is closed, or when jobs channel is closed AND empty.
func startOne(ctx context.Context, id int, jobs <-chan types.Job, results chan<- types.Result, done <-chan struct{}, wg *sync.WaitGroup, failureRate int) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		rand.Seed(time.Now().UnixNano() + int64(id))

		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case job, ok := <-jobs:
				if !ok {
					return // producer closed the channel
				}

				// Simulate processing time (100–500 ms)
				time.Sleep(time.Duration(100+rand.Intn(400)) * time.Millisecond)

				// Simulate failure
				var err error
				if rand.Intn(failureRate) == 0 { // e.g. failureRate = 10  → 10 % failures
					err = types.ErrJobFailed
					log.Printf("[worker %02d] job %d failed: %v", id, job.ID, err)
				}

				results <- types.Result{JobID: job.ID, WorkerID: id, Error: err}
			}
		}
	}()
}

// Spawn creates n workers and returns a slice of stop channels—one per worker—
// that the supervisor can close individually to remove workers on the fly.
func Spawn(ctx context.Context, n int, jobs <-chan types.Job, results chan<- types.Result, wg *sync.WaitGroup, failureRate int) []chan struct{} {
	stops := make([]chan struct{}, n)
	for i := 0; i < n; i++ {
		stop := make(chan struct{})
		startOne(ctx, i, jobs, results, stop, wg, failureRate)
		stops[i] = stop
	}
	return stops
}
