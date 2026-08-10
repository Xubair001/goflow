package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abdullah-zubair/jobqueue/internal/job"
)

// jobColumns must list every job.Job field in the exact order scanJob
// expects them. Columns are qualified with the jobs. prefix because
// ClaimDue and ReclaimStale RETURN this list from an UPDATE ... FROM a CTE
// that also has an id column — unqualified names would be ambiguous there.
const jobColumns = `jobs.id, jobs.type, jobs.payload, jobs.status, jobs.priority, jobs.run_at, jobs.attempts, ` +
	`jobs.max_attempts, jobs.last_error, jobs.result, jobs.locked_by, jobs.locked_at, jobs.created_at, jobs.updated_at`

// Postgres is the Store implementation backed by PostgreSQL, using
// SELECT ... FOR UPDATE SKIP LOCKED for safe concurrent job claiming across
// multiple dispatcher/reconciler instances.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres wraps an existing connection pool as a Store. The caller owns
// the pool's lifecycle (including Close).
func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

var _ Store = (*Postgres)(nil)

// rowScanner is satisfied by both pgx.Row and pgx.Rows, letting scanJob
// serve single-row and multi-row callers alike.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanJob reads one row shaped like jobColumns into a job.Job. Callers that
// SELECT extra trailing columns (e.g. a window-function total count) pass
// pointers for them via extra, in the order they appear in the query.
func scanJob(row rowScanner, extra ...any) (*job.Job, error) {
	var (
		j       job.Job
		payload []byte
		result  []byte
	)
	dest := append([]any{
		&j.ID, &j.Type, &payload, &j.Status, &j.Priority, &j.RunAt,
		&j.Attempts, &j.MaxAttempts, &j.LastError, &result,
		&j.LockedBy, &j.LockedAt, &j.CreatedAt, &j.UpdatedAt,
	}, extra...)
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	j.Payload = json.RawMessage(payload)
	if result != nil {
		j.Result = json.RawMessage(result)
	}
	return &j, nil
}

// Create implements Store.
func (p *Postgres) Create(ctx context.Context, j *job.Job) error {
	const q = `
		INSERT INTO jobs (id, type, payload, status, priority, run_at, max_attempts)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at`
	err := p.pool.QueryRow(ctx, q,
		j.ID, j.Type, []byte(j.Payload), j.Status, j.Priority, j.RunAt, j.MaxAttempts,
	).Scan(&j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: create job: %w", err)
	}
	return nil
}

// Get implements Store.
func (p *Postgres) Get(ctx context.Context, id uuid.UUID) (*job.Job, error) {
	q := `SELECT ` + jobColumns + ` FROM jobs WHERE id = $1`
	j, err := scanJob(p.pool.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get job %s: %w", id, err)
	}
	return j, nil
}

