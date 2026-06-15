import client from './client';

/** Aggregate request counts over rolling time windows. */
export interface StatsSummary {
  requests_last_24h: number;
  requests_last_7d: number;
  requests_last_30d: number;
}

/** Per-host request count for a given period. */
export interface HostStat {
  host_id: string;
  hostname: string;
  count: number;
}

/** HTTP status code distribution entry. */
export interface StatusStat {
  code: number;
  count: number;
}

/** Traffic volume data point for a time bucket. */
export interface TrafficBucket {
  /** ISO 8601 timestamp string marking the start of this bucket. */
  bucket: string;
  bytes_sent: number;
}

/** TLS certificate expiry information for a proxy host. */
export interface CertExpiry {
  host_id: string;
  hostname: string;
  /** ISO 8601 string of the certificate expiry date. */
  expires_at: string;
  days_left: number;
}

/** Stats ingestion health metrics. */
export interface StatsHealth {
  dropped_count: number;
}

/** Rolling time window for stats queries. */
export type StatsPeriod = '24h' | '7d' | '30d';

/** Bucket granularity for time-series stats queries. */
export type StatsBucket = '1h' | '6h' | '1d';

/** WebSocket push message from the stats hub. */
export interface StatsPushMessage {
  type: 'stats_update';
  data: StatsSummary;
}

/**
 * Fetches aggregate request counts for the last 24h, 7d, and 30d.
 * @returns Promise resolving to StatsSummary
 * @throws {AxiosError} If the request fails
 */
export const getStatsSummary = async (): Promise<StatsSummary> => {
  const { data } = await client.get<StatsSummary>('/stats/summary');
  return data;
};

/**
 * Fetches the top hosts by request count for the given period.
 * @param period - Rolling time window: '24h', '7d', or '30d'
 * @param limit - Maximum number of hosts to return (default: 10)
 * @returns Promise resolving to array of HostStat objects
 * @throws {AxiosError} If the request fails
 */
export const getTopHosts = async (period: StatsPeriod, limit = 10): Promise<HostStat[]> => {
  const { data } = await client.get<HostStat[]>(`/stats/top-hosts?period=${period}&limit=${limit}`);
  return data;
};

/**
 * Fetches HTTP status code distribution for the given period.
 * @param period - Rolling time window: '24h', '7d', or '30d'
 * @returns Promise resolving to array of StatusStat objects
 * @throws {AxiosError} If the request fails
 */
export const getStatusDistribution = async (period: StatsPeriod): Promise<StatusStat[]> => {
  const { data } = await client.get<StatusStat[]>(`/stats/status-distribution?period=${period}`);
  return data;
};

/**
 * Fetches traffic volume time-series data for the given bucket granularity.
 * @param bucket - Bucket size: '1h', '6h', or '1d'
 * @returns Promise resolving to array of TrafficBucket objects
 * @throws {AxiosError} If the request fails
 */
export const getTrafficVolume = async (bucket: StatsBucket): Promise<TrafficBucket[]> => {
  const { data } = await client.get<TrafficBucket[]>(`/stats/traffic-volume?bucket=${bucket}`);
  return data;
};

/**
 * Fetches TLS certificate expiry information for hosts expiring within the given window.
 * @param withinDays - Number of days to look ahead (default: 30)
 * @returns Promise resolving to array of CertExpiry objects
 * @throws {AxiosError} If the request fails
 */
export const getCertExpiry = async (withinDays = 30): Promise<CertExpiry[]> => {
  const { data } = await client.get<CertExpiry[]>(`/stats/cert-expiry?within_days=${withinDays}`);
  return data;
};

/**
 * Fetches request count time-series data for the given bucket granularity.
 * @param bucket - Bucket size: '1h', '6h', or '1d'
 * @returns Promise resolving to array of TrafficBucket objects
 * @throws {AxiosError} If the request fails
 */
export const getRequests = async (bucket: StatsBucket): Promise<TrafficBucket[]> => {
  const { data } = await client.get<TrafficBucket[]>(`/stats/requests?bucket=${bucket}`);
  return data;
};

/**
 * Fetches stats ingestion health metrics.
 * @returns Promise resolving to StatsHealth with dropped_count
 * @throws {AxiosError} If the request fails
 */
export const getStatsHealth = async (): Promise<StatsHealth> => {
  const { data } = await client.get<StatsHealth>('/stats/health');
  return data;
};

/**
 * Connects to the stats WebSocket endpoint for real-time summary updates.
 * Authentication is handled via HttpOnly cookies sent automatically by the browser.
 * @param onMessage - Callback invoked for each StatsPushMessage received
 * @param onOpen - Optional callback when the connection is established
 * @param onError - Optional callback on WebSocket error
 * @param onClose - Optional callback when the connection closes
 * @returns Function to close the WebSocket connection
 */
export const connectStatsWebSocket = (
  onMessage: (msg: StatsPushMessage) => void,
  onOpen?: () => void,
  onError?: (error: Event) => void,
  onClose?: () => void
): (() => void) => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/api/v1/stats/ws`;

  const ws = new WebSocket(wsUrl);

  ws.onopen = () => {
    onOpen?.();
  };

  ws.onmessage = (event: MessageEvent) => {
    try {
      const msg = JSON.parse(event.data as string) as StatsPushMessage;
      onMessage(msg);
    } catch (err) {
      console.error('Failed to parse stats WebSocket message:', err);
    }
  };

  ws.onerror = (error: Event) => {
    console.error('Stats WebSocket error:', error);
    onError?.(error);
  };

  ws.onclose = (event: CloseEvent) => {
    console.log('Stats WebSocket closed', { code: event.code, reason: event.reason, wasClean: event.wasClean });
    onClose?.();
  };

  return () => {
    if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
      ws.close();
    }
  };
};
