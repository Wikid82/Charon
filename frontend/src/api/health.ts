import client from './client';

export interface HealthResponse {
  status: string;
  service: string;
  version: string;
  git_commit: string;
  build_time: string;
}

export const checkHealth = async (): Promise<HealthResponse> => {
  const { data } = await client.get<HealthResponse>('/health');
  return data;
};
