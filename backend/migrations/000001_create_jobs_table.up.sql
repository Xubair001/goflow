CREATE TABLE jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type         TEXT NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}'::jsonb,
    status       TEXT NOT NULL DEFAULT 'pending',
    priority     INTEGER NOT NULL DEFAULT 0,
    run_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts     INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    last_error   TEXT NOT NULL DEFAULT '',
    result       JSONB,
    locked_by    TEXT NOT NULL DEFAULT '',
    locked_at    TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT jobs_status_check CHECK (
        status IN ('pending', 'queued', 'running', 'completed', 'dead', 'cancelled')
    )
);

-- Dispatcher's due-job scan: pending jobs ordered by priority then age.
CREATE INDEX idx_jobs_dispatch_due ON jobs (priority DESC, run_at)
    WHERE status = 'pending';

-- Dashboard / API listing filtered by status and/or type.
CREATE INDEX idx_jobs_status_type ON jobs (status, type);

-- Reconciler's stale-lease scan: queued/running jobs whose lock is old.
CREATE INDEX idx_jobs_stale_lease ON jobs (locked_at)
    WHERE status IN ('queued', 'running');
