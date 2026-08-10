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
const STATUS_COLOR: Record<JobStatus, string> = {
  pending: "bg-status-neutral",
  queued: "bg-status-neutral",
  running: "bg-status-running",
  completed: "bg-status-good",
  dead: "bg-status-critical",
  cancelled: "bg-status-neutral",
};

export function StatusBadge({ status }: { status: JobStatus }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-sm text-text-secondary">
      <span className={`h-2 w-2 rounded-full ${STATUS_COLOR[status]}`} aria-hidden="true" />
      {STATUS_LABEL[status]}
    </span>
  );
}
