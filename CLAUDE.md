# CLAUDE.md

Guidance for Claude Code (or any future contributor) working in this repository.

## What this is

A production-style job queue / background worker system, built to demonstrate real Go concurrency
(goroutines, channels, worker pools, context cancellation, graceful shutdown, retries/backoff)
rather than just Go syntax. See `README.md` for the user-facing overview.

The system is feature-complete: three Go binaries (`apiserver`, `dispatcher`, `worker`), a REST API
with SSE live updates, six job type handlers, a React dashboard embedded into `apiserver`, Prometheus
metrics on all three processes, and a full `docker compose up` deployment. Ongoing work is normal
iteration from here, not scaffolding.

## Architecture

- `apiserver` — REST API (chi) + SSE live updates + serves the built React SPA. Writes job rows to
  Postgres; never talks to Redis directly.
- `dispatcher` — polls Postgres for due/pending jobs using `SELECT ... FOR UPDATE SKIP LOCKED`,
  publishes claimed jobs to a Redis Stream. Also runs the reconciler, which resets Postgres rows
  stuck `queued`/`running` past their lease back to `pending`.
- `worker` — horizontally scalable pool that reads from the Redis Stream via a consumer group
  (`XREADGROUP`), periodically reclaims entries abandoned by dead peers (`XAUTOCLAIM`), loads the
  full job from Postgres, executes the registered `Handler` for that job type, writes the
  result/status back to Postgres, then `XACK`s.
- **Postgres is the source of truth.** Redis is a delivery mechanism, not a store of record — if in
  doubt about job state, trust Postgres.

## Repo layout

```
backend/
  cmd/{apiserver,worker,dispatcher}/main.go   thin entrypoints: wire config → deps → run
  internal/
    config/      env-based config structs (one per binary, no framework)
    logging/     log/slog setup (JSON in production, text in development)
    job/         domain types: Job, Status, Handler interface, handler Registry
    store/       Store interface + Postgres implementation (pgx)
    queue/       Queue interface + Redis Streams implementation
    worker/      pool, retry/backoff-with-jitter, graceful shutdown
    dispatcher/  dispatch loop + reconciler
    handlers/    job type implementations (email, imageresize, csv, httprequest, report, scheduled)
    api/         chi router, HTTP handlers, middleware, error envelope, SSE broadcaster
    metrics/     Prometheus collectors + the worker/dispatcher metrics HTTP server
    web/         go:embed of the built frontend
  migrations/    golang-migrate SQL files
  Dockerfile.{apiserver,worker,dispatcher}   multi-stage builds -> distroless
frontend/
  src/api/         REST client, TypeScript types mirroring the Go JSON shapes, the SSE hook
  src/components/  StatsRow, JobTable, JobDetail, JobForm, StatusBadge
docker-compose.yml   full stack: postgres, redis, mailpit, migrate (one-shot), apiserver, dispatcher, worker
.env.example         host port overrides (copy to .env if a default port is already taken)
```

Nothing under `backend/` is meant to be imported by other modules, so it all lives under
`internal/` — no `pkg/`.

## Commands

```
cd backend
make build              # build all three binaries into backend/bin/
make test               # unit tests, -race
make test-integration   # integration tests (testcontainers-go, needs Docker), tag: integration
make lint               # golangci-lint run ./...
make fmt                # gofumpt + goimports
make dev-up             # docker compose up -d postgres redis mailpit -- for running Go binaries on the host
make migrate-new name=X # create a new migration pair
make migrate-up         # apply migrations against DB_URL
make frontend-build     # npm ci && npm run build; output lands in internal/web/dist for go:embed

cd frontend
npm run dev             # Vite dev server on :5173, proxies /api etc. to :8080
npx tsc -b && npx oxlint .   # typecheck + lint
```

From the repo root: `docker compose up --build` runs the full containerized stack (all three Go
services built from source, plus their dependencies) -- that's the one to reach for to run
everything at once, not `make dev-up` (which only brings up dependencies, for running the Go
binaries directly on the host during development).

## Conventions

- **Errors**: `fmt.Errorf("...: %w", err)` when callers should `errors.Is/As` the cause; package-level
  `Err*` sentinels for conditions callers branch on; handle an error once (don't log-and-return the
  same one); no `panic` outside a top-level HTTP recover middleware.
- **Concurrency**: every non-trivial function takes `context.Context` as its first parameter.
  Long-running processes shut down via `signal.NotifyContext` + a bounded shutdown timeout +
  `sync.WaitGroup`, draining in-flight work instead of being killed mid-job. Blocking reads (e.g. the
  worker's Redis consume loop) use a bounded timeout rather than an infinite block, even under
  context cancellation -- see the worker pool's `ConsumeBlock` comment for why relying on
  cancellation alone left a real shutdown hang.
- **Naming**: short lowercase package names, no stutter (`job.Job`, not `job.JobStruct`), interfaces
  named for behavior (`Store`, `Queue`, `Handler`). Exported identifiers get a doc comment starting
  with their own name.
- **Testing**: unit tests colocated and table-driven, using small hand-written fakes for `Store`/
  `Queue` rather than a mocking library. Integration tests live behind `//go:build integration` and
  use `testcontainers-go` against real Postgres/Redis -- don't mock the database.
- Frontend: TypeScript throughout, no client-side router (the dashboard is one page with local view
  state -- there was never a second route to justify one). Status colors follow the dataviz skill's
  validated palette; see `StatusBadge.tsx`.
- Enforced by `golangci-lint` (`backend/.golangci.yml`) and `oxlint` (`frontend/.oxlintrc.json`), not
  just written down here -- if a rule matters, it should be a lint failure, not a comment in this
  file.

## Gotchas

- `make test-integration` needs a working Docker daemon (testcontainers spins up real Postgres/Redis
  containers) -- it will hang or fail without one.
- Migrations are applied via the `golang-migrate` CLI, not custom Go code -- install it separately
  (see its releases page) if `make migrate-up` fails with "command not found". Inside
  `docker compose up`, the `migrate` service applies them once and exits; apiserver/dispatcher/worker
  wait on it via `service_completed_successfully`, not a fixed sleep.
- The `apiserver` embeds the frontend's built assets (`embed.FS`) for production; `internal/web/dist`
  ships with a placeholder page committed to git so a fresh clone always builds, and
  `make frontend-build` regenerates the real app before a production `make build`. In local dev, run
  the Vite dev server separately instead of rebuilding the Go binary on every frontend change.
- **`Dockerfile.apiserver`'s frontend-build stage must stay on a glibc base (`node:20-bookworm-slim`),
  not `-alpine`.** Vite's Rolldown bundler ships native platform bindings, and the musl variant isn't
  reliably resolvable via `npm ci` against a lockfile generated on a glibc dev machine -- this broke
  the image build for real, not hypothetically. The stage is discarded after `npm run build`, so the
  larger base costs nothing in the final image.
- The `send_email` handler points at `mailpit:1025` inside compose (`localhost:1025` outside it) --
  open http://localhost:8025 to see mail workers "send" instead of it going anywhere real.
- `worker`/`dispatcher` expose `/metrics` + `/healthz` on `:9091`/`:9092` respectively, not published
  to the host in `docker-compose.yml` (a fixed host port would collide under
  `docker compose up --scale worker=N`); `apiserver` folds its own `/metrics` into the main API port.
- Default host ports (5432/6379/8025/1025/8080) are overridable via `.env` (see `.env.example`) --
  reach for that instead of editing `docker-compose.yml` if one collides with something already
  running on your machine.
