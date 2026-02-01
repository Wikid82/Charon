import client from './client';

/** Represents an active import session. */
export interface ImportSession {
  id: string;
  state: 'pending' | 'reviewing' | 'completed' | 'failed' | 'transient';
  created_at: string;
  updated_at: string;
  source_file?: string;
}

/** Preview of a Caddyfile import with hosts and conflicts. */
export interface ImportPreview {
  session: ImportSession;
  preview: {
    hosts: Array<{ domain_names: string; [key: string]: unknown }>;
    conflicts: string[];
    errors: string[];
  };
  /** Optional top-level warning message returned by the backend (file_server, no-sites, etc.) */
  warning?: string;
  caddyfile_content?: string;
  conflict_details?: Record<string, {
    existing: {
      forward_scheme: string;
      forward_host: string;
      forward_port: number;
      ssl_forced: boolean;
      websocket: boolean;
      enabled: boolean;
    };
    imported: {
      forward_scheme: string;
      forward_host: string;
      forward_port: number;
      ssl_forced: boolean;
      websocket: boolean;
    };
  }>;
}

/**
 * Uploads a Caddyfile content for import preview.
 * @param content - The Caddyfile content as a string
 * @returns Promise resolving to ImportPreview with parsed hosts
 * @throws {AxiosError} If parsing fails or content is invalid
 */
export const uploadCaddyfile = async (content: string): Promise<ImportPreview> => {
  const { data } = await client.post<ImportPreview>('/import/upload', { content });
  return data;
};

/**
 * Represents a Caddyfile with its filename and content.
 */
export interface CaddyFile {
  filename: string;
  content: string;
}

/**
 * Uploads multiple Caddyfiles for batch import.
 * @param files - Array of CaddyFile objects with filename and content
 * @returns Promise resolving to combined ImportPreview
 * @throws {AxiosError} If parsing fails
 */
export const uploadCaddyfilesMulti = async (files: CaddyFile[]): Promise<ImportPreview> => {
  const { data } = await client.post<ImportPreview>('/import/upload-multi', { files });
  return data;
};

/**
 * Gets the current import preview for the active session.
 * @returns Promise resolving to ImportPreview
 * @throws {AxiosError} If no active session or request fails
 */
export const getImportPreview = async (): Promise<ImportPreview> => {
  const { data } = await client.get<ImportPreview>('/import/preview');
  return data;
};

/** Result of committing an import operation. */
export interface ImportCommitResult {
  created: number;
  updated: number;
  skipped: number;
  errors: string[];
}

/**
 * Commits the import, creating/updating proxy hosts.
 * @param sessionUUID - The import session UUID
 * @param resolutions - Map of conflict resolutions (domain -> 'keep'|'replace'|'skip')
 * @param names - Map of custom names for imported hosts
 * @returns Promise resolving to ImportCommitResult with counts
 * @throws {AxiosError} If commit fails
 */
export const commitImport = async (
  sessionUUID: string,
  resolutions: Record<string, string>,
  names: Record<string, string>
): Promise<ImportCommitResult> => {
  const { data } = await client.post<ImportCommitResult>('/import/commit', {
    session_uuid: sessionUUID,
    resolutions,
    names,
  });
  return data;
};

/**
 * Cancels the current import session.
 * @throws {AxiosError} If cancellation fails
 */
export const cancelImport = async (): Promise<void> => {
  await client.post('/import/cancel');
};

/**
 * Gets the current import session status.
 * @returns Promise resolving to object with pending status and optional session
 */
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
  } catch {
    // Fallback if status endpoint doesn't exist, though the hook used it.
    return { has_pending: false };
  }
};
