package supervisor

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/yourusername/dynamic-pipeline/internal/worker"
	"github.com/yourusername/dynamic-pipeline/pkg/types"
)

type Config struct {
	MinWorkers     int           // floor
	MaxWorkers     int           // ceiling
	GrowThreshold  int           // if len(jobs) >= this, add a worker (unless at Max)
	ShrinkThreshold int          // if len(jobs) <= this, remove a worker (unless at Min)
	CheckInterval  time.Duration // how often to inspect queue length
	FailureRate    int           // N: one out of every N jobs fails
}

type Supervisor struct {
	cfg       Config
	jobs      chan types.Job
	results   chan types.Result
	ctx       context.Context
	cancel    context.CancelFunc
	wg        *sync.WaitGroup
	workers   []chan struct{} // per-worker stop channels
	mu        sync.Mutex      // protects workers slice
}

func New(cfg Config, parent context.Context, jobs chan types.Job, results chan types.Result, wg *sync.WaitGroup) *Supervisor {
	ctx, cancel := context.WithCancel(parent)
	return &Supervisor{
		cfg:     cfg,
		jobs:    jobs,
		results: results,
		ctx:     ctx,
		cancel:  cancel,
		wg:      wg,
	}
}

func (s *Supervisor) Start() {
	// Start with MinWorkers
	s.workers = worker.Spawn(s.ctx, s.cfg.MinWorkers, s.jobs, s.results, s.wg, s.cfg.FailureRate)

	ticker := time.NewTicker(s.cfg.CheckInterval)
	s.wg.Add(1)
	go func() {
		defer func() {
			// Tell collector we’re done when all workers have exited.
			close(s.results)
			s.wg.Done()
		}()

		for {
			select {
			case <-s.ctx.Done():
				s.stopAll()
				return
			case <-ticker.C:
				s.scale()
			}
		}
	}()
}

func (s *Supervisor) Stop() { s.cancel() }

func (s *Supervisor) scale() {
	qLen := len(s.jobs)

	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	case qLen >= s.cfg.GrowThreshold && len(s.workers) < s.cfg.MaxWorkers:
		s.addWorker()
	case qLen <= s.cfg.ShrinkThreshold && len(s.workers) > s.cfg.MinWorkers:
		s.removeWorker()
	}
}

func (s *Supervisor) addWorker() {
	stopChans := worker.Spawn(s.ctx, 1, s.jobs, s.results, s.wg, s.cfg.FailureRate)
	s.workers = append(s.workers, stopChans[0])
	log.Printf("↑ added worker (total %d)", len(s.workers))
}

func (s *Supervisor) removeWorker() {
	// stop oldest worker
	stop := s.workers[0]
	close(stop)
	s.workers = s.workers[1:]
	log.Printf("↓ removed worker (total %d)", len(s.workers))
}

func (s *Supervisor) stopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, stop := range s.workers {
		close(stop)
	}
	s.workers = nil
}
