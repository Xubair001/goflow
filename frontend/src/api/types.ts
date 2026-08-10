// Mirrors backend/internal/api/jobs.go's jobResponse and
// backend/internal/store/store.go's Stats -- keep these two in sync by hand
// since there's no shared schema generation yet.

export type JobStatus =
  | "pending"
  | "queued"
  | "running"
  | "completed"
  | "dead"
  | "cancelled";

export interface Job {
  id: string;
  type: string;
  payload: unknown;
  status: JobStatus;
  priority: number;
  run_at: string;
  attempts: number;
  max_attempts: number;
  last_error?: string;
  result?: unknown;
  created_at: string;
  updated_at: string;
}

export interface ListJobsResponse {
  jobs: Job[];
  total: number;
  limit: number;
  offset: number;
}

export interface QueueStats {
  Pending: number;
  Queued: number;
  Running: number;
  Completed: number;
  Dead: number;
  Cancelled: number;
}

export interface CreateJobRequest {
  type: string;
  payload?: unknown;
  priority?: number;
  run_at?: string;
  max_attempts?: number;
}

export interface ApiError {
  error: {
    code: string;
    message: string;
  };
}

// The built-in job types from backend/internal/handlers/registry.go. There's
// no endpoint to discover these dynamically yet, so the submit form's type
// dropdown is this fixed list -- update both sides if a handler is added.
export const JOB_TYPES = [
  "send_email",
  "resize_image",
  "process_csv",
  "make_http_request",
  "generate_report",
  "scheduled_task",
] as const;

export const JOB_STATUSES: JobStatus[] = [
  "pending",
  "queued",
  "running",
  "completed",
  "dead",
  "cancelled",
];
