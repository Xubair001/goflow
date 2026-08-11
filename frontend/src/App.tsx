import { useCallback, useEffect, useState } from "react";
import { api } from "./api/client";
import { useEvents } from "./api/useEvents";
import type { Job, JobStatus, QueueStats } from "./api/types";
import { StatsRow } from "./components/StatsRow";
import { JobForm } from "./components/JobForm";
import { JobTable } from "./components/JobTable";
import { JobDetail } from "./components/JobDetail";

const PAGE_SIZE = 20;

export default function App() {
  const { stats: liveStats, connected } = useEvents();
  const [initialStats, setInitialStats] = useState<QueueStats | null>(null);

  const [jobs, setJobs] = useState<Job[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [statusFilter, setStatusFilter] = useState<JobStatus | "">("");
  const [selected, setSelected] = useState<Job | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  const refreshJobs = useCallback(async () => {
    try {
      const result = await api.listJobs({
        status: statusFilter || undefined,
        limit: PAGE_SIZE,
        offset,
      });
      setJobs(result.jobs);
      setTotal(result.total);
      setLoadError(null);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "Failed to load jobs");
    }
  }, [statusFilter, offset]);

  // Initial render: fetch stats once over plain REST so the tiles aren't
  // empty while waiting for the first SSE tick.
  useEffect(() => {
    api
      .queueStats()
      .then(setInitialStats)
      .catch(() => {
        /* the SSE feed will populate stats shortly regardless */
      });
  }, []);

  useEffect(() => {
    void refreshJobs();
  }, [refreshJobs]);

  // Stats arrive roughly every couple of seconds over SSE (see
  // RunEventsPoller on the backend). There's no per-row event stream, so
  // each tick is treated as "something in the queue may have changed" and
  // triggers a refetch of the current page -- giving the table a live feel
  // without a second, chattier event type.
  useEffect(() => {
    if (liveStats) void refreshJobs();
    // Intentionally reacting only to new stats snapshots, not to
    // refreshJobs identity changes (those are handled by the effect above).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [liveStats]);

  function handleStatusFilterChange(status: JobStatus | "") {
    setStatusFilter(status);
    setOffset(0);
  }

  function handleJobChanged(job: Job) {
    setSelected(job);
    void refreshJobs();
  }

  const closeDetail = useCallback(() => setSelected(null), []);

  // Escape closes the detail panel, matching the slide-over convention used
  // by GitHub/Linear/Vercel for this exact pattern.
  useEffect(() => {
    if (!selected) return;
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") closeDetail();
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [selected, closeDetail]);

  const stats = liveStats ?? initialStats;

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-10 border-b border-border-hairline bg-surface-card/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-3.5">
          <div className="flex items-center gap-2.5">
            <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-accent-primary text-sm font-bold text-accent-primary-foreground">
              G
            </span>
            <div>
              <h1 className="text-base leading-tight font-semibold text-text-primary">GoFlow</h1>
              <p className="text-xs leading-tight text-text-muted">Job queue dashboard</p>
            </div>
          </div>
          <span
            className={`flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ${
              connected ? "bg-status-good/10 text-status-good" : "bg-status-neutral/10 text-text-secondary"
            }`}
          >
            <span
              className={`h-1.5 w-1.5 rounded-full ${connected ? "bg-status-good" : "bg-status-neutral"}`}
              aria-hidden="true"
            />
            {connected ? "Live" : "Connecting…"}
          </span>
        </div>
      </header>

      <main className="mx-auto max-w-6xl space-y-5 p-4">
        <StatsRow stats={stats} />

        <JobForm onCreated={refreshJobs} />

        {loadError && (
          <p className="rounded-lg bg-status-critical/10 px-3 py-2 text-sm text-status-critical">{loadError}</p>
        )}

        <JobTable
          jobs={jobs}
          total={total}
          limit={PAGE_SIZE}
          offset={offset}
          statusFilter={statusFilter}
          onStatusFilterChange={handleStatusFilterChange}
          onPageChange={setOffset}
          onSelect={setSelected}
          onJobChanged={refreshJobs}
          selectedId={selected?.id}
        />
      </main>

      {selected && (
        <div className="fixed inset-0 z-20 flex justify-end">
          <button
            type="button"
            aria-label="Close job detail"
            onClick={closeDetail}
            className="absolute inset-0 animate-[fade-in_150ms_ease-out] bg-slate-950/40"
          />
          <div className="relative flex h-full w-full max-w-md animate-[slide-in_200ms_ease-out] flex-col overflow-y-auto border-l border-border-hairline bg-surface-card p-5 shadow-xl">
            <JobDetail job={selected} onChanged={handleJobChanged} onClose={closeDetail} />
          </div>
        </div>
      )}
    </div>
  );
}
