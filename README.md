# Dynamic Concurrent Pipeline in Go

A resilient, auto-scaling, multi-stage pipeline in Go that adapts to changing workloads. It demonstrates concurrency, synchronization, resource management, process control, graceful shutdown, and robust error handling.

---

## Overview

This command-line application processes "jobs" through three stages:

1. **Producer** — generates jobs and pushes them into a bounded queue.
2. **Workers** — pull jobs, simulate 1–5 seconds of work, may fail and retry.
3. **Collector** — records results and errors with timestamps to a log file.

It scales workers up or down automatically based on a true backlog signal and performs a graceful, draining shutdown on Ctrl+C: the producer stops immediately, in-flight/queued jobs finish, the jobs channel is closed, and the system exits cleanly.

---

## Core Concepts

* **Concurrency**: goroutines and channels for parallel job production, processing, and collection.
* **Synchronization**: WaitGroups and atomic counters to coordinate goroutine lifecycles and backlog accounting.
* **Resource Management**: dynamic worker pool bounded by min/max limits.
* **Process Control**: signal handling for graceful startup/shutdown.
* **Error Handling**: simulated failures, bounded retries, and continued processing.

---

## Architecture

### Data Flow & Channels

* `jobs chan types.Job` — bounded queue of work units.
* `results chan types.Result` — worker outputs to the collector.
* `rateCh chan time.Duration` — producer’s rate-control channel to vary load in real time.

### Backlog (True Load Signal)

An atomic backlog counter tracks **queued + in-progress** jobs:

* Producer increments on job creation.
* Each worker decrements on completion (success or permanent failure).
* Supervisor scales based on `counter.Load()`.

### Dynamic Scaling (Supervisor)

Runs every `CheckInterval` and evaluates:

* **Scale Up**: if `backlog >= GrowThreshold` and `workers < MaxWorkers` → spawn one worker.
* **Scale Down**: if `backlog <= ShrinkThreshold` and `workers > MinWorkers` → stop one worker.

Workers receive globally unique IDs for observability.

### Worker Model

* Simulated processing time: **1–5 seconds** via `time.Sleep`.
* Failure simulation: approximately 1 out of every N jobs fails (configurable).
* **Retry policy**: failed jobs are requeued up to `maxRetries` (default 3). After that, failures are marked permanent and logged; the pipeline continues.

### Producer with Variable Rate

* Listens on `rateCh` to adjust the production interval at runtime (e.g., 20 ms bursts versus 600 ms idle).
* Produces workload swings that exercise auto-scaling.

### Graceful, Draining Shutdown

* On Ctrl+C (SIGINT) or SIGTERM, the application cancels the producer and load generator immediately so no new jobs are added.
* Wait until `backlog == 0` (all queued and in-flight jobs finished).
* Close the `jobs` channel, stop the supervisor (which stops workers and closes `results`), allow the collector to drain, and exit.

### Logging

* All packages use `log.Printf`.
* `cmd/main.go` directs output to **both stdout and `./logs/pipeline.log`** with microsecond timestamps and creates the `logs/` folder if missing.

### Configuration (Separate Module)

A second Go module, `pipeline-config`, embeds `config.yml` via `//go:embed` and exposes `Load()`.

`config.yml` keys:

```yaml
pipeline:
  min_workers:      2
  max_workers:      8
  grow_threshold:   50
  shrink_threshold: 10
  check_interval:   1s
  failure_rate:     10
```

---

## Project Structure

```
my-workspace/
├── pipeline-config/                # Separate module with embedded YAML config
│   ├── go.mod
│   ├── config.yml
│   └── config.go
└── dynamic-pipeline/               # Main module
    ├── go.mod
    ├── cmd/
    │   └── main.go                 # Wire-up, logging to ./logs/pipeline.log
    ├── internal/
    │   ├── collector/collector.go  # Prints/logs results
    │   ├── producer/producer.go    # Generates jobs at dynamic rate
    │   ├── shutdown/shutdown.go    # OS signal → context cancellation
    │   ├── supervisor/supervisor.go# Scaling logic, worker lifecycle
    │   └── worker/worker.go        # 1–5s work; retries; result emit
    └── pkg/
        ├── backlog/backlog.go      # Atomic backlog counter (pending+running)
        └── types/types.go          # Job/Result types & errors
```

---

## How to Run

Prerequisite: Go 1.21 or newer.

1. Initialize modules

```bash
cd pipeline-config
go mod tidy

cd ../dynamic-pipeline
# dynamic-pipeline/go.mod should contain:
# module dynamic-pipeline
# replace pipeline-config => ../pipeline-config
go mod tidy
```

2. Run the application

```bash
cd ../dynamic-pipeline
go run ./cmd
```

3. Observe behavior

* During execution, the built-in load generator alternates the producer rate to trigger scaling events.
* Logs are sent to console and to `logs/pipeline.log`. To tail the file:

```bash
tail -f logs/pipeline.log
```

4. Graceful shutdown

* Press Ctrl+C. The producer stops, the system drains all remaining work until backlog is zero, the jobs channel closes, the supervisor stops and closes results, and the collector exits.

5. Optional: build an executable

```bash
cd dynamic-pipeline
go build -o pipeline ./cmd
./pipeline
```

---

## Testing & Analysis (Sample Logs)

### Dynamic Scaling Up/Down

```
2025/08/13 10:00:00.000123 pipeline running — press Ctrl+C to start graceful drain
2025/08/13 10:00:10.101234 backlog size: 55
2025/08/13 10:00:10.101300 ↑ added worker (total 3)
2025/08/13 10:00:11.202345 backlog size: 78
2025/08/13 10:00:11.202410 ↑ added worker (total 4)
...
2025/08/13 10:00:40.501234 backlog size: 8
2025/08/13 10:00:40.501290 ↓ removed worker (total 3)
```

### Error Handling (Retries and Permanent Failures)

```
2025/08/13 10:02:15.000321 [worker 02] job 17 failed, retrying (1/3)
2025/08/13 10:02:17.100512 [worker 02] job 17 failed, retrying (2/3)
2025/08/13 10:02:19.200774 [worker 01] job 17 permanently failed
2025/08/13 10:02:19.200820 ✗ job 17 handled by worker 01, ERROR: simulated worker failure
```

### Graceful Shutdown (Draining)

```
2025/08/13 10:03:00.000000 signal received — stopping producer, beginning drain
2025/08/13 10:03:00.500250 draining backlog=41
2025/08/13 10:03:01.000417 draining backlog=29
2025/08/13 10:03:01.500588 draining backlog=15
2025/08/13 10:03:02.000745 draining backlog=0
2025/08/13 10:03:02.000900 jobs channel closed — stopping workers and supervisor
2025/08/13 10:03:02.001020 graceful exit complete
```

---

## Key Design Decisions

* **Backlog counter over `len(jobs)`** to capture queued and in-progress work and scale accurately.
* **Globally unique worker IDs** for clear observability and debugging.
* **Bounded retries** to handle transient failures without infinite loops.
* **Draining shutdown** separates halting production from stopping workers, ensuring no jobs are lost.
* **Configuration as a separate module** improves isolation, testability, and reproducibility.
