import { useState, type ReactNode } from "react";
import type {
  DeadJobBrief,
  GenerateReportResult,
  Job,
  JobType,
  MakeHttpRequestResult,
  ProcessCsvResult,
  ResizeImageResult,
  ScheduledTaskResult,
  SendEmailResult,
} from "../api/types";
import { JOB_TYPE_LABELS } from "../api/types";
import { api } from "../api/client";
import { StatusBadge } from "./StatusBadge";
import { formatRelativeTime } from "../lib/time";
import { base64ToBlob, downloadBlob, downloadJSON, downloadText } from "../lib/download";

interface JobDetailProps {
  job: Job;
  onChanged: (job: Job) => void;
  onClose: () => void;
}

function DownloadButton({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <button type="button" onClick={onClick} className="btn-secondary btn-sm">
      ↓ {children}
    </button>
  );
}

function ResizeImageOutput({ job, result }: { job: Job; result: ResizeImageResult }) {
  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <p className="field-label !mb-0">Output</p>
        <DownloadButton
          onClick={() =>
            downloadBlob(
              `job-${job.id.slice(0, 8)}-resized.${result.format}`,
              base64ToBlob(result.image_base64, `image/${result.format}`),
            )
          }
        >
          Download image
        </DownloadButton>
      </div>
      <div className="flex h-48 w-48 items-center justify-center rounded-lg border border-border-hairline bg-surface-sunken p-2">
        <img
          src={`data:image/${result.format};base64,${result.image_base64}`}
          alt="Resize result"
          className="h-full w-full object-contain"
          style={result.width < 192 || result.height < 192 ? { imageRendering: "pixelated" } : undefined}
        />
      </div>
      <p className="mt-1.5 text-xs text-text-muted">
        {result.width}×{result.height} · {result.resized_size_bytes.toLocaleString()} bytes (from{" "}
        {result.original_size_bytes.toLocaleString()})
      </p>
    </div>
  );
}

