import client from './client';

/** Status indicating if initial setup is required. */
export interface SetupStatus {
  setupRequired: boolean;
  /** True when the first-run setup needs the setup code the server logged at startup. Absent means false. */
  setup_code_required?: boolean;
}

/** Request payload for initial setup. */
export interface SetupRequest {
  name: string;
  email: string;
  password: string;
  /** The setup code from the Charon logs. Sent only when the status requires it. */
  setup_code?: string;
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
 * @param data - SetupRequest with admin user details and, when required, the setup code
 * @throws {AxiosError} If setup fails or already completed
 */
export const performSetup = async (data: SetupRequest): Promise<void> => {
  await client.post('/setup', data);
};
