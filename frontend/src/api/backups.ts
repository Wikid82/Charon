import client from './client';

export interface BackupFile {
  filename: string;
  size: number;
  time: string;
}

export const getBackups = async (): Promise<BackupFile[]> => {
  const response = await client.get<BackupFile[]>('/backups');
  return response.data;
};

export const createBackup = async (): Promise<{ filename: string }> => {
  const response = await client.post<{ filename: string }>('/backups');
  return response.data;
};

export const restoreBackup = async (filename: string): Promise<void> => {
  await client.post(`/backups/${filename}/restore`);
};

export const deleteBackup = async (filename: string): Promise<void> => {
  await client.delete(`/backups/${filename}`);
};