// List implements Store.
func (p *Postgres) List(ctx context.Context, filter ListFilter) (ListResult, error) {
	where := make([]string, 0, 2)
	args := make([]any, 0, 4)
	if filter.Status != nil {
		args = append(args, *filter.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Type != nil {
		args = append(args, *filter.Type)
		where = append(where, fmt.Sprintf("type = $%d", len(args)))
	}
	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	args = append(args, limit, filter.Offset)
	limitArg, offsetArg := len(args)-1, len(args)

	q := fmt.Sprintf(`
		SELECT %s, count(*) OVER() AS total_count
		FROM jobs
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, jobColumns, whereClause, limitArg, offsetArg)

	rows, err := p.pool.Query(ctx, q, args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("store: list jobs: %w", err)
	}
	defer rows.Close()

	var out ListResult
	for rows.Next() {
		var total int
		j, err := scanJob(rows, &total)
		if err != nil {
			return ListResult{}, fmt.Errorf("store: list jobs: scan: %w", err)
		}
		out.Jobs = append(out.Jobs, j)
		out.Total = total
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, fmt.Errorf("store: list jobs: %w", err)
	}
	return out, nil
}

// ClaimDue implements Store.
func (p *Postgres) ClaimDue(ctx context.Context, limit int) ([]*job.Job, error) {
	q := `
		WITH claimed AS (
			SELECT id FROM jobs
			WHERE status = $1 AND run_at <= now()
			ORDER BY priority DESC, run_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE jobs SET status = $3, locked_at = now(), updated_at = now()
		FROM claimed
		WHERE jobs.id = claimed.id
		RETURNING ` + jobColumns

	rows, err := p.pool.Query(ctx, q, job.StatusPending, limit, job.StatusQueued)
	if err != nil {
		return nil, fmt.Errorf("store: claim due jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*job.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("store: claim due jobs: scan: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// MarkRunning implements Store.
func (p *Postgres) MarkRunning(ctx context.Context, id uuid.UUID, consumer string) (*job.Job, error) {
	q := `
		UPDATE jobs
		SET status = $2, locked_by = $3, locked_at = now(), attempts = attempts + 1, updated_at = now()
		WHERE id = $1 AND status IN ($2, $4)
		RETURNING ` + jobColumns

	j, err := scanJob(p.pool.QueryRow(ctx, q, id, job.StatusRunning, consumer, job.StatusQueued))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: mark job %s running: %w", id, err)
	}
	return j, nil
}

// Complete implements Store.
func (p *Postgres) Complete(ctx context.Context, id uuid.UUID, result json.RawMessage) error {
	const q = `
		UPDATE jobs
		SET status = $2, result = $3, last_error = '', locked_by = '', locked_at = NULL, updated_at = now()
		WHERE id = $1`
	tag, err := p.pool.Exec(ctx, q, id, job.StatusCompleted, []byte(result))
	if err != nil {
		return fmt.Errorf("store: complete job %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Retry implements Store.
func (p *Postgres) Retry(ctx context.Context, id uuid.UUID, lastErr string, nextRunAt time.Time) error {
	const q = `
		UPDATE jobs
		SET status = $2, run_at = $3, last_error = $4, locked_by = '', locked_at = NULL, updated_at = now()
		WHERE id = $1`
	tag, err := p.pool.Exec(ctx, q, id, job.StatusPending, nextRunAt, lastErr)
	if err != nil {
		return fmt.Errorf("store: retry job %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Kill implements Store.
func (p *Postgres) Kill(ctx context.Context, id uuid.UUID, lastErr string) error {
	const q = `
		UPDATE jobs
		SET status = $2, last_error = $3, locked_by = '', locked_at = NULL, updated_at = now()
		WHERE id = $1`
	tag, err := p.pool.Exec(ctx, q, id, job.StatusDead, lastErr)
	if err != nil {
		return fmt.Errorf("store: kill job %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Cancel implements Store.
func (p *Postgres) Cancel(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE jobs SET status = $2, updated_at = now()
		WHERE id = $1 AND status IN ($3, $4)`
	tag, err := p.pool.Exec(ctx, q, id, job.StatusCancelled, job.StatusPending, job.StatusQueued)
	if err != nil {
		return fmt.Errorf("store: cancel job %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReclaimStale implements Store.
func (p *Postgres) ReclaimStale(ctx context.Context, olderThan time.Duration, limit int) ([]*job.Job, error) {
	q := `
		WITH stale AS (
			SELECT id FROM jobs
			WHERE status IN ($1, $2) AND locked_at < now() - $3::interval
			ORDER BY locked_at
			LIMIT $4
			FOR UPDATE SKIP LOCKED
		)
		UPDATE jobs
		SET status = $5, run_at = now(), locked_by = '', locked_at = NULL, updated_at = now()
		FROM stale
		WHERE jobs.id = stale.id
		RETURNING ` + jobColumns

	interval := fmt.Sprintf("%d seconds", int64(olderThan.Seconds()))

	rows, err := p.pool.Query(ctx, q, job.StatusQueued, job.StatusRunning, interval, limit, job.StatusPending)
	if err != nil {
		return nil, fmt.Errorf("store: reclaim stale jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*job.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("store: reclaim stale jobs: scan: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// Stats implements Store.
func (p *Postgres) Stats(ctx context.Context) (Stats, error) {
	const q = `SELECT status, count(*) FROM jobs GROUP BY status`
	rows, err := p.pool.Query(ctx, q)
	if err != nil {
		return Stats{}, fmt.Errorf("store: stats: %w", err)
	}
	defer rows.Close()

	var s Stats
	for rows.Next() {
		var status job.Status
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return Stats{}, fmt.Errorf("store: stats: scan: %w", err)
		}
		switch status {
		case job.StatusPending:
			s.Pending = count
		case job.StatusQueued:
			s.Queued = count
		case job.StatusRunning:
			s.Running = count
		case job.StatusCompleted:
			s.Completed = count
		case job.StatusDead:
			s.Dead = count
		case job.StatusCancelled:
			s.Cancelled = count
		}
	}
	return s, rows.Err()
}
