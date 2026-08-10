import type { Job, JobStatus } from "../api/types";
import { JOB_STATUSES } from "../api/types";
import { StatusBadge } from "./StatusBadge";

interface JobTableProps {
  jobs: Job[];
  total: number;
  limit: number;
  offset: number;
  statusFilter: JobStatus | "";
  onStatusFilterChange: (status: JobStatus | "") => void;
  onPageChange: (offset: number) => void;
  onSelect: (job: Job) => void;
  selectedId?: string;
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
  selectedId,
}: JobTableProps) {
  const page = Math.floor(offset / limit) + 1;
  const pageCount = Math.max(1, Math.ceil(total / limit));

  return (
    <div className="rounded-lg border border-border-hairline bg-surface-card">
      <div className="flex items-center justify-between border-b border-border-hairline p-3">
        <h2 className="text-sm font-semibold text-text-primary">Jobs</h2>
        <select
          value={statusFilter}
          onChange={(e) => onStatusFilterChange(e.target.value as JobStatus | "")}
          className="rounded border border-border-hairline bg-surface-page px-2 py-1 text-sm text-text-primary"
        >
          <option value="">All statuses</option>
          {JOB_STATUSES.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </div>

      <table className="w-full text-left text-sm">
        <thead className="text-text-muted">
          <tr className="border-b border-border-hairline">
            <th className="px-3 py-2 font-normal">Type</th>
            <th className="px-3 py-2 font-normal">Status</th>
            <th className="px-3 py-2 font-normal">Attempts</th>
            <th className="px-3 py-2 font-normal">Created</th>
          </tr>
        </thead>
        <tbody>
          {jobs.length === 0 && (
            <tr>
              <td colSpan={4} className="px-3 py-6 text-center text-text-muted">
                No jobs match this filter.
              </td>
            </tr>
          )}
          {jobs.map((job) => (
            <tr
              key={job.id}
              onClick={() => onSelect(job)}
              className={`cursor-pointer border-b border-border-hairline last:border-0 hover:bg-surface-page ${
                selectedId === job.id ? "bg-surface-page" : ""
              }`}
            >
              <td className="px-3 py-2 font-mono text-text-primary">{job.type}</td>
              <td className="px-3 py-2">
                <StatusBadge status={job.status} />
              </td>
              <td className="px-3 py-2 tabular-nums text-text-secondary">
                {job.attempts}/{job.max_attempts}
              </td>
              <td className="px-3 py-2 tabular-nums text-text-secondary">
                {new Date(job.created_at).toLocaleString()}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <div className="flex items-center justify-between p-3 text-sm text-text-secondary">
        <span>
          Page {page} of {pageCount} ({total} total)
        </span>
        <div className="flex gap-2">
          <button
            type="button"
            disabled={offset === 0}
            onClick={() => onPageChange(Math.max(0, offset - limit))}
            className="rounded border border-border-hairline px-2 py-1 disabled:opacity-40"
          >
            Previous
          </button>
          <button
            type="button"
            disabled={offset + limit >= total}
            onClick={() => onPageChange(offset + limit)}
            className="rounded border border-border-hairline px-2 py-1 disabled:opacity-40"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  );
}
