import { useEffect, useState } from "react";
import type { QueueStats } from "./types";

export interface UseEventsResult {
  stats: QueueStats | null;
  connected: boolean;
}

// useEvents subscribes to the backend's SSE stats feed (see
// backend/internal/api/events.go). EventSource has automatic reconnection
// built into the browser -- it retries on drop with its own backoff -- so
// there's no manual reconnect logic here, just tracking the open/error
// transitions for the UI's live indicator.
export function useEvents(): UseEventsResult {
  const [stats, setStats] = useState<QueueStats | null>(null);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    const source = new EventSource("/api/v1/events");

    source.addEventListener("open", () => setConnected(true));
    source.addEventListener("error", () => setConnected(false));
    source.addEventListener("stats", (event: MessageEvent<string>) => {
      try {
        setStats(JSON.parse(event.data) as QueueStats);
      } catch {
        // Malformed frame; the next tick brings a fresh one.
      }
    });

    return () => source.close();
  }, []);

  return { stats, connected };
}
