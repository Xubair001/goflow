//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/abdullah-zubair/jobqueue/internal/job"
	"github.com/abdullah-zubair/jobqueue/internal/store"
)

// testPool is shared across every test in this file: starting one Postgres
// container is far cheaper than one per test, and newStore truncates the
// table before each test to keep them isolated.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("jobqueue_test"),
		tcpostgres.WithUsername("jobqueue"),
		tcpostgres.WithPassword("jobqueue"),
	)
	if err != nil {
		log.Fatalf("start postgres container: %v", err)
	}
	defer func() { _ = container.Terminate(ctx) }()

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("build connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("connect to test database: %v", err)
	}
	defer pool.Close()

	// The postgres image restarts its server process once after running
	// first-time init (applying POSTGRES_USER/PASSWORD/DB), so the port can
	// accept and then immediately drop a connection right after the
	// container's own readiness check passes. Wait for a real query to
	// succeed before trusting the pool.
	if err := waitForReady(ctx, pool, 30*time.Second); err != nil {
		log.Fatalf("wait for postgres ready: %v", err)
	}

	if err := applyMigrations(ctx, pool); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	testPool = pool
	return m.Run()
}

// waitForReady polls the pool with a real query until it succeeds or
// timeout elapses, absorbing the connection resets that happen while
// Postgres is mid-restart right after container startup.
func waitForReady(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if lastErr = pool.Ping(ctx); lastErr == nil {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("no successful ping within %s: %w", timeout, lastErr)
}

// applyMigrations runs every *.up.sql file in backend/migrations, in order,
// directly through the pool. It intentionally doesn't shell out to the
// golang-migrate CLI so these tests only need Docker, not a separately
// installed tool.
func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		contents, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(contents)); err != nil {
			return err
		}
	}
	return nil
}

// newStore truncates the jobs table and returns a Store backed by the
// shared test container, giving each test a clean slate.
func newStore(t *testing.T) *store.Postgres {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), "TRUNCATE jobs"); err != nil {
		t.Fatalf("truncate jobs: %v", err)
	}
	return store.NewPostgres(testPool)
}

// assertJSONEqual compares two JSON documents semantically rather than
// byte-for-byte: Postgres's JSONB type reformats JSON on storage (e.g. adds
// a space after ":"), so exact string comparison would be the wrong check.
func assertJSONEqual(t *testing.T, got, want json.RawMessage) {
	t.Helper()
	var gotVal, wantVal any
	if err := json.Unmarshal(got, &gotVal); err != nil {
		t.Fatalf("unmarshal got %s: %v", got, err)
	}
	if err := json.Unmarshal(want, &wantVal); err != nil {
		t.Fatalf("unmarshal want %s: %v", want, err)
	}
	if !reflect.DeepEqual(gotVal, wantVal) {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

// setupRunningJob creates a job and drives it through ClaimDue + MarkRunning
// so tests can exercise the post-dispatch transitions (Complete/Retry/Kill)
// without repeating that setup inline.
func setupRunningJob(t *testing.T, s *store.Postgres, ctx context.Context) *job.Job {
	t.Helper()
	j := job.New("send_email", json.RawMessage(`{}`))
	if err := s.Create(ctx, j); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := s.ClaimDue(ctx, 10); err != nil {
		t.Fatalf("ClaimDue() error = %v", err)
	}
	if _, err := s.MarkRunning(ctx, j.ID, "worker-1"); err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}
	return j
}

func TestPostgres_CreateAndGet(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	j := job.New("send_email", json.RawMessage(`{"to":"a@example.com"}`))
	if err := s.Create(ctx, j); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if j.CreatedAt.IsZero() {
		t.Error("Create() did not populate CreatedAt")
	}

	got, err := s.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != j.ID || got.Type != j.Type || got.Status != job.StatusPending {
		t.Errorf("Get() = %+v, want matching %+v", got, j)
	}
	assertJSONEqual(t, got.Payload, j.Payload)
}

func TestPostgres_Get_NotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(context.Background(), uuid.New())
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get() error = %v, want %v", err, store.ErrNotFound)
	}
}

