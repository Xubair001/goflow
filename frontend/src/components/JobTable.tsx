import { useState, type MouseEvent } from "react";
import type { Job, JobStatus, JobType } from "../api/types";
import { JOB_STATUSES, JOB_TYPE_LABELS } from "../api/types";
import { StatusBadge } from "./StatusBadge";
import { api } from "../api/client";
import { formatRelativeTime } from "../lib/time";

interface JobTableProps {
  jobs: Job[];
  total: number;
  limit: number;
  offset: number;
  statusFilter: JobStatus | "";
  onStatusFilterChange: (status: JobStatus | "") => void;
  onPageChange: (offset: number) => void;
  onSelect: (job: Job) => void;
  onJobChanged: () => void;
  selectedId?: string;
}

function RowActions({
  job,
  onSelect,
  onJobChanged,
}: {
  job: Job;
  onSelect: (job: Job) => void;
  onJobChanged: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const canRetry = job.status === "dead" || job.status === "cancelled";
  const canCancel = job.status === "pending" || job.status === "queued";

  async function run(e: MouseEvent, action: (id: string) => Promise<Job>) {
    e.stopPropagation();
    setBusy(true);
    try {
      await action(job.id);
      onJobChanged();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex gap-1.5">
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation();
          onSelect(job);
        }}
        className="btn-secondary btn-sm"
      >
        View
      </button>
      {canRetry && (
        <button
          type="button"
          disabled={busy}
          onClick={(e) => run(e, api.retryJob)}
          className="btn-secondary btn-sm"
        >
          Retry
        </button>
      )}
      {canCancel && (
        <button
          type="button"
          disabled={busy}
          onClick={(e) => run(e, api.cancelJob)}
          className="btn-danger btn-sm"
        >
          Cancel
        </button>
      )}
    </div>
  );
}

export function JobTable({
  jobs,
  total,
  limit,
  offset,
  statusFilter,
  onStatusFilterChange,
  onPageChange,
  onSelect,
  onJobChanged,
  selectedId,
}: JobTableProps) {
  const page = Math.floor(offset / limit) + 1;
  const pageCount = Math.max(1, Math.ceil(total / limit));

  return (
    <div className="card overflow-hidden">
      <div className="flex items-center justify-between border-b border-border-hairline px-4 py-3">
        <h2 className="text-sm font-semibold text-text-primary">Jobs</h2>
        <select
          value={statusFilter}
          onChange={(e) => onStatusFilterChange(e.target.value as JobStatus | "")}
          className="field w-auto py-1.5"
        >
          <option value="">All statuses</option>
          {JOB_STATUSES.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="border-b border-border-hairline bg-surface-sunken text-text-muted">
              <th className="px-4 py-2 text-xs font-medium tracking-wide uppercase">Job</th>
              <th className="px-4 py-2 text-xs font-medium tracking-wide uppercase">Status</th>
              <th className="px-4 py-2 text-xs font-medium tracking-wide uppercase">Attempts</th>
              <th className="px-4 py-2 text-xs font-medium tracking-wide uppercase">Created</th>
              <th className="px-4 py-2 text-xs font-medium tracking-wide uppercase">Actions</th>
            </tr>
          </thead>
          <tbody>
            {jobs.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-10 text-center text-text-muted">
                  No jobs match this filter.
                </td>
              </tr>
            )}
            {jobs.map((job) => {
              const retried = job.attempts > 1 && job.status !== "dead";
              return (
                <tr
                  key={job.id}
                  onClick={() => onSelect(job)}
                  className={`cursor-pointer border-b border-border-hairline last:border-0 hover:bg-surface-sunken ${
                    selectedId === job.id ? "bg-surface-sunken" : ""
                  }`}
                >
                  <td className="px-4 py-2.5">
                    <div className="font-medium text-text-primary">
                      {JOB_TYPE_LABELS[job.type as JobType] ?? job.type}
                    </div>
                    <div className="font-mono text-xs text-text-muted">{job.id.slice(0, 8)}</div>
                  </td>
                  <td className="px-4 py-2.5">
                    <StatusBadge status={job.status} />
                  </td>
                  <td className="px-4 py-2.5 tabular-nums">
                    <span className={retried ? "font-medium text-status-warning" : "text-text-secondary"}>
                      {job.attempts}/{job.max_attempts}
                    </span>
                  </td>
                  <td
                    className="px-4 py-2.5 whitespace-nowrap text-text-secondary"
                    title={new Date(job.created_at).toLocaleString()}
                  >
                    {formatRelativeTime(job.created_at)}
                  </td>
                  <td className="px-4 py-2.5" onClick={(e) => e.stopPropagation()}>
                    <RowActions job={job} onSelect={onSelect} onJobChanged={onJobChanged} />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between px-4 py-3 text-sm text-text-secondary">
        <span>
          Page {page} of {pageCount} ({total} total)
        </span>
        <div className="flex gap-2">
          <button
            type="button"
            disabled={offset === 0}
            onClick={() => onPageChange(Math.max(0, offset - limit))}
            className="btn-secondary btn-sm"
          >
            Previous
          </button>
          <button
            type="button"
            disabled={offset + limit >= total}
            onClick={() => onPageChange(offset + limit)}
            className="btn-secondary btn-sm"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  );
}
