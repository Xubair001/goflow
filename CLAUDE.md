# CLAUDE.md

Guidance for Claude Code (or any future contributor) working in this repository.

## What this is

A production-style job queue / background worker system, built to demonstrate real Go concurrency
(goroutines, channels, worker pools, context cancellation, graceful shutdown) rather than just Go
syntax. See `README.md` for the user-facing overview and `/home/abdullah-zubair/.claude/plans/breezy-mixing-otter.md`
for the original build plan and milestone breakdown.

**Status**: under active initial build, milestone by milestone (see plan file for the list). This
file will be expanded as each milestone lands — treat sections below as accurate for what currently
exists, not aspirational.

## Architecture (target end-state)

- `apiserver` — REST API (Gin) + SSE live updates + serves the built React SPA. Writes job rows to
  Postgres; never talks to Redis directly.
- `dispatcher` — polls Postgres for due/pending jobs using `SELECT ... FOR UPDATE SKIP LOCKED`,
  publishes claimed jobs to a Redis Stream. Also runs the reconciler: `XAUTOCLAIM`s idle pending
  stream entries and re-dispatches Postgres rows stuck `running` past their lease.
- `worker` — horizontally scalable pool that reads from the Redis Stream via a consumer group,
  loads the full job from Postgres, executes the registered `Handler` for that job type, writes the
  result/status back to Postgres, then `XACK`s.
- **Postgres is the source of truth.** Redis is a delivery mechanism, not a store of record — if in
  doubt about job state, trust Postgres.

## Repo layout

```
backend/
  cmd/{apiserver,worker,dispatcher}/main.go   thin entrypoints: wire config → deps → run
  internal/
    config/      env-based config structs (one per binary, no framework)
    logging/     log/slog setup
    job/         domain types: Job, Status, Handler interface, handler Registry
    store/       Store interface + Postgres implementation (pgx)
    queue/       Queue interface + Redis Streams implementation
    worker/      pool, retry/backoff-with-jitter, graceful shutdown
    dispatcher/  dispatch loop + reconciler
    handlers/    job type implementations (email, imageresize, csv, report, httpcall, scheduled)
    api/         Gin router, HTTP handlers, middleware, error envelope, SSE broadcaster
    metrics/     Prometheus collectors
  migrations/    golang-migrate SQL files
  Dockerfile.{apiserver,worker,dispatcher}   multi-stage builds -> distroless
frontend/        React + Vite + TypeScript + Tailwind admin dashboard
docker-compose.yml   full local stack: postgres, redis, mailpit, migrate (one-shot), apiserver, dispatcher, worker
```

Nothing here is meant to be imported by other modules, so everything backend lives under
`internal/` — no `pkg/`.

## Commands

```
make build              # build all three binaries into backend/bin/
make test               # unit tests, -race
make test-integration   # integration tests (testcontainers-go, needs Docker), tag: integration
make lint               # golangci-lint run ./...
make fmt                # gofumpt + goimports
make dev-up             # docker compose up -d (postgres, redis only -- for local dev against `go run`)
make migrate-new name=X # create a new migration pair
make migrate-up         # apply migrations against DB_URL
make frontend-build     # npm ci && npm run build, output lands in internal/web/dist for go:embed
```

For the full containerized stack (apiserver + dispatcher + worker + their dependencies, all built from
source), run `docker compose up --build` from the repo root, not `make dev-up` (that's the lighter
dependencies-only stack for running the Go binaries directly on the host during development).

Run these from `backend/`, or use the Makefile targets from the repo root once one exists there too.

## Conventions

- **Errors**: `fmt.Errorf("...: %w", err)` when callers should `errors.Is/As` the cause; package-level
  `Err*` sentinels for conditions callers branch on; handle an error once (don't log-and-return the
  same one); no `panic` outside a top-level HTTP recover middleware.
- **Concurrency**: every non-trivial function takes `context.Context` as its first parameter.
  Long-running processes shut down via `signal.NotifyContext` + a bounded shutdown timeout +
  `sync.WaitGroup`, draining in-flight work instead of being killed mid-job.
- **Naming**: short lowercase package names, no stutter (`job.Job`, not `job.JobStruct`), interfaces
  named for behavior (`Store`, `Queue`, `Handler`). Exported identifiers get a doc comment starting
  with their own name.
- **Testing**: unit tests colocated and table-driven. Integration tests live behind
  `//go:build integration` and use `testcontainers-go` against real Postgres/Redis — don't mock the
  database.
- Enforced by `golangci-lint` (`backend/.golangci.yml`), not just written down here — if a rule
  matters, it should be a lint failure, not a comment in this file.

## Gotchas

- `make test-integration` needs a working Docker daemon (testcontainers spins up real Postgres/Redis
  containers) — it will hang or fail without one.
- Migrations are applied via the `golang-migrate` CLI, not custom Go code — install it separately
  (`brew install golang-migrate` / see their releases page) if `make migrate-up` fails with
  "command not found".
- The `apiserver` embeds the frontend's built assets (`embed.FS`) for production; in local dev, run
  the Vite dev server separately and let it proxy API calls instead of rebuilding the Go binary on
  every frontend change.
- `docker compose up`'s `migrate` service applies migrations once and exits; apiserver/dispatcher/
  worker wait on it via `service_completed_successfully`, not a fixed sleep.
- The `send_email` handler defaults to `mailpit:1025` inside compose (`localhost:1025` outside it) --
  open http://localhost:8025 to see mail workers "send" instead of it going anywhere real. Set
  `SMTP_ADDR`/`SMTP_FROM`/`SMTP_USERNAME`/`SMTP_PASSWORD` in `.env` (see `.env.example`) to point it at
  a real provider's SMTP relay instead -- no code changes needed, since `net/smtp.SendMail` negotiates
  STARTTLS automatically against any provider on the standard port 587.
- `worker`/`dispatcher` expose `/metrics` + `/healthz` on `:9091`/`:9092` respectively (not published
  to the host in `docker-compose.yml`, since a fixed host port would collide under
  `docker compose up --scale worker=N`); `apiserver` folds its own `/metrics` into the main API port.
