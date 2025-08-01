package shutdown

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// WithSignal returns a child context that is cancelled when the user hits Ctrl+C
// or the parent is done.  Call cancelFunc to clean up manually if needed.
func WithSignal(parent context.Context) (ctx context.Context, cancel context.CancelFunc) {
	ctx, cancel = context.WithCancel(parent)

	// Relay OS interrupt/terminate to the context.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-parent.Done():
			cancel()
		case <-ch:
			cancel()
		}
	}()

	return ctx, cancel
}
