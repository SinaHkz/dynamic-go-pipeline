package types

import "errors"

var ErrJobFailed = errors.New("simulated worker failure")

type Job struct {
	ID   int
	Data interface{}
}

type Result struct {
	JobID    int
	WorkerID int
	Error    error
}
