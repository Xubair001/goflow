import type { JobStatus } from "../api/types";

const STATUS_LABEL: Record<JobStatus, string> = {
  pending: "Pending",
  queued: "Queued",
  running: "Running",
  completed: "Completed",
  dead: "Dead",
  cancelled: "Cancelled",
};

// Status color is never the only cue -- the label text carries the state
// too, so this reads fine without color.
const STATUS_DOT: Record<JobStatus, string> = {
  pending: "bg-status-neutral",
  queued: "bg-status-neutral",
  running: "bg-status-running",
  completed: "bg-status-good",
  dead: "bg-status-critical",
  cancelled: "bg-status-neutral",
};

const STATUS_PILL: Record<JobStatus, string> = {
  pending: "bg-status-neutral/10 text-text-secondary",
  queued: "bg-status-neutral/10 text-text-secondary",
  running: "bg-status-running/10 text-status-running",
  completed: "bg-status-good/10 text-status-good",
  dead: "bg-status-critical/10 text-status-critical",
  cancelled: "bg-status-neutral/10 text-text-secondary",
};

export function StatusBadge({ status }: { status: JobStatus }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ${STATUS_PILL[status]}`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${STATUS_DOT[status]}`} aria-hidden="true" />
      {STATUS_LABEL[status]}
    </span>
  );
}
