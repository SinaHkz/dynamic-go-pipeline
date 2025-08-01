package types

import "errors"

// ErrJobFailed is returned by a worker to indicate
// that the simulated processing of a job failed.
var ErrJobFailed = errors.New("simulated worker failure")

// Job represents a single unit of work produced by the Producer
// and consumed by the Worker pool.
type Job struct {
	ID   int         // unique identifier
	Data interface{} // payload (use any type you like; nil for now)
}

// Result is emitted by each Worker after processing a Job.
// If Error is non-nil, the Collector can log it or aggregate metrics.
type Result struct {
	JobID    int   // original Job.ID
	WorkerID int   // ID of the worker that handled it
	Error    error // ErrJobFailed or another error; nil on success
}
