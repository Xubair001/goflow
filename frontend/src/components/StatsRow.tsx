import type { QueueStats } from "../api/types";
import { StatTile } from "./StatTile";

const EMPTY: QueueStats = {
  Pending: 0,
  Queued: 0,
  Running: 0,
  Completed: 0,
  Dead: 0,
  Cancelled: 0,
};

export function StatsRow({ stats }: { stats: QueueStats | null }) {
  const s = stats ?? EMPTY;
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
      <StatTile label="Pending" value={s.Pending} />
      <StatTile label="Queued" value={s.Queued} />
      <StatTile label="Running" value={s.Running} colorClassName="text-status-running" />
      <StatTile label="Completed" value={s.Completed} colorClassName="text-status-good" />
      <StatTile label="Dead" value={s.Dead} colorClassName="text-status-critical" />
      <StatTile label="Cancelled" value={s.Cancelled} />
    </div>
  );
}
