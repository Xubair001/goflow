import { useState, type ChangeEvent, type FormEvent } from "react";
import { api } from "../api/client";
import {
  JOB_TYPES,
  JOB_TYPE_LABELS,
  type HttpMethod,
  type JobType,
} from "../api/types";

interface JobFormProps {
  onCreated: () => void;
}

type IntervalUnit = "seconds" | "minutes" | "hours";
type ImageSource = "upload" | "url";

const INTERVAL_SECONDS: Record<IntervalUnit, number> = {
  seconds: 1,
  minutes: 60,
  hours: 3600,
};

export function JobForm({ onCreated }: JobFormProps) {
  const [type, setType] = useState<JobType>("send_email");
  const [priority, setPriority] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [justSubmitted, setJustSubmitted] = useState(false);

  // send_email
  const [to, setTo] = useState("");
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");

  // resize_image
  const [imageSource, setImageSource] = useState<ImageSource>("upload");
  const [imagePreview, setImagePreview] = useState<string | null>(null);
  const [uploadedId, setUploadedId] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [imageUrl, setImageUrl] = useState("");
  const [width, setWidth] = useState(200);
  const [height, setHeight] = useState(200);

  // process_csv
  const [csvData, setCsvData] = useState("name,value\nAlice,90\nBob,80");

  // make_http_request
  const [httpUrl, setHttpUrl] = useState("https://example.com");
  const [httpMethod, setHttpMethod] = useState<HttpMethod>("GET");
  const [httpBody, setHttpBody] = useState("");

  // generate_report
  const [failedSampleSize, setFailedSampleSize] = useState("");

  // scheduled_task
  const [taskMessage, setTaskMessage] = useState("tick");
  const [intervalValue, setIntervalValue] = useState(60);
  const [intervalUnit, setIntervalUnit] = useState<IntervalUnit>("seconds");

  async function handleImageFileChange(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    setImagePreview(URL.createObjectURL(file));
    setUploadedId(null);
    setUploading(true);
    setError(null);
    try {
      const res = await api.uploadFile(file);
      setUploadedId(res.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to upload image");
    } finally {
      setUploading(false);
    }
  }

  async function handleCsvFileChange(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setCsvData(await file.text());
  }

  function buildPayload(): { payload: unknown; error?: string } {
    switch (type) {
      case "send_email":
        if (!to) return { payload: null, error: '"To" is required' };
        return { payload: { to, subject, body } };

      case "resize_image":
        if (width <= 0 || height <= 0) {
          return { payload: null, error: "Width and height must be positive" };
        }
        if (imageSource === "upload") {
          if (!uploadedId) return { payload: null, error: "Upload an image first" };
          return { payload: { upload_id: uploadedId, width, height } };
        }
        if (!imageUrl) return { payload: null, error: "Image URL is required" };
        return { payload: { source_url: imageUrl, width, height } };

      case "process_csv":
        if (!csvData.trim()) return { payload: null, error: "CSV data is required" };
        return { payload: { csv_data: csvData } };

      case "make_http_request":
        if (!httpUrl) return { payload: null, error: "URL is required" };
        return {
          payload: {
            url: httpUrl,
            method: httpMethod,
            ...(httpBody ? { body: httpBody } : {}),
          },
        };

      case "generate_report":
        return {
          payload: failedSampleSize === "" ? {} : { failed_sample_size: Number(failedSampleSize) },
        };

      case "scheduled_task":
        if (intervalValue <= 0) return { payload: null, error: "Interval must be positive" };
        return {
          payload: {
            message: taskMessage,
            interval_seconds: intervalValue * INTERVAL_SECONDS[intervalUnit],
          },
        };
    }
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setJustSubmitted(false);

    const { payload, error: validationError } = buildPayload();
    if (validationError) {
      setError(validationError);
      return;
    }

    setSubmitting(true);
    try {
      await api.createJob({ type, payload, priority });
      setJustSubmitted(true);
      onCreated();
      setTimeout(() => setJustSubmitted(false), 2500);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to submit job");
    } finally {
      setSubmitting(false);
    }
  }

  const submitDisabled = submitting || (type === "resize_image" && imageSource === "upload" && uploading);

  return (
    <form onSubmit={handleSubmit} className="card space-y-4 p-5">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-text-primary">Submit a job</h2>
        {justSubmitted && (
          <span className="flex items-center gap-1 text-sm font-medium text-status-good">
            <span aria-hidden="true">✓</span> Submitted
          </span>
        )}
      </div>

      <div className="flex flex-wrap gap-1 rounded-lg bg-surface-sunken p-1">
        {JOB_TYPES.map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => setType(t)}
            className={type === t ? "tab-btn-active" : "tab-btn-inactive"}
          >
            {JOB_TYPE_LABELS[t]}
          </button>
        ))}
      </div>

      {type === "send_email" && (
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="sm:col-span-1">
            <span className="field-label">To</span>
            <input
              type="email"
              required
              value={to}
              onChange={(e) => setTo(e.target.value)}
              placeholder="someone@example.com"
              className="field"
            />
          </label>
          <label className="sm:col-span-1">
            <span className="field-label">Subject</span>
            <input
              type="text"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              placeholder="Hello!"
              className="field"
            />
          </label>
          <label className="sm:col-span-2">
            <span className="field-label">Body</span>
            <textarea
              value={body}
              onChange={(e) => setBody(e.target.value)}
              rows={3}
              placeholder="Message body…"
              className="field"
            />
          </label>
        </div>
      )}

      {type === "resize_image" && (
        <div className="space-y-3">
          <div className="flex gap-1 rounded-lg bg-surface-sunken p-1 w-fit">
            <button
              type="button"
              onClick={() => setImageSource("upload")}
              className={imageSource === "upload" ? "tab-btn-active" : "tab-btn-inactive"}
            >
              Upload a file
            </button>
            <button
              type="button"
              onClick={() => setImageSource("url")}
              className={imageSource === "url" ? "tab-btn-active" : "tab-btn-inactive"}
            >
              Paste a URL
            </button>
          </div>

          <div className="flex items-start gap-4">
            {imagePreview && (
              <img
                src={imagePreview}
                alt="Selected preview"
                className="h-20 w-20 rounded-lg border border-border-hairline object-cover"
              />
            )}
            <div className="flex-1 space-y-3">
              {imageSource === "upload" ? (
                <label className="block">
                  <span className="field-label">Image file</span>
                  <input
                    type="file"
                    accept="image/png,image/jpeg,image/gif,image/webp"
                    onChange={handleImageFileChange}
                    className="field file:mr-3 file:rounded-md file:border-0 file:bg-accent-primary file:px-3 file:py-1.5 file:text-sm file:font-medium file:text-accent-primary-foreground"
                  />
                  {uploading && <p className="mt-1 text-xs text-text-muted">Uploading…</p>}
                  {uploadedId && !uploading && (
                    <p className="mt-1 text-xs text-status-good">Ready to resize</p>
                  )}
                </label>
              ) : (
                <label className="block">
                  <span className="field-label">Image URL</span>
                  <input
                    type="url"
                    value={imageUrl}
                    onChange={(e) => setImageUrl(e.target.value)}
                    placeholder="https://example.com/photo.jpg"
                    className="field"
                  />
                </label>
              )}
              <div className="flex gap-3">
                <label>
                  <span className="field-label">Width</span>
                  <input
                    type="number"
                    min={1}
                    value={width}
                    onChange={(e) => setWidth(Number(e.target.value))}
                    className="field w-28"
                  />
                </label>
                <label>
                  <span className="field-label">Height</span>
                  <input
                    type="number"
                    min={1}
                    value={height}
                    onChange={(e) => setHeight(Number(e.target.value))}
                    className="field w-28"
                  />
                </label>
              </div>
            </div>
          </div>
        </div>
      )}

      {type === "process_csv" && (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="field-label !mb-0">CSV data</span>
            <label className="btn-secondary btn-sm cursor-pointer">
              Upload .csv file
              <input type="file" accept=".csv,text/csv" onChange={handleCsvFileChange} className="hidden" />
            </label>
          </div>
          <textarea
            value={csvData}
            onChange={(e) => setCsvData(e.target.value)}
            rows={5}
            className="field font-mono"
          />
        </div>
      )}

      {type === "make_http_request" && (
        <div className="space-y-3">
          <div className="flex gap-3">
            <label className="w-32">
              <span className="field-label">Method</span>
              <select
                value={httpMethod}
                onChange={(e) => setHttpMethod(e.target.value as HttpMethod)}
                className="field"
              >
                {(["GET", "POST", "PUT", "PATCH", "DELETE"] as const).map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
            </label>
            <label className="flex-1">
              <span className="field-label">URL</span>
              <input
                type="url"
                value={httpUrl}
                onChange={(e) => setHttpUrl(e.target.value)}
                placeholder="https://api.example.com/resource"
                className="field"
              />
            </label>
          </div>
          {httpMethod !== "GET" && httpMethod !== "DELETE" && (
            <label className="block">
              <span className="field-label">Body (optional)</span>
              <textarea
                value={httpBody}
                onChange={(e) => setHttpBody(e.target.value)}
                rows={3}
                className="field font-mono"
              />
            </label>
          )}
        </div>
      )}

      {type === "generate_report" && (
        <label className="block w-56">
          <span className="field-label">Failed sample size (optional)</span>
          <input
            type="number"
            min={1}
            value={failedSampleSize}
            onChange={(e) => setFailedSampleSize(e.target.value)}
            placeholder="Default: 5"
            className="field"
          />
        </label>
      )}

      {type === "scheduled_task" && (
        <div className="grid gap-3 sm:grid-cols-2">
          <label>
            <span className="field-label">Message</span>
            <input
              type="text"
              value={taskMessage}
              onChange={(e) => setTaskMessage(e.target.value)}
              className="field"
            />
          </label>
          <label>
            <span className="field-label">Repeat every</span>
            <div className="flex gap-2">
              <input
                type="number"
                min={1}
                value={intervalValue}
                onChange={(e) => setIntervalValue(Number(e.target.value))}
                className="field w-20"
              />
              <select
                value={intervalUnit}
                onChange={(e) => setIntervalUnit(e.target.value as IntervalUnit)}
                className="field"
              >
                <option value="seconds">Seconds</option>
                <option value="minutes">Minutes</option>
                <option value="hours">Hours</option>
              </select>
            </div>
          </label>
        </div>
      )}

      <div className="flex items-center justify-between border-t border-border-hairline pt-4">
        <label className="flex items-center gap-2 text-sm text-text-secondary">
          Priority
          <input
            type="number"
            value={priority}
            onChange={(e) => setPriority(Number(e.target.value))}
            className="field w-20"
          />
        </label>
        <button type="submit" disabled={submitDisabled} className="btn-primary">
          {submitting ? "Submitting…" : "Submit job"}
        </button>
      </div>

      {error && <p className="text-sm text-status-critical">{error}</p>}
    </form>
  );
}
