import client from './client';

/** Represents a host parsed from an NPM export. */
export interface NPMHost {
  domain_names: string;
  forward_scheme: string;
  forward_host: string;
  forward_port: number;
  ssl_forced: boolean;
  websocket_support: boolean;
}

/** Preview of an NPM import with hosts and conflicts. */
export interface NPMImportPreview {
  session: {
    id: string;
    state: string;
    source: string;
  };
  preview: {
    hosts: NPMHost[];
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

/** Result of committing an NPM import operation. */
export interface NPMImportCommitResult {
  created: number;
  updated: number;
  skipped: number;
  errors: string[];
}

/**
 * Uploads NPM export content for import preview.
 * @param content - The NPM export JSON content as a string
 * @returns Promise resolving to NPMImportPreview with parsed hosts
 * @throws {AxiosError} If parsing fails or content is invalid
 */
export const uploadNPMExport = async (content: string): Promise<NPMImportPreview> => {
  const { data } = await client.post<NPMImportPreview>('/import/npm/upload', { content });
  return data;
};

/**
 * Commits the NPM import, creating/updating proxy hosts.
 * @param sessionUuid - The import session UUID
 * @param resolutions - Map of conflict resolutions (domain -> 'keep'|'replace'|'skip')
 * @param names - Map of custom names for imported hosts
 * @returns Promise resolving to NPMImportCommitResult with counts
 * @throws {AxiosError} If commit fails
 */
export const commitNPMImport = async (
  sessionUuid: string,
  resolutions: Record<string, string>,
  names: Record<string, string>
): Promise<NPMImportCommitResult> => {
  const { data } = await client.post<NPMImportCommitResult>('/import/npm/commit', {
    session_uuid: sessionUuid,
    resolutions,
    names,
  });
  return data;
};

/**
 * Cancels the current NPM import session.
 * @param sessionUuid - The import session UUID
 * @throws {AxiosError} If cancellation fails
 */
export const cancelNPMImport = async (sessionUuid: string): Promise<void> => {
  await client.post('/import/npm/cancel', {
    session_uuid: sessionUuid,
  });
};
