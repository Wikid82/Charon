import client from './client';

/** Health check response with version and build information. */
export interface HealthResponse {
  status: string;
  service: string;
  version: string;
  git_commit: string;
  build_time: string;
}

/**
 * Checks the health status of the API server.
 * @returns Promise resolving to HealthResponse with version info
 * @throws {AxiosError} If the health check fails
 */
export const checkHealth = async (): Promise<HealthResponse> => {
  const { data } = await client.get<HealthResponse>('/health');
  return data;
};
