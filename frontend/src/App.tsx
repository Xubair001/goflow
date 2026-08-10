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

  const stats = liveStats ?? initialStats;

  return (
    <div className="mx-auto max-w-6xl space-y-4 p-4">
      <header className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-text-primary">GoFlow</h1>
        <span className="flex items-center gap-1.5 text-sm text-text-secondary">
          <span
            className={`h-2 w-2 rounded-full ${connected ? "bg-status-good" : "bg-status-neutral"}`}
            aria-hidden="true"
          />
          {connected ? "Live" : "Connecting…"}
        </span>
      </header>

      <StatsRow stats={stats} />

      <JobForm onCreated={refreshJobs} />

      {loadError && <p className="text-sm text-status-critical">{loadError}</p>}

      <div className="grid gap-4 lg:grid-cols-2">
        <JobTable
          jobs={jobs}
          total={total}
          limit={PAGE_SIZE}
          offset={offset}
          statusFilter={statusFilter}
          onStatusFilterChange={handleStatusFilterChange}
          onPageChange={setOffset}
          onSelect={setSelected}
          selectedId={selected?.id}
        />
        {selected && (
          <JobDetail job={selected} onChanged={handleJobChanged} onClose={() => setSelected(null)} />
        )}
      </div>
    </div>
  );
}
