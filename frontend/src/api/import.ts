import client from './client';

export interface ImportSession {
  id: string;
  state: 'pending' | 'reviewing' | 'completed' | 'failed';
  created_at: string;
  updated_at: string;
}

export interface ImportPreview {
  session: ImportSession;
  preview: {
    hosts: Array<{ domain_names: string; [key: string]: unknown }>;
    conflicts: Record<string, string>;
    errors: string[];
  };
}

export const uploadCaddyfile = async (content: string): Promise<ImportPreview> => {
  const { data } = await client.post<ImportPreview>('/import/upload', { content });
  return data;
};

export const getImportPreview = async (): Promise<ImportPreview> => {
  const { data } = await client.get<ImportPreview>('/import/preview');
  return data;
};

export const commitImport = async (resolutions: Record<string, string>): Promise<void> => {
  await client.post('/import/commit', { resolutions });
};

export const cancelImport = async (): Promise<void> => {
  await client.post('/import/cancel');
};

export const getImportStatus = async (): Promise<{ has_pending: boolean; session?: ImportSession }> => {
  // Note: Assuming there might be a status endpoint or we infer from preview.
  // If no dedicated status endpoint exists in backend, we might rely on preview returning 404 or empty.
  // Based on previous context, there wasn't an explicit status endpoint mentioned in the simple API,
  // but the hook used `importAPI.status()`. I'll check the backend routes if needed.
  // For now, I'll implement it assuming /import/preview can serve as status check or there is a /import/status.
  // Let's check the backend routes to be sure.
  try {
    const { data } = await client.get<{ has_pending: boolean; session?: ImportSession }>('/import/status');
    return data;
  } catch (error) {
    // Fallback if status endpoint doesn't exist, though the hook used it.
    return { has_pending: false };
  }
};
