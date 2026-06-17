import { useEffect, useState } from 'react';

import { connectStatsWebSocket } from '../api/stats';
import type { StatsPushMessage, StatsSummary } from '../api/stats';

export interface UseStatsWebSocketResult {
  connected: boolean;
  lastMessage: StatsPushMessage | null;
  latestSummary: StatsSummary | null;
}

/**
 * Connects to the stats WebSocket endpoint and tracks real-time summary updates.
 * Cleans up the WebSocket connection on unmount.
 */
export function useStatsWebSocket(): UseStatsWebSocketResult {
  const [connected, setConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState<StatsPushMessage | null>(null);
  const [latestSummary, setLatestSummary] = useState<StatsSummary | null>(null);

  useEffect(() => {
    const cleanup = connectStatsWebSocket(
      (msg) => {
        setLastMessage(msg);
        if (msg.type === 'stats_update') {
          setLatestSummary(msg.data);
        }
      },
      () => {
        setConnected(true);
      },
      undefined,
      () => {
        setConnected(false);
      }
    );

    return cleanup;
  }, []);

  return { connected, lastMessage, latestSummary };
}
