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

type Config struct {
	MinWorkers      int
	MaxWorkers      int
	GrowThreshold   int
	ShrinkThreshold int
	CheckInterval   time.Duration
	FailureRate     int
}

type Supervisor struct {
	cfg     Config
	ctx     context.Context
	cancel  context.CancelFunc
	jobs    chan types.Job
	results chan types.Result
	wg      *sync.WaitGroup
	counter *backlog.Counter

	mu      sync.Mutex
	workers []chan struct{}
	nextID  int
}

func New(cfg Config, parent context.Context,
	jobs chan types.Job, results chan types.Result,
	wg *sync.WaitGroup, counter *backlog.Counter) *Supervisor {

	ctx, cancel := context.WithCancel(parent)
	return &Supervisor{cfg: cfg, ctx: ctx, cancel: cancel,
		jobs: jobs, results: results, wg: wg, counter: counter}
}

func (s *Supervisor) Start() {
	s.workers = worker.Spawn(s.ctx, s.nextID, s.cfg.MinWorkers,
		s.jobs, s.results, s.wg, s.cfg.FailureRate, s.counter)
	s.nextID += s.cfg.MinWorkers

	ticker := time.NewTicker(s.cfg.CheckInterval)
	s.wg.Add(1)
	go func() {
		defer func() { close(s.results); s.wg.Done() }()
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
	backlog := s.counter.Load()

	switch {
	case backlog >= int64(s.cfg.GrowThreshold) &&
		len(s.workers) < s.cfg.MaxWorkers:
		s.add()
	case backlog <= int64(s.cfg.ShrinkThreshold) &&
		len(s.workers) > s.cfg.MinWorkers:
		s.remove()
	}
}

func (s *Supervisor) add() {
	stop := worker.Spawn(s.ctx, s.nextID, 1,
		s.jobs, s.results, s.wg, s.cfg.FailureRate, s.counter)[0]
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
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, stop := range s.workers {
		close(stop)
	}
	s.workers = nil
}
