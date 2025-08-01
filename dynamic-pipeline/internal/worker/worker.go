package worker

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"time"

	"dynamic-pipeline/pkg/types"
)

// startOne runs a single worker goroutine.
func startOne(
	ctx context.Context,
	id int,
	jobs <-chan types.Job,
	results chan<- types.Result,
	stop <-chan struct{},
	wg *sync.WaitGroup,
	failureRate int,
) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		rand.Seed(time.Now().UnixNano() + int64(id))

		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case job, ok := <-jobs:
				if !ok {
					return // channel closed
				}

				time.Sleep(time.Duration(100+rand.Intn(400)) * time.Millisecond) // 100–500 ms

				var err error
				if rand.Intn(failureRate) == 0 {
					err = types.ErrJobFailed
					log.Printf("[worker %02d] job %d failed: %v", id, job.ID, err)
				}

				results <- types.Result{JobID: job.ID, WorkerID: id, Error: err}
			}
		}
	}()
}

// Spawn creates n workers and returns their stop channels.
func Spawn(
	ctx context.Context,
	n int,
	jobs <-chan types.Job,
	results chan<- types.Result,
	wg *sync.WaitGroup,
	failureRate int,
) []chan struct{} {
	stops := make([]chan struct{}, n)
	for i := 0; i < n; i++ {
		stop := make(chan struct{})
		startOne(ctx, i, jobs, results, stop, wg, failureRate)
		stops[i] = stop
	}
	return stops
}
