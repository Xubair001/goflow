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

export interface UploadResponse {
  id: string;
  url: string;
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

export type JobType = (typeof JOB_TYPES)[number];

export const JOB_TYPE_LABELS: Record<JobType, string> = {
  send_email: "Send email",
  resize_image: "Resize image",
  process_csv: "Process CSV",
  make_http_request: "Make HTTP request",
  generate_report: "Generate report",
  scheduled_task: "Scheduled task",
};

export const JOB_STATUSES: JobStatus[] = [
  "pending",
  "queued",
  "running",
  "completed",
  "dead",
  "cancelled",
];

// Mirrors backend/internal/handlers -- one payload shape per job type,
// matching the Go structs (SendEmailPayload etc.) field for field.

export interface SendEmailPayload {
  to: string;
  subject: string;
  body: string;
}

export interface ResizeImagePayload {
  source_url?: string;
  upload_id?: string;
  width: number;
  height: number;
}

export interface ProcessCsvPayload {
  csv_data: string;
  email_to?: string;
}

export type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

export interface MakeHttpRequestPayload {
  url: string;
  method: HttpMethod;
  body?: string;
}

export interface GenerateReportPayload {
  failed_sample_size?: number;
  email_to?: string;
}

// Scheduled tasks can recur into any job type in principle, but the form
// only offers these three: they're the ones where "run this again later"
// is actually useful (a recurring health check, a periodic emailed report,
// a reminder email) rather than replaying a one-off input like an uploaded
// image or pasted CSV.
export const SCHEDULABLE_TARGET_TYPES = ["make_http_request", "generate_report", "send_email"] as const;
export type ScheduledTargetType = (typeof SCHEDULABLE_TARGET_TYPES)[number];

export interface ScheduledTaskPayload {
  target_type: ScheduledTargetType;
  target_payload?: unknown;
  interval_seconds?: number;
}

export interface ColumnSummary {
  name: string;
  numeric: boolean;
  count: number;
  sum?: number;
  min?: number;
  max?: number;
  average?: number;
}

export interface ProcessCsvResult {
  row_count: number;
  column_count: number;
  columns: ColumnSummary[];
  emailed_to?: string;
}

export interface ResizeImageResult {
  width: number;
  height: number;
  format: string;
  image_base64: string;
  original_size_bytes: number;
  resized_size_bytes: number;
}

export interface MakeHttpRequestResult {
  status_code: number;
  body: string;
  truncated: boolean;
}

export interface DeadJobBrief {
  id: string;
  type: string;
  last_error: string;
}

export interface GenerateReportResult {
  generated_at: string;
  stats: QueueStats;
  recent_dead_jobs: DeadJobBrief[];
  summary: string;
  emailed_to?: string;
}

export interface ScheduledTaskResult {
  ran_at: string;
  target_type: string;
  target_result?: unknown;
  target_error?: string;
  next_job_id?: string;
  next_run_at?: string;
}

export interface SendEmailResult {
  sent_to: string;
  sent_at: string;
}
