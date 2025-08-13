package producer

import (
	"context"
	"math/rand"
	"time"

	"dynamic-pipeline/pkg/backlog"
	"dynamic-pipeline/pkg/types"
)

// Start generates jobs with a changeable frequency and updates backlog counter.
// IMPORTANT: This function does NOT close(jobs). The owner (main) will close it
// after a graceful drain so retries in workers won't panic.
func Start(
	ctx context.Context,
	jobs chan<- types.Job,
	rateCh <-chan time.Duration,
	counter *backlog.Counter,
) {
	go func() {
		id := 0
		rand.Seed(time.Now().UnixNano())

		interval := 150 * time.Millisecond
		t := time.NewTimer(interval)

		for {
			select {
			case <-ctx.Done():
				// Stop producing immediately, but DO NOT close(jobs).
				// The main shutdown flow will drain and close safely.
				if !t.Stop() {
					select { case <-t.C: default: }
				}
				return

			case newInt := <-rateCh:
				if !t.Stop() {
					select { case <-t.C: default: }
				}
				interval = newInt
				t.Reset(interval)

			case <-t.C:
				jobs <- types.Job{ID: id}
				counter.Inc() // backlog++
				id++
				t.Reset(interval)
			}
		}
	}()
}
