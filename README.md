# GoFlow

A production-style job queue and background worker system in Go — built as a learning project to
demonstrate real Go concurrency (goroutines, channels, worker pools, context cancellation, graceful
shutdown, retries/backoff) end to end, not just syntax.

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
git clone git@github.com:Xubair001/goflow.git && cd goflow
docker compose up --build
```

Open **http://localhost:8080** for the dashboard, **http://localhost:8080/docs** for the API
reference, and **http://localhost:8025** for Mailpit (every `send_email` job lands there instead of
anywhere real).

If any of the default host ports (5432, 6379, 8025, 1025, 8080) are already taken on your machine,
copy `.env.example` to `.env` and change them there — nothing else needs to change, since
inter-service traffic uses the compose network regardless of the host mapping.

To scale workers: `docker compose up --build --scale worker=3`.

## Local development (without Docker)

Useful when iterating on the Go services or the dashboard directly, with hot reload:

```bash
docker compose up -d postgres redis mailpit   # just the dependencies
cd backend
make migrate-up
make build
./bin/dispatcher & ./bin/worker & ./bin/apiserver &

cd ../frontend
npm install
npm run dev   # http://localhost:5173, proxies /api etc. to :8080
```

## Testing

```bash
cd backend
make test              # unit tests, -race
make test-integration  # real Postgres/Redis via testcontainers-go (needs Docker)
make lint               # golangci-lint

cd ../frontend
npx tsc -b && npx oxlint .
```

## Project layout

See [CLAUDE.md](CLAUDE.md#repo-layout) for the annotated directory tree, and the rest of that file
for the conventions (error handling, concurrency, naming, testing) enforced throughout.

## License

MIT
