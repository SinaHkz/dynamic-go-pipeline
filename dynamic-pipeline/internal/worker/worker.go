package worker

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"time"

	"dynamic-pipeline/pkg/backlog"
	"dynamic-pipeline/pkg/types"
)

const maxRetries = 3 // adjust as you like

func startOne(
	ctx context.Context,
	id int,
	jobs chan types.Job,
	results chan<- types.Result,
	stop <-chan struct{},
	wg *sync.WaitGroup,
	failureRate int,
	counter *backlog.Counter,
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
					return
				}

				// Simulate processing time: 1–5 seconds
				time.Sleep(time.Duration(1+rand.Intn(5)) * time.Second)

				// Simulate a failure with probability 1/failureRate
				if rand.Intn(failureRate) == 0 {
					// Emit a failed result for THIS attempt so the error-rate sensor sees it
					log.Printf("[worker %02d] job %d attempt failed (retryCount=%d/%d)", id, job.ID, job.RetryCount, maxRetries)
					results <- types.Result{JobID: job.ID, WorkerID: id, Error: types.ErrJobFailed}

					// Retry path (do NOT decrement backlog)
					if job.RetryCount < maxRetries {
						job.RetryCount++
						// Requeue the job
						select {
						case <-ctx.Done():
							return
						case <-stop:
							return
						case jobs <- job:
							// requeued
						}
						continue
					}

					// Permanent failure (we have already emitted an error result above)
					log.Printf("[worker %02d] job %d permanently failed", id, job.ID)
					// Decrement backlog now that this job is concluded
					counter.Dec()
					continue
				}

				// Success path
				results <- types.Result{JobID: job.ID, WorkerID: id, Error: nil}
				counter.Dec()
			}
		}
	}()
}

func Spawn(
	ctx context.Context,
	startID, n int,
	jobs chan types.Job,
	results chan<- types.Result,
	wg *sync.WaitGroup,
	failureRate int,
	counter *backlog.Counter,
) []chan struct{} {
	stops := make([]chan struct{}, n)
	for i := 0; i < n; i++ {
		id := startID + i
		stop := make(chan struct{})
		startOne(ctx, id, jobs, results, stop, wg, failureRate, counter)
		stops[i] = stop
	}
	return stops
}
