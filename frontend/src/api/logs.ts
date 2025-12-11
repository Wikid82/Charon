import client from './client';

export interface LogFile {
  name: string;
  size: number;
  mod_time: string;
}

export interface CaddyAccessLog {
  level: string;
  ts: number;
  logger: string;
  msg: string;
  request: {
    remote_ip: string;
    method: string;
    host: string;
    uri: string;
    proto: string;
  };
  status: number;
  duration: number;
  size: number;
}

export interface LogResponse {
  filename: string;
  logs: CaddyAccessLog[];
  total: number;
  limit: number;
  offset: number;
}

export interface LogFilter {
  search?: string;
  host?: string;
  status?: string;
  level?: string;
  limit?: number;
  offset?: number;
  sort?: 'asc' | 'desc';
}

export const getLogs = async (): Promise<LogFile[]> => {
  const response = await client.get<LogFile[]>('/logs');
  return response.data;
};

export const getLogContent = async (filename: string, filter: LogFilter = {}): Promise<LogResponse> => {
  const params = new URLSearchParams();
  if (filter.search) params.append('search', filter.search);
  if (filter.host) params.append('host', filter.host);
  if (filter.status) params.append('status', filter.status);
  if (filter.level) params.append('level', filter.level);
  if (filter.limit) params.append('limit', filter.limit.toString());
  if (filter.offset) params.append('offset', filter.offset.toString());
  if (filter.sort) params.append('sort', filter.sort);

  const response = await client.get<LogResponse>(`/logs/${filename}?${params.toString()}`);
  return response.data;
};

export const downloadLog = (filename: string) => {
  // Direct window location change to trigger download
  // We need to use the base URL from the client config if possible,
  // but for now we assume relative path works with the proxy setup
  window.location.href = `/api/v1/logs/${filename}/download`;
};

export interface LiveLogEntry {
  level: string;
  timestamp: string;
  message: string;
  source?: string;
  data?: Record<string, unknown>;
}

export interface LiveLogFilter {
  level?: string;
  source?: string;
}

/**
 * Connects to the live logs WebSocket endpoint.
 * Returns a function to close the connection.
 */
export const connectLiveLogs = (
  filters: LiveLogFilter,
  onMessage: (log: LiveLogEntry) => void,
  onOpen?: () => void,
  onError?: (error: Event) => void,
  onClose?: () => void
): (() => void) => {
  const params = new URLSearchParams();
  if (filters.level) params.append('level', filters.level);
  if (filters.source) params.append('source', filters.source);

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/api/v1/logs/live?${params.toString()}`;

  console.log('Connecting to WebSocket:', wsUrl);
  const ws = new WebSocket(wsUrl);

  ws.onopen = () => {
    console.log('WebSocket connection established');
    onOpen?.();
  };

  ws.onmessage = (event: MessageEvent) => {
    try {
      const log = JSON.parse(event.data) as LiveLogEntry;
      onMessage(log);
    } catch (err) {
      console.error('Failed to parse log message:', err);
    }
  };

  ws.onerror = (error: Event) => {
    console.error('WebSocket error:', error);
    onError?.(error);
  };

  ws.onclose = (event: CloseEvent) => {
    console.log('WebSocket connection closed', { code: event.code, reason: event.reason, wasClean: event.wasClean });
    onClose?.();
  };

  return () => {
    if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
      ws.close();
    }
  };
};
