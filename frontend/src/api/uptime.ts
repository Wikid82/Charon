import client from './client';

export interface UptimeMonitor {
  id: string;
  name: string;
  type: string;
  url: string;
  interval: number;
  enabled: boolean;
  status: string;
  last_check: string;
  latency: number;
  max_retries: number;
}

export interface UptimeHeartbeat {
  id: number;
  monitor_id: string;
  status: string;
  latency: number;
  message: string;
  created_at: string;
}

export const getMonitors = async () => {
  const response = await client.get<UptimeMonitor[]>('/uptime/monitors');
  return response.data;
};

export const getMonitorHistory = async (id: string, limit: number = 50) => {
  const response = await client.get<UptimeHeartbeat[]>(`/uptime/monitors/${id}/history?limit=${limit}`);
  return response.data;
};

export const updateMonitor = async (id: string, data: Partial<UptimeMonitor>) => {
  const response = await client.put<UptimeMonitor>(`/uptime/monitors/${id}`, data);
  return response.data;
};
