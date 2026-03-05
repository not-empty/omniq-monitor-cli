# omniq-monitor-cli

Terminal UI (TUI) to **monitor and operate Omniq queues** via Redis: view counters, list jobs, retry/remove failed jobs, pause/unpause queues, and publish new jobs.

## Requirements

- Go (1.24.0)
- Redis access used by Omniq

## Setup

Create/edit `config.toml` (use `config.example.toml` as a base):

```toml
redis_url = "redis://localhost:6379"
interval_s = 0.5
auto_discover = true
discover_limit = 200
```


## How to Run

```bash
go run .
```

Or build:

```bash
go build -o omniq-monitoring .
./omniq-monitoring
```

## Shortcuts (TUI)

- **Ctrl+C**: quit

### Queues

- **q**: add/save a queue manually and select it
- **s**: scan queues in Redis (may cause high load)
- **l**: list saved queues and select one

Saved queues are stored in `queues.toml`.

### Jobs / Status

- **w**: view jobs in *waiting*
- **c**: view jobs in *completed*
- **f**: view jobs in *failed*

### Actions

- **a**: add a job to the selected queue (**payload must be valid JSON**)
- **p**: pause/unpause the selected queue
- **r**: retry a job (only when status = *failed*)
- **b**: batch retry listed jobs (only when status = *failed*)
- **x**: remove a job (current status lane)
- **d**: batch remove listed jobs (current status lane)
