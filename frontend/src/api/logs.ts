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

  const response = await client.get<LogResponse>(`/logs/${filename}?${params.toString()}`);
  return response.data;
};

export const downloadLog = (filename: string) => {
  // Direct window location change to trigger download
  // We need to use the base URL from the client config if possible,
  // but for now we assume relative path works with the proxy setup
  window.location.href = `/api/v1/logs/${filename}/download`;
};
