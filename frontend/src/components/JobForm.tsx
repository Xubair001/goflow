import { useState, type FormEvent } from "react";
import { api } from "../api/client";
import { JOB_TYPES } from "../api/types";

interface JobFormProps {
  onCreated: () => void;
}

export function JobForm({ onCreated }: JobFormProps) {
  const [type, setType] = useState<string>(JOB_TYPES[0]);
  const [payload, setPayload] = useState("{}");
  const [priority, setPriority] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);

    let parsedPayload: unknown;
    try {
      parsedPayload = JSON.parse(payload);
    } catch {
      setError("Payload must be valid JSON");
      return;
    }

    setSubmitting(true);
    try {
      await api.createJob({ type, payload: parsedPayload, priority });
      setPayload("{}");
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to submit job");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="space-y-3 rounded-lg border border-border-hairline bg-surface-card p-4"
    >
      <h2 className="text-sm font-semibold text-text-primary">Submit a job</h2>
      <div className="flex flex-wrap gap-3">
        <label className="flex flex-col gap-1 text-sm text-text-secondary">
          Type
          <select
            value={type}
            onChange={(e) => setType(e.target.value)}
            className="rounded border border-border-hairline bg-surface-page px-2 py-1 text-text-primary"
          >
            {JOB_TYPES.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-sm text-text-secondary">
          Priority
          <input
            type="number"
            value={priority}
            onChange={(e) => setPriority(Number(e.target.value))}
            className="w-24 rounded border border-border-hairline bg-surface-page px-2 py-1 text-text-primary"
          />
        </label>
      </div>
      <label className="flex flex-col gap-1 text-sm text-text-secondary">
        Payload (JSON)
        <textarea
          value={payload}
          onChange={(e) => setPayload(e.target.value)}
          rows={3}
          className="rounded border border-border-hairline bg-surface-page px-2 py-1 font-mono text-sm text-text-primary"
        />
      </label>
      {error && <p className="text-sm text-status-critical">{error}</p>}
      <button
        type="submit"
        disabled={submitting}
        className="rounded bg-status-running px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50"
      >
        {submitting ? "Submitting…" : "Submit job"}
      </button>
    </form>
  );
}
