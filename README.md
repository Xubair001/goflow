# GoFlow

A production-style job queue and background worker system in Go — built as a learning project to
demonstrate real Go concurrency (goroutines, channels, worker pools, context cancellation, graceful
shutdown) end to end, not just syntax.

> **Status**: under active build. This README tracks what's actually implemented; see
> [CLAUDE.md](CLAUDE.md) for repo conventions and the build plan for the full milestone list.

## Architecture

```
                 ┌────────────────────┐
   HTTP clients →│   API Server (Go)  │→ writes job row → PostgreSQL (source of truth)
   React SPA     │  Gin + REST + SSE  │
                 └─────────┬──────────┘
                           │
                 ┌─────────▼──────────┐
                 │     Dispatcher      │  polls Postgres (SKIP LOCKED) for due/pending jobs,
                 │                     │  publishes job IDs to Redis Streams
                 └─────────┬──────────┘
                           │
                 ┌─────────▼──────────┐
                 │   Redis Streams     │  consumer group "workers"
                 └───┬─────────┬──────┘
                     │         │
              ┌──────▼──┐ ┌────▼────┐
              │ Worker 1 │ │ Worker N│  XREADGROUP → load job from Postgres → execute handler
              └──────────┘ └─────────┘  → write result/status to Postgres → XACK
```

**Postgres is the source of truth**; Redis Streams is the fast delivery layer. A reconciler
(part of the dispatcher) reclaims stream entries abandoned by dead workers and re-dispatches any
Postgres row stuck `running` past its lease — so a crashed worker never silently loses a job.

## Stack

Go 1.26, PostgreSQL (`pgx`), Redis Streams (`go-redis`), Gin router, `log/slog`, Prometheus metrics,
React + Vite + TypeScript + Tailwind for the dashboard, Docker Compose for local orchestration.

## Quick start

```bash
docker compose up -d          # postgres + redis
cd backend && make migrate-up # apply schema
make build && make run-apiserver
```

(Full instructions — including running workers/dispatcher and the dashboard — land as those pieces
are built; see the milestone plan.)

## Development

```bash
cd backend
make test              # unit tests (-race)
make test-integration  # integration tests against real Postgres/Redis (needs Docker)
make lint               # golangci-lint
```

## Project layout

See [CLAUDE.md](CLAUDE.md#repo-layout) for the annotated directory tree.

## License

MIT