function ProcessCsvOutput({ job, result }: { job: Job; result: ProcessCsvResult }) {
  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <p className="field-label !mb-0">Output</p>
        <DownloadButton onClick={() => downloadJSON(`job-${job.id.slice(0, 8)}-summary.json`, result)}>
          Download summary
        </DownloadButton>
      </div>
      <p className="mb-2 text-xs text-text-muted">
        {result.row_count} rows · {result.column_count} columns
      </p>
      <div className="overflow-x-auto rounded-lg border border-border-hairline">
        <table className="w-full text-left text-xs">
          <thead>
            <tr className="border-b border-border-hairline bg-surface-sunken text-text-muted">
              <th className="px-2.5 py-1.5 font-medium">Column</th>
              <th className="px-2.5 py-1.5 font-medium">Count</th>
              <th className="px-2.5 py-1.5 font-medium">Sum</th>
              <th className="px-2.5 py-1.5 font-medium">Min</th>
              <th className="px-2.5 py-1.5 font-medium">Max</th>
              <th className="px-2.5 py-1.5 font-medium">Average</th>
            </tr>
          </thead>
          <tbody>
            {result.columns.map((col) => (
              <tr key={col.name} className="border-b border-border-hairline last:border-0">
                <td className="px-2.5 py-1.5 font-medium text-text-primary">{col.name}</td>
                <td className="px-2.5 py-1.5 tabular-nums text-text-secondary">{col.count}</td>
                {col.numeric ? (
                  <>
                    <td className="px-2.5 py-1.5 tabular-nums text-text-secondary">{col.sum?.toFixed(2)}</td>
                    <td className="px-2.5 py-1.5 tabular-nums text-text-secondary">{col.min?.toFixed(2)}</td>
                    <td className="px-2.5 py-1.5 tabular-nums text-text-secondary">{col.max?.toFixed(2)}</td>
                    <td className="px-2.5 py-1.5 tabular-nums text-text-secondary">{col.average?.toFixed(2)}</td>
                  </>
                ) : (
                  <td colSpan={4} className="px-2.5 py-1.5 text-text-muted">
                    non-numeric
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function httpResponseFilename(job: Job, body: string): string {
  try {
    JSON.parse(body);
    return `job-${job.id.slice(0, 8)}-response.json`;
  } catch {
    return `job-${job.id.slice(0, 8)}-response.txt`;
  }
}

function prettyBody(body: string): string {
  try {
    return JSON.stringify(JSON.parse(body), null, 2);
  } catch {
    return body;
  }
}

function MakeHttpRequestOutput({ job, result }: { job: Job; result: MakeHttpRequestResult }) {
  const statusClass =
    result.status_code >= 500
      ? "bg-status-critical/10 text-status-critical"
      : result.status_code >= 400
        ? "bg-status-warning/10 text-status-warning"
        : "bg-status-good/10 text-status-good";

  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <p className="field-label !mb-0">Output</p>
        <DownloadButton onClick={() => downloadText(httpResponseFilename(job, result.body), result.body)}>
          Download response
        </DownloadButton>
      </div>
      <div className="mb-2 flex items-center gap-2">
        <span className={`rounded-full px-2 py-0.5 text-xs font-semibold tabular-nums ${statusClass}`}>
          {result.status_code}
        </span>
        {result.truncated && <span className="text-xs text-status-warning">Response truncated</span>}
      </div>
      <pre className="max-h-64 overflow-auto rounded-lg bg-surface-sunken p-3 text-xs text-text-primary">
        {prettyBody(result.body)}
      </pre>
    </div>
  );
}

function GenerateReportOutput({ job, result }: { job: Job; result: GenerateReportResult }) {
  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <p className="field-label !mb-0">Output</p>
        <DownloadButton onClick={() => downloadJSON(`job-${job.id.slice(0, 8)}-report.json`, result)}>
          Download report
        </DownloadButton>
      </div>
      <p className="mb-2 text-sm text-text-primary">{result.summary}</p>
      <dl className="mb-2 grid grid-cols-3 gap-2 text-xs">
        {(
          [
            ["Pending", result.stats.Pending],
            ["Queued", result.stats.Queued],
            ["Running", result.stats.Running],
            ["Completed", result.stats.Completed],
            ["Dead", result.stats.Dead],
            ["Cancelled", result.stats.Cancelled],
          ] as const
        ).map(([label, value]) => (
          <div key={label} className="rounded-lg bg-surface-sunken px-2 py-1.5">
            <dt className="text-text-muted">{label}</dt>
            <dd className="tabular-nums font-semibold text-text-primary">{value}</dd>
          </div>
        ))}
      </dl>
      {result.recent_dead_jobs.length > 0 && (
        <div className="space-y-1.5">
          <p className="text-xs font-medium tracking-wide text-text-muted uppercase">Recent dead jobs</p>
          {result.recent_dead_jobs.map((j: DeadJobBrief) => (
            <div key={j.id} className="rounded-lg border border-border-hairline p-2 text-xs">
              <span className="font-mono text-text-muted">{j.id.slice(0, 8)}</span>{" "}
              <span className="font-medium text-text-primary">{j.type}</span>
              <p className="mt-0.5 text-status-critical">{j.last_error}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function ScheduledTaskOutput({ job, result }: { job: Job; result: ScheduledTaskResult }) {
  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <p className="field-label !mb-0">Output</p>
        <DownloadButton onClick={() => downloadJSON(`job-${job.id.slice(0, 8)}-result.json`, result)}>
          Download
        </DownloadButton>
      </div>
      <p className="text-sm text-text-primary">{result.message || <span className="text-text-muted">(no message)</span>}</p>
      {result.next_run_at && (
        <p className="mt-1.5 text-xs text-text-muted">
          Next occurrence: {formatRelativeTime(result.next_run_at)}
          {result.next_job_id && <> · job {result.next_job_id.slice(0, 8)}</>}
        </p>
      )}
    </div>
  );
}

function SendEmailOutput({ job, result }: { job: Job; result: SendEmailResult }) {
  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <p className="field-label !mb-0">Output</p>
        <DownloadButton onClick={() => downloadJSON(`job-${job.id.slice(0, 8)}-result.json`, result)}>
          Download
        </DownloadButton>
      </div>
      <p className="flex items-center gap-1.5 rounded-lg bg-status-good/10 px-3 py-2 text-sm text-status-good">
        <span aria-hidden="true">✓</span> Sent to {result.sent_to}
      </p>
      <p className="mt-1 text-xs text-text-muted" title={new Date(result.sent_at).toLocaleString()}>
        {formatRelativeTime(result.sent_at)}
      </p>
    </div>
  );
}

function ResultOutput({ job }: { job: Job }) {
  if (job.result == null) return null;

  switch (job.type as JobType) {
    case "resize_image":
      return <ResizeImageOutput job={job} result={job.result as ResizeImageResult} />;
    case "process_csv":
      return <ProcessCsvOutput job={job} result={job.result as ProcessCsvResult} />;
    case "make_http_request":
      return <MakeHttpRequestOutput job={job} result={job.result as MakeHttpRequestResult} />;
    case "generate_report":
      return <GenerateReportOutput job={job} result={job.result as GenerateReportResult} />;
    case "scheduled_task":
      return <ScheduledTaskOutput job={job} result={job.result as ScheduledTaskResult} />;
    case "send_email":
      return <SendEmailOutput job={job} result={job.result as SendEmailResult} />;
    default:
      return (
        <div>
          <div className="mb-1.5 flex items-center justify-between">
            <p className="field-label !mb-0">Output</p>
            <DownloadButton onClick={() => downloadJSON(`job-${job.id.slice(0, 8)}-result.json`, job.result)}>
              Download
            </DownloadButton>
          </div>
          <pre className="overflow-x-auto rounded-lg bg-surface-sunken p-3 text-xs text-text-primary">
            {JSON.stringify(job.result, null, 2)}
          </pre>
        </div>
      );
  }
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

      <ResultOutput job={job} />

      <div>
        <p className="field-label">Payload</p>
        <pre className="overflow-x-auto rounded-lg bg-surface-sunken p-3 text-xs text-text-primary">
          {JSON.stringify(job.payload, null, 2)}
        </pre>
      </div>

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
