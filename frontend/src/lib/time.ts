const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 31536000],
  ["month", 2592000],
  ["week", 604800],
  ["day", 86400],
  ["hour", 3600],
  ["minute", 60],
  ["second", 1],
];

const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });

// "5m ago", "2h ago", etc. -- pair with a title attribute holding the exact
// timestamp so precision isn't lost, matching how GitHub/Linear/Vercel show
// relative time in activity feeds.
export function formatRelativeTime(iso: string): string {
  const deltaSeconds = (new Date(iso).getTime() - Date.now()) / 1000;
  const abs = Math.abs(deltaSeconds);
  for (const [unit, secondsInUnit] of UNITS) {
    if (abs >= secondsInUnit || unit === "second") {
      return rtf.format(Math.round(deltaSeconds / secondsInUnit), unit);
    }
  }
  return rtf.format(0, "second");
}
