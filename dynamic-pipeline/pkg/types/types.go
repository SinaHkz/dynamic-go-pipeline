package types

import "errors"

var ErrJobFailed = errors.New("simulated worker failure")

type Job struct {
	ID   int
	Data interface{}
	RetryCount int // number of retries for this job
}

type Result struct {
	JobID    int
	WorkerID int
	Error    error
}
