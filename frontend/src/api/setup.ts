import client from './client';

export interface SetupStatus {
  setupRequired: boolean;
}

export interface SetupRequest {
  name: string;
  email: string;
  password: string;
}

export const getSetupStatus = async (): Promise<SetupStatus> => {
  const response = await client.get<SetupStatus>('/setup');
  return response.data;
};

export const performSetup = async (data: SetupRequest): Promise<void> => {
  await client.post('/setup', data);
};
