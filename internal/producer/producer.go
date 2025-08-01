package producer

import (
	"context"
	"math/rand"
	"time"

	"github.com/SinaHkz/dynamic-pipeline/pkg/types"
)

// Start launches a goroutine that keeps generating jobs until ctx is cancelled.
func Start(ctx context.Context, jobs chan<- types.Job) {
	go func() {
		defer close(jobs)

		id := 0
		rand.Seed(time.Now().UnixNano())

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Simulate variable production speed (0–200 ms)
				time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)

				job := types.Job{ID: id}
				jobs <- job
				id++
			}
		}
	}()
}
