import type {
  ApiError,
  CreateJobRequest,
  Job,
  JobStatus,
  ListJobsResponse,
  QueueStats,
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
};
