import client from './client';

/** Represents a log file on the server. */
export interface LogFile {
  name: string;
  size: number;
  mod_time: string;
}

/** Parsed Caddy access log entry. */
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

/** Paginated log response. */
export interface LogResponse {
  filename: string;
  logs: CaddyAccessLog[];
  total: number;
  limit: number;
  offset: number;
}

/** Filter options for log queries. */
export interface LogFilter {
  search?: string;
  host?: string;
  status?: string;
  level?: string;
  limit?: number;
  offset?: number;
  sort?: 'asc' | 'desc';
}

/**
 * Fetches the list of available log files.
 * @returns Promise resolving to array of LogFile objects
 * @throws {AxiosError} If the request fails
 */
export const getLogs = async (): Promise<LogFile[]> => {
  const response = await client.get<LogFile[]>('/logs');
  return response.data;
};

/**
 * Fetches paginated and filtered log entries from a specific file.
 * @param filename - The log file name to read
 * @param filter - Optional filter and pagination options
 * @returns Promise resolving to LogResponse with entries and metadata
 * @throws {AxiosError} If the request fails or file not found
 */
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

/**
 * Initiates a log file download by redirecting the browser.
 * @param filename - The log file name to download
 */
export const downloadLog = (filename: string) => {
  // Direct window location change to trigger download
  // We need to use the base URL from the client config if possible,
  // but for now we assume relative path works with the proxy setup
  window.location.href = `/api/v1/logs/${filename}/download`;
};

/** Live log entry from WebSocket stream. */
export interface LiveLogEntry {
  level: string;
  timestamp: string;
  message: string;
  source?: string;
  data?: Record<string, unknown>;
}

/** Filter options for live log streaming. */
export interface LiveLogFilter {
  level?: string;
  source?: string;
}

/**
 * SecurityLogEntry represents a security-relevant log entry from Cerberus.
 * This matches the backend SecurityLogEntry struct from /api/v1/cerberus/logs/ws
 */
export interface SecurityLogEntry {
  timestamp: string;
  level: string;
  logger: string;
  client_ip: string;
  method: string;
  uri: string;
  status: number;
  duration: number;
  size: number;
  user_agent: string;
  host: string;
  source: 'waf' | 'crowdsec' | 'ratelimit' | 'acl' | 'normal';
  blocked: boolean;
  block_reason?: string;
  details?: Record<string, unknown>;
}

/**
 * Filters for the Cerberus security logs WebSocket endpoint.
 */
export interface SecurityLogFilter {
  source?: string;       // Filter by security module: waf, crowdsec, ratelimit, acl, normal
  level?: string;        // Filter by log level: info, warn, error
  ip?: string;           // Filter by client IP (partial match)
  host?: string;         // Filter by host (partial match)
  blocked_only?: boolean; // Only show blocked requests
}

/**
 * Connects to the live logs WebSocket endpoint for real-time log streaming.
 * Returns a cleanup function to close the connection.
 * @param filters - LiveLogFilter options for level and source filtering
 * @param onMessage - Callback invoked for each received LiveLogEntry
 * @param onOpen - Optional callback when WebSocket connection is established
 * @param onError - Optional callback on WebSocket error
 * @param onClose - Optional callback when WebSocket connection closes
 * @returns Function to close the WebSocket connection
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

  // Authentication is handled via HttpOnly cookies sent automatically by the browser
  // This prevents tokens from being logged in access logs or exposed to XSS attacks

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

/**
 * Connects to the Cerberus security logs WebSocket endpoint.
 * This streams parsed Caddy access logs with security event annotations.
 *
 * @param filters - Optional filters for source, level, IP, host, and blocked_only
 * @param onMessage - Callback for each received SecurityLogEntry
 * @param onOpen - Callback when connection is established
 * @param onError - Callback on connection error
 * @param onClose - Callback when connection closes
 * @returns A function to close the WebSocket connection
 */
export const connectSecurityLogs = (
  filters: SecurityLogFilter,
  onMessage: (log: SecurityLogEntry) => void,
  onOpen?: () => void,
  onError?: (error: Event) => void,
  onClose?: () => void
): (() => void) => {
  const params = new URLSearchParams();
  if (filters.source) params.append('source', filters.source);
  if (filters.level) params.append('level', filters.level);
  if (filters.ip) params.append('ip', filters.ip);
  if (filters.host) params.append('host', filters.host);
  if (filters.blocked_only) params.append('blocked_only', 'true');

  // Authentication is handled via HttpOnly cookies sent automatically by the browser
  // This prevents tokens from being logged in access logs or exposed to XSS attacks

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/api/v1/cerberus/logs/ws?${params.toString()}`;

  console.log('Connecting to Cerberus logs WebSocket:', wsUrl);
  const ws = new WebSocket(wsUrl);

  ws.onopen = () => {
    console.log('Cerberus logs WebSocket connection established');
    onOpen?.();
  };

  ws.onmessage = (event: MessageEvent) => {
    try {
      const log = JSON.parse(event.data) as SecurityLogEntry;
      onMessage(log);
    } catch (err) {
      console.error('Failed to parse security log message:', err);
    }
  };

  ws.onerror = (error: Event) => {
    console.error('Cerberus logs WebSocket error:', error);
    onError?.(error);
  };

  ws.onclose = (event: CloseEvent) => {
    console.log('Cerberus logs WebSocket closed', { code: event.code, reason: event.reason, wasClean: event.wasClean });
    onClose?.();
  };

  return () => {
    if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
      ws.close();
    }
  };
};
