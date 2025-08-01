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

const maxRetries = 3 // NEW: max retry attempts per job

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

				time.Sleep(time.Duration(1+rand.Intn(5)) * time.Second)

				// Simulate failure
				if rand.Intn(failureRate) == 0 {
					if job.RetryCount < maxRetries {
						job.RetryCount++
						log.Printf("[worker %02d] job %d failed, retrying (%d/%d)", id, job.ID, job.RetryCount, maxRetries)
						jobs <- job // requeue the job
						continue    // do NOT decrement backlog yet
					} else {
						log.Printf("[worker %02d] job %d permanently failed", id, job.ID)
						results <- types.Result{JobID: job.ID, WorkerID: id, Error: types.ErrJobFailed}
						counter.Dec()
						continue
					}
				}

				// Successful case
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
