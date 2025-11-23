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

export const getMonitorHistory = async (id: string) => {
  const response = await client.get<UptimeHeartbeat[]>(`/uptime/monitors/${id}/history`);
  return response.data;
};
