import { useState } from "react";
import type { Job, JobType, ResizeImageResult } from "../api/types";
import { JOB_TYPE_LABELS } from "../api/types";
import { api } from "../api/client";
import { StatusBadge } from "./StatusBadge";
import { formatRelativeTime } from "../lib/time";

interface JobDetailProps {
  job: Job;
  onChanged: (job: Job) => void;
  onClose: () => void;
}

function isResizeImageResult(job: Job): job is Job & { result: ResizeImageResult } {
  return (
    job.type === "resize_image" &&
    job.result != null &&
    typeof job.result === "object" &&
    "image_base64" in job.result
  );
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
    <div className="space-y-4">
      <div className="flex items-start justify-between border-b border-border-hairline pb-3">
        <div>
          <h2 className="text-sm font-semibold text-text-primary">
            {JOB_TYPE_LABELS[job.type as JobType] ?? job.type}
          </h2>
          <p className="mt-0.5 font-mono text-xs text-text-muted">{job.id}</p>
        </div>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close job detail"
          className="rounded-md p-1 text-text-muted hover:bg-surface-sunken hover:text-text-primary"
        >
          ✕
        </button>
      </div>

      <StatusBadge status={job.status} />

      <dl className="grid grid-cols-2 gap-x-3 gap-y-2 text-sm">
        <dt className="text-text-secondary">Attempts</dt>
        <dd className="tabular-nums text-text-primary">
          {job.attempts} / {job.max_attempts}
        </dd>
        <dt className="text-text-secondary">Priority</dt>
        <dd className="tabular-nums text-text-primary">{job.priority}</dd>
        <dt className="text-text-secondary">Run at</dt>
        <dd className="text-text-primary" title={new Date(job.run_at).toLocaleString()}>
          {formatRelativeTime(job.run_at)}
        </dd>
        <dt className="text-text-secondary">Updated</dt>
        <dd className="text-text-primary" title={new Date(job.updated_at).toLocaleString()}>
          {formatRelativeTime(job.updated_at)}
        </dd>
      </dl>

      {job.last_error && (
        <div>
          <p className="field-label">Last error</p>
          <p className="text-sm text-status-critical">{job.last_error}</p>
        </div>
      )}

      {isResizeImageResult(job) && (
        <div>
          <p className="field-label">Resized image</p>
          <img
            src={`data:image/${job.result.format};base64,${job.result.image_base64}`}
            alt="Resize result"
            className="max-h-64 rounded-lg border border-border-hairline"
          />
          <p className="mt-1.5 text-xs text-text-muted">
            {job.result.width}×{job.result.height} · {job.result.resized_size_bytes.toLocaleString()} bytes
            (from {job.result.original_size_bytes.toLocaleString()})
          </p>
        </div>
      )}

      <div>
        <p className="field-label">Payload</p>
        <pre className="overflow-x-auto rounded-lg bg-surface-sunken p-3 text-xs text-text-primary">
          {JSON.stringify(job.payload, null, 2)}
        </pre>
      </div>

      {job.result != null && !isResizeImageResult(job) && (
        <div>
          <p className="field-label">Result</p>
          <pre className="overflow-x-auto rounded-lg bg-surface-sunken p-3 text-xs text-text-primary">
            {JSON.stringify(job.result, null, 2)}
          </pre>
        </div>
      )}

      {error && <p className="text-sm text-status-critical">{error}</p>}

      {(canRetry || canCancel) && (
        <div className="flex gap-2 border-t border-border-hairline pt-4">
          {canRetry && (
            <button type="button" onClick={handleRetry} disabled={busy} className="btn-primary">
              Retry
            </button>
          )}
          {canCancel && (
            <button type="button" onClick={handleCancel} disabled={busy} className="btn-danger">
              Cancel
            </button>
          )}
        </div>
      )}
    </div>
  );
}
