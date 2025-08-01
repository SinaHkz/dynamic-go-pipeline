package producer

import (
	"context"
	"math/rand"
	"time"

	"dynamic-pipeline/pkg/backlog"
	"dynamic-pipeline/pkg/types"
)

// Start generates jobs with a changeable frequency and updates backlog counter.
func Start(
	ctx context.Context,
	jobs chan<- types.Job,
	rateCh <-chan time.Duration,
	counter *backlog.Counter,
) {
	go func() {
		defer close(jobs)

		id := 0
		rand.Seed(time.Now().UnixNano())

		interval := 150 * time.Millisecond
		t := time.NewTimer(interval)

		for {
			select {
			case <-ctx.Done():
				return
			case newInt := <-rateCh:
				if !t.Stop() {
					<-t.C
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
