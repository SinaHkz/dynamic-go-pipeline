package supervisor

import (
	"context"
	"log"
	"sync"
	"time"

	"dynamic-pipeline/internal/worker"
	"dynamic-pipeline/pkg/backlog"
	"dynamic-pipeline/pkg/types"
)

// Config controls scaling and error-rate shutdown behavior.
type Config struct {
	MinWorkers      int
	MaxWorkers      int
	GrowThreshold   int
	ShrinkThreshold int
	CheckInterval   time.Duration
	FailureRate     int

	// Error-rate triggered graceful shutdown.
	ErrorWindow    time.Duration // sliding window length
	ErrorThreshold int           // trigger when >= this many errors in window
}

type Supervisor struct {
	cfg     Config
	ctx     context.Context
	cancel  context.CancelFunc
	jobs    chan types.Job
	results chan types.Result
	wg      *sync.WaitGroup
	counter *backlog.Counter

	// error-rate tracking
	errSig       <-chan struct{}
	errTimes     []time.Time
	notifiedOnce bool

	// worker pool management
	workers []chan struct{} // per-worker stop channels
	nextID  int             // unique worker ID counter

	// request shutdown notification to main (buffered size=1 recommended)
	shutdownCh chan<- struct{}
}

func New(
	cfg Config,
	parent context.Context,
	jobs chan types.Job,
	results chan types.Result,
	wg *sync.WaitGroup,
	counter *backlog.Counter,
	errSig <-chan struct{},
	shutdownCh chan<- struct{},
) *Supervisor {
	ctx, cancel := context.WithCancel(parent)
	return &Supervisor{
		cfg:        cfg,
		ctx:        ctx,
		cancel:     cancel,
		jobs:       jobs,
		results:    results,
		wg:         wg,
		counter:    counter,
		errSig:     errSig,
		shutdownCh: shutdownCh,
	}
}

func (s *Supervisor) Start() {
	// bootstrap minimum workers
	s.workers = worker.Spawn(
		s.ctx,
		s.nextID,
		s.cfg.MinWorkers,
		s.jobs,
		s.results,
		s.wg,
		s.cfg.FailureRate,
		s.counter,
	)
	s.nextID += s.cfg.MinWorkers

	ticker := time.NewTicker(s.cfg.CheckInterval)

	s.wg.Add(1)
	go func() {
		defer func() {
			close(s.results) // signal collector we're done when workers exit
			s.wg.Done()
		}()

		for {
			select {
			case <-s.ctx.Done():
				s.stopAll()
				return

			case <-ticker.C:
				s.scale()

			case <-s.errSig:
				// record error timestamp and evaluate immediately
				now := time.Now()
				s.errTimes = append(s.errTimes, now)
				s.scale() // immediate check so shutdown is prompt on threshold
			}
		}
	}()
}

func (s *Supervisor) Stop() { s.cancel() }

// scale performs two things:
//  1) error-window pruning and optional shutdown notify
//  2) backlog-based add/remove of workers
func (s *Supervisor) scale() {
	// ---- error-rate gate ----
	if s.cfg.ErrorThreshold > 0 && s.cfg.ErrorWindow > 0 {
		now := time.Now()
		cutoff := now.Add(-s.cfg.ErrorWindow)

		// prune old timestamps in-place
		n := 0
		for _, t := range s.errTimes {
			if t.After(cutoff) {
				s.errTimes[n] = t
				n++
			}
		}
		s.errTimes = s.errTimes[:n]

		if !s.notifiedOnce && len(s.errTimes) >= s.cfg.ErrorThreshold {
			log.Printf(
				"critical: %d errors within %s (threshold=%d) — requesting graceful shutdown",
				len(s.errTimes), s.cfg.ErrorWindow, s.cfg.ErrorThreshold,
			)
			s.notifiedOnce = true
			select {
			case s.shutdownCh <- struct{}{}:
			default:
			}
			// Do not Stop() here; main coordinates a proper drain.
		}
	}

	// ---- backlog-based scaling ----
	backlogSize := s.counter.Load()

	switch {
	case backlogSize >= int64(s.cfg.GrowThreshold) && len(s.workers) < s.cfg.MaxWorkers:
		s.add()
	case backlogSize <= int64(s.cfg.ShrinkThreshold) && len(s.workers) > s.cfg.MinWorkers:
		s.remove()
	}
}

func (s *Supervisor) add() {
	stop := worker.Spawn(
		s.ctx,
		s.nextID,
		1,
		s.jobs,
		s.results,
		s.wg,
		s.cfg.FailureRate,
		s.counter,
	)[0]
	s.nextID++
	s.workers = append(s.workers, stop)
	log.Printf("↑ added worker (total %d)", len(s.workers))
}

func (s *Supervisor) remove() {
	stop := s.workers[0]
	close(stop)
	s.workers = s.workers[1:]
	log.Printf("↓ removed worker (total %d)", len(s.workers))
}

func (s *Supervisor) stopAll() {
	for _, stop := range s.workers {
		close(stop)
	}
	s.workers = nil
}