func TestPostgres_List_FilterAndPagination(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	for range 5 {
		j := job.New("send_email", json.RawMessage(`{}`))
		if err := s.Create(ctx, j); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}
	for range 3 {
		j := job.New("resize_image", json.RawMessage(`{}`))
		if err := s.Create(ctx, j); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	emailType := "send_email"
	page1, err := s.List(ctx, store.ListFilter{Type: &emailType, Limit: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if page1.Total != 5 {
		t.Errorf("Total = %d, want 5", page1.Total)
	}
	if len(page1.Jobs) != 2 {
		t.Fatalf("len(Jobs) = %d, want 2", len(page1.Jobs))
	}

	page2, err := s.List(ctx, store.ListFilter{Type: &emailType, Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("List() page 2 error = %v", err)
	}
	if len(page2.Jobs) != 2 {
		t.Fatalf("len(Jobs) page 2 = %d, want 2", len(page2.Jobs))
	}
	if page1.Jobs[0].ID == page2.Jobs[0].ID {
		t.Error("pagination returned the same job on both pages")
	}
}

func TestPostgres_ClaimDue_RespectsRunAtAndStatus(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	due := job.New("send_email", json.RawMessage(`{}`))
	if err := s.Create(ctx, due); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	future := job.New("send_email", json.RawMessage(`{}`), job.WithRunAt(time.Now().Add(time.Hour)))
	if err := s.Create(ctx, future); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	claimed, err := s.ClaimDue(ctx, 10)
	if err != nil {
		t.Fatalf("ClaimDue() error = %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("ClaimDue() returned %d jobs, want 1", len(claimed))
	}
	if claimed[0].ID != due.ID {
		t.Errorf("ClaimDue() claimed %s, want %s", claimed[0].ID, due.ID)
	}
	if claimed[0].Status != job.StatusQueued {
		t.Errorf("claimed job status = %s, want %s", claimed[0].Status, job.StatusQueued)
	}

	claimedAgain, err := s.ClaimDue(ctx, 10)
	if err != nil {
		t.Fatalf("ClaimDue() second call error = %v", err)
	}
	if len(claimedAgain) != 0 {
		t.Errorf("ClaimDue() re-claimed an already-queued job: %v", claimedAgain)
	}
}

// TestPostgres_ClaimDue_ConcurrentClaimsAreDisjoint is the load-bearing test
// for this whole design: it proves FOR UPDATE SKIP LOCKED does what the
// dispatcher depends on — N concurrent claimers never receive the same row,
// and every row is claimed by exactly one of them.
func TestPostgres_ClaimDue_ConcurrentClaimsAreDisjoint(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	const total = 50
	for range total {
		j := job.New("send_email", json.RawMessage(`{}`))
		if err := s.Create(ctx, j); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	const workers = 10
	var (
		mu   sync.Mutex
		seen = make(map[uuid.UUID]int)
		wg   sync.WaitGroup
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claimed, err := s.ClaimDue(ctx, total/workers)
			if err != nil {
				t.Errorf("ClaimDue() error = %v", err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, j := range claimed {
				seen[j.ID]++
			}
		}()
	}
	wg.Wait()

	if len(seen) != total {
		t.Fatalf("claimed %d distinct jobs across %d workers, want %d", len(seen), workers, total)
	}
	for id, count := range seen {
		if count != 1 {
			t.Errorf("job %s claimed %d times, want exactly 1", id, count)
		}
	}
}

func TestPostgres_MarkRunning(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	j := job.New("send_email", json.RawMessage(`{}`))
	if err := s.Create(ctx, j); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := s.ClaimDue(ctx, 10); err != nil {
		t.Fatalf("ClaimDue() error = %v", err)
	}

	running, err := s.MarkRunning(ctx, j.ID, "worker-1")
	if err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}
	if running.Status != job.StatusRunning {
		t.Errorf("Status = %s, want %s", running.Status, job.StatusRunning)
	}
	if running.LockedBy != "worker-1" {
		t.Errorf("LockedBy = %q, want %q", running.LockedBy, "worker-1")
	}
	if running.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", running.Attempts)
	}
	if running.LockedAt == nil {
		t.Error("LockedAt not set")
	}
}

func TestPostgres_MarkRunning_NotClaimable(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	j := job.New("send_email", json.RawMessage(`{}`))
	if err := s.Create(ctx, j); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	// Still pending: never claimed via ClaimDue, so it isn't runnable yet.

	_, err := s.MarkRunning(ctx, j.ID, "worker-1")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("MarkRunning() error = %v, want %v", err, store.ErrNotFound)
	}
}

func TestPostgres_Complete(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	j := setupRunningJob(t, s, ctx)

	result := json.RawMessage(`{"sent":true}`)
	if err := s.Complete(ctx, j.ID, result); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	got, err := s.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != job.StatusCompleted {
		t.Errorf("Status = %s, want %s", got.Status, job.StatusCompleted)
	}
	assertJSONEqual(t, got.Result, result)
}

func TestPostgres_Retry(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	j := setupRunningJob(t, s, ctx)

	nextRunAt := time.Now().Add(30 * time.Second).UTC().Truncate(time.Microsecond)
	if err := s.Retry(ctx, j.ID, "smtp timeout", nextRunAt); err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	got, err := s.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != job.StatusPending {
		t.Errorf("Status = %s, want %s", got.Status, job.StatusPending)
	}
	if got.LastError != "smtp timeout" {
		t.Errorf("LastError = %q, want %q", got.LastError, "smtp timeout")
	}
	if got.LockedBy != "" {
		t.Errorf("LockedBy = %q, want empty after retry", got.LockedBy)
	}
	if !got.RunAt.Equal(nextRunAt) {
		t.Errorf("RunAt = %v, want %v", got.RunAt, nextRunAt)
	}
}

func TestPostgres_Kill(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	j := setupRunningJob(t, s, ctx)

	if err := s.Kill(ctx, j.ID, "exhausted retries"); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}

	got, err := s.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != job.StatusDead {
		t.Errorf("Status = %s, want %s", got.Status, job.StatusDead)
	}
	if got.LastError != "exhausted retries" {
		t.Errorf("LastError = %q, want %q", got.LastError, "exhausted retries")
	}
}

func TestPostgres_Cancel_Pending(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	j := job.New("send_email", json.RawMessage(`{}`))
	if err := s.Create(ctx, j); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	cancelled, err := s.Cancel(ctx, j.ID)
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if cancelled.Status != job.StatusCancelled {
		t.Errorf("Cancel() returned Status = %s, want %s", cancelled.Status, job.StatusCancelled)
	}
	got, err := s.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != job.StatusCancelled {
		t.Errorf("Status = %s, want %s", got.Status, job.StatusCancelled)
	}
}

func TestPostgres_Cancel_AlreadyRunning(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	j := setupRunningJob(t, s, ctx)

	_, err := s.Cancel(ctx, j.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Cancel() error = %v, want %v", err, store.ErrNotFound)
	}
}

func TestPostgres_Reactivate_Dead(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	j := setupRunningJob(t, s, ctx)
	if err := s.Kill(ctx, j.ID, "exhausted retries"); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}

	reactivated, err := s.Reactivate(ctx, j.ID)
	if err != nil {
		t.Fatalf("Reactivate() error = %v", err)
	}
	if reactivated.Status != job.StatusPending {
		t.Errorf("Status = %s, want %s", reactivated.Status, job.StatusPending)
	}
	if reactivated.Attempts != 0 {
		t.Errorf("Attempts = %d, want 0", reactivated.Attempts)
	}
	if reactivated.LastError != "" {
		t.Errorf("LastError = %q, want empty", reactivated.LastError)
	}
}

func TestPostgres_Reactivate_Cancelled(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	j := job.New("send_email", json.RawMessage(`{}`))
	if err := s.Create(ctx, j); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := s.Cancel(ctx, j.ID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	reactivated, err := s.Reactivate(ctx, j.ID)
	if err != nil {
		t.Fatalf("Reactivate() error = %v", err)
	}
	if reactivated.Status != job.StatusPending {
		t.Errorf("Status = %s, want %s", reactivated.Status, job.StatusPending)
	}
}

func TestPostgres_Reactivate_NotDeadOrCancelled(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	j := job.New("send_email", json.RawMessage(`{}`))
	if err := s.Create(ctx, j); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	// Still pending -- Reactivate is only for dead/cancelled jobs.

	_, err := s.Reactivate(ctx, j.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Reactivate() error = %v, want %v", err, store.ErrNotFound)
	}
}

func TestPostgres_ReclaimStale(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	j := setupRunningJob(t, s, ctx)

	// Simulate a lease going stale (e.g. the worker that held it died
	// without acking) by backdating locked_at directly.
	if _, err := testPool.Exec(ctx,
		`UPDATE jobs SET locked_at = now() - interval '10 minutes' WHERE id = $1`, j.ID,
	); err != nil {
		t.Fatalf("backdate locked_at: %v", err)
	}

	reclaimed, err := s.ReclaimStale(ctx, 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("ReclaimStale() error = %v", err)
	}
	if len(reclaimed) != 1 || reclaimed[0].ID != j.ID {
		t.Fatalf("ReclaimStale() = %v, want exactly [%s]", reclaimed, j.ID)
	}
	if reclaimed[0].Status != job.StatusPending {
		t.Errorf("Status = %s, want %s", reclaimed[0].Status, job.StatusPending)
	}

	fresh := setupRunningJob(t, s, ctx)
	reclaimedAgain, err := s.ReclaimStale(ctx, 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("ReclaimStale() second call error = %v", err)
	}
	for _, r := range reclaimedAgain {
		if r.ID == fresh.ID {
			t.Errorf("ReclaimStale() reclaimed a fresh lease for job %s", fresh.ID)
		}
	}
}

func TestPostgres_Stats(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	// Stats() only needs to count rows by status, so fixtures are built by
	// setting Status directly before Create rather than driving each job
	// through its real state machine.
	statuses := []job.Status{
		job.StatusPending, job.StatusPending,
		job.StatusQueued,
		job.StatusRunning,
		job.StatusCompleted,
		job.StatusDead,
		job.StatusCancelled,
	}
	for _, st := range statuses {
		j := job.New("send_email", json.RawMessage(`{}`))
		j.Status = st
		if err := s.Create(ctx, j); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	stats, err := s.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	want := store.Stats{Pending: 2, Queued: 1, Running: 1, Completed: 1, Dead: 1, Cancelled: 1}
	if stats != want {
		t.Errorf("Stats() = %+v, want %+v", stats, want)
	}
}
