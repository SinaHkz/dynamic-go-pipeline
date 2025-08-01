package shutdown

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// WithSignal returns a child context cancelled on Ctrl+C or parent cancel.
func WithSignal(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-parent.Done():
			cancel()
		case <-c:
			cancel()
		}
	}()

	return ctx, cancel
}
