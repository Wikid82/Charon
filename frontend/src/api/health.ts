import client from './client';

export interface HealthResponse {
  status: string;
  service: string;
}

export const checkHealth = async (): Promise<HealthResponse> => {
  const { data } = await client.get<HealthResponse>('/health');
  return data;
};
