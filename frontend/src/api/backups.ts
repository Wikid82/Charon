import client from './client';

/** Represents a backup file stored on the server. */
export interface BackupFile {
  filename: string;
  size: number;
  time: string;
}

/**
 * Fetches all available backup files.
 * @returns Promise resolving to array of BackupFile objects
 * @throws {AxiosError} If the request fails
 */
export const getBackups = async (): Promise<BackupFile[]> => {
  const response = await client.get<BackupFile[]>('/backups');
  return response.data;
};

/**
 * Creates a new backup of the current configuration.
 * @returns Promise resolving to object containing the new backup filename
 * @throws {AxiosError} If backup creation fails
 */
export const createBackup = async (): Promise<{ filename: string }> => {
  const response = await client.post<{ filename: string }>('/backups');
  return response.data;
};

/**
 * Restores configuration from a backup file.
 * @param filename - The name of the backup file to restore
 * @throws {AxiosError} If restoration fails or file not found
 */
export const restoreBackup = async (filename: string): Promise<void> => {
  await client.post(`/backups/${filename}/restore`);
};

/**
 * Deletes a backup file.
 * @param filename - The name of the backup file to delete
 * @throws {AxiosError} If deletion fails or file not found
 */
export const deleteBackup = async (filename: string): Promise<void> => {
  await client.delete(`/backups/${filename}`);
};
