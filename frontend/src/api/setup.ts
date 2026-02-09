import client from './client';

/** Status indicating if initial setup is required. */
export interface SetupStatus {
  setupRequired: boolean;
}

/** Request payload for initial setup. */
export interface SetupRequest {
  name: string;
  email: string;
  password: string;
}

/**
 * Checks if initial setup is required.
 * @returns Promise resolving to SetupStatus
 * @throws {AxiosError} If the request fails
 */
export const getSetupStatus = async (): Promise<SetupStatus> => {
  const response = await client.get<SetupStatus>('/setup');
  return response.data;
};

/**
 * Performs initial application setup with admin user creation.
 * @param data - SetupRequest with admin user details
 * @throws {AxiosError} If setup fails or already completed
 */
export const performSetup = async (data: SetupRequest): Promise<void> => {
  await client.post('/setup', data);
};
