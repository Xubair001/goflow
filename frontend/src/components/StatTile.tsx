interface StatTileProps {
  label: string;
  value: number;
  colorClassName?: string;
}

// Stat tile contract: sentence-case label with no trailing colon, semibold
// value in the default proportional figures (not tabular-nums -- that's
// for aligned columns, not a standalone display number).
export function StatTile({ label, value, colorClassName }: StatTileProps) {
  return (
    <div className="rounded-lg border border-border-hairline bg-surface-card p-4">
      <p className="text-sm text-text-secondary">{label}</p>
      <p className={`mt-1 text-3xl font-semibold ${colorClassName ?? "text-text-primary"}`}>
        {value.toLocaleString()}
      </p>
    </div>
  );
}
