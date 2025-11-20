import client from './client';

export interface BackupFile {
  name: string;
  size: number;
  mod_time: string;
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

export const deleteBackup = async (_filename: string): Promise<void> => {
    // Note: Delete endpoint wasn't explicitly asked for in the backend implementation plan,
    // but it's good practice. I'll skip implementing the API call for now if the backend doesn't support it yet
    // to avoid 404s, but I should probably add it to the backend later.
    // For now, let's stick to what we built.
    throw new Error("Not implemented");
};
