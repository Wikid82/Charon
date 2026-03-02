import client from './client';

/** Represents a host parsed from a JSON export. */
export interface JSONHost {
  domain_names: string;
  forward_scheme: string;
  forward_host: string;
  forward_port: number;
  ssl_forced: boolean;
  websocket_support: boolean;
}

/** Preview of a JSON import with hosts and conflicts. */
export interface JSONImportPreview {
  session: {
    id: string;
    state: string;
    source: string;
  };
  preview: {
    hosts: JSONHost[];
    conflicts: string[];
    errors: string[];
  };
  conflict_details: Record<string, {
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

/** Result of committing a JSON import operation. */
export interface JSONImportCommitResult {
  created: number;
  updated: number;
  skipped: number;
  errors: string[];
}

/**
 * Uploads JSON export content for import preview.
 * @param content - The JSON export content as a string
 * @returns Promise resolving to JSONImportPreview with parsed hosts
 * @throws {AxiosError} If parsing fails or content is invalid
 */
export const uploadJSONExport = async (content: string): Promise<JSONImportPreview> => {
  const { data } = await client.post<JSONImportPreview>('/import/json/upload', { content });
  return data;
};

/**
 * Commits the JSON import, creating/updating proxy hosts.
 * @param sessionUuid - The import session UUID
 * @param resolutions - Map of conflict resolutions (domain -> 'keep'|'replace'|'skip')
 * @param names - Map of custom names for imported hosts
 * @returns Promise resolving to JSONImportCommitResult with counts
 * @throws {AxiosError} If commit fails
 */
export const commitJSONImport = async (
  sessionUuid: string,
  resolutions: Record<string, string>,
  names: Record<string, string>
): Promise<JSONImportCommitResult> => {
  const { data } = await client.post<JSONImportCommitResult>('/import/json/commit', {
    session_uuid: sessionUuid,
    resolutions,
    names,
  });
  return data;
};

/**
 * Cancels the current JSON import session.
 * @param sessionUuid - The import session UUID
 * @throws {AxiosError} If cancellation fails
 */
export const cancelJSONImport = async (sessionUuid: string): Promise<void> => {
  await client.post('/import/json/cancel', {
    session_uuid: sessionUuid,
  });
};
