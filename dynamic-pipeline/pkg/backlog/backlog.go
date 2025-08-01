package backlog

import "sync/atomic"

// Counter is a threadsafe counter for job backlog.
type Counter struct {
	n int64
}

func (c *Counter) Inc() {
	atomic.AddInt64(&c.n, 1)
}

func (c *Counter) Dec() {
	atomic.AddInt64(&c.n, -1)
}

func (c *Counter) Load() int64 {
	return atomic.LoadInt64(&c.n)
}
