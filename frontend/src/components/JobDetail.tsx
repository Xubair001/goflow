import { useState } from "react";
import type { Job } from "../api/types";
import { api } from "../api/client";
import { StatusBadge } from "./StatusBadge";

interface JobDetailProps {
  job: Job;
  onChanged: (job: Job) => void;
  onClose: () => void;
}

export function JobDetail({ job, onChanged, onClose }: JobDetailProps) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const canRetry = job.status === "dead" || job.status === "cancelled";
  const canCancel = job.status === "pending" || job.status === "queued";

  async function handleRetry() {
    setBusy(true);
    setError(null);
    try {
      onChanged(await api.retryJob(job.id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to retry job");
    } finally {
      setBusy(false);
    }
  }

  async function handleCancel() {
    setBusy(true);
    setError(null);
    try {
      onChanged(await api.cancelJob(job.id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to cancel job");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-3 rounded-lg border border-border-hairline bg-surface-card p-4">
      <div className="flex items-start justify-between">
        <div>
          <h2 className="font-mono text-sm font-semibold text-text-primary">{job.type}</h2>
          <p className="font-mono text-xs text-text-muted">{job.id}</p>
        </div>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close job detail"
          className="text-text-muted hover:text-text-primary"
        >
          ✕
        </button>
      </div>

      <StatusBadge status={job.status} />

      <dl className="grid grid-cols-2 gap-2 text-sm">
        <dt className="text-text-secondary">Attempts</dt>
        <dd className="tabular-nums text-text-primary">
          {job.attempts} / {job.max_attempts}
        </dd>
        <dt className="text-text-secondary">Priority</dt>
        <dd className="tabular-nums text-text-primary">{job.priority}</dd>
        <dt className="text-text-secondary">Run at</dt>
        <dd className="text-text-primary">{new Date(job.run_at).toLocaleString()}</dd>
        <dt className="text-text-secondary">Updated</dt>
        <dd className="text-text-primary">{new Date(job.updated_at).toLocaleString()}</dd>
      </dl>

      {job.last_error && (
        <div>
          <p className="text-sm text-text-secondary">Last error</p>
          <p className="text-sm text-status-critical">{job.last_error}</p>
        </div>
      )}

      <div>
        <p className="text-sm text-text-secondary">Payload</p>
        <pre className="overflow-x-auto rounded bg-surface-page p-2 text-xs text-text-primary">
          {JSON.stringify(job.payload, null, 2)}
        </pre>
      </div>

      {job.result != null && (
        <div>
          <p className="text-sm text-text-secondary">Result</p>
          <pre className="overflow-x-auto rounded bg-surface-page p-2 text-xs text-text-primary">
            {JSON.stringify(job.result, null, 2)}
          </pre>
        </div>
      )}

      {error && <p className="text-sm text-status-critical">{error}</p>}

      <div className="flex gap-2">
        {canRetry && (
          <button
            type="button"
            onClick={handleRetry}
            disabled={busy}
            className="rounded bg-status-running px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50"
          >
            Retry
          </button>
        )}
        {canCancel && (
          <button
            type="button"
            onClick={handleCancel}
            disabled={busy}
            className="rounded border border-border-hairline px-3 py-1.5 text-sm font-medium text-text-primary disabled:opacity-50"
          >
            Cancel
          </button>
        )}
      </div>
    </div>
  );
}
