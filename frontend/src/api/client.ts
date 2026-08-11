import type {
  ApiError,
  CreateJobRequest,
  Job,
  JobStatus,
  ListJobsResponse,
  QueueStats,
  UploadResponse,
} from "./types";

// ApiRequestError carries the server's structured error code/message
// instead of a generic "response not ok" string, so callers can show the
// real reason a request failed.
export class ApiRequestError extends Error {
  code: string;

  constructor(code: string, message: string) {
    super(message);
    this.name = "ApiRequestError";
    this.code = code;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });

  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as ApiError | null;
    throw new ApiRequestError(
      body?.error.code ?? "unknown_error",
      body?.error.message ?? `request failed with status ${res.status}`,
    );
  }

  if (res.status === 204) {
    return undefined as T;
  }
  return (await res.json()) as T;
}

// Deliberately not routed through request(): that helper always sends
// Content-Type: application/json, but a multipart body needs the browser
// to set Content-Type itself (with the boundary it picked), so this must
// not set that header at all.
async function upload(path: string, file: File): Promise<UploadResponse> {
  const form = new FormData();
  form.append("file", file);

  const res = await fetch(path, { method: "POST", body: form });
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as ApiError | null;
    throw new ApiRequestError(
      body?.error.code ?? "unknown_error",
      body?.error.message ?? `upload failed with status ${res.status}`,
    );
  }
  return (await res.json()) as UploadResponse;
}

export interface ListJobsParams {
  status?: JobStatus;
  type?: string;
  limit?: number;
  offset?: number;
}

export const api = {
  listJobs(params: ListJobsParams = {}): Promise<ListJobsResponse> {
    const query = new URLSearchParams();
    if (params.status) query.set("status", params.status);
    if (params.type) query.set("type", params.type);
    query.set("limit", String(params.limit ?? 20));
    query.set("offset", String(params.offset ?? 0));
    return request(`/api/v1/jobs?${query.toString()}`);
  },

  getJob(id: string): Promise<Job> {
    return request(`/api/v1/jobs/${id}`);
  },

  createJob(body: CreateJobRequest): Promise<Job> {
    return request("/api/v1/jobs", { method: "POST", body: JSON.stringify(body) });
  },

  retryJob(id: string): Promise<Job> {
    return request(`/api/v1/jobs/${id}/retry`, { method: "POST" });
  },

  cancelJob(id: string): Promise<Job> {
    return request(`/api/v1/jobs/${id}/cancel`, { method: "POST" });
  },

  queueStats(): Promise<QueueStats> {
    return request("/api/v1/queue/stats");
  },

  uploadFile(file: File): Promise<UploadResponse> {
    return upload("/api/v1/uploads", file);
  },
};
