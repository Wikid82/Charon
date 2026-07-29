import client from './client';

/** A single security-relevant changelog item: a vague-by-design summary plus its source commit. */
export interface SecurityEntry {
  summary: string;
  sha: string;
}

/** A single released version's categorized "What's New" content. */
export interface ChangelogEntry {
  version: string;
  date: string;
  features: string[];
  fixes: string[];
  other: string[];
  security: SecurityEntry[];
}

/** Response shape for GET /changelog/status. */
export interface ChangelogStatus {
  show_changelog: boolean;
  versions: ChangelogEntry[];
}

/** Response shape for GET /changelog/all. */
export interface ChangelogAll {
  versions: ChangelogEntry[];
}

/** Action requested when a user dismisses the "What's New" modal. */
export type ChangelogAckAction = 'dismiss_temporary' | 'dismiss_permanent';

/** Request body for POST /changelog/ack. */
export interface ChangelogAckRequest {
  action: ChangelogAckAction;
  opt_out: boolean;
}

/**
 * Fetches the current user's changelog status — whether the "What's New"
 * modal should be shown, and the unseen version entries to render.
 * @returns Promise resolving to ChangelogStatus
 * @throws {AxiosError} If the request fails
 */
export const getChangelogStatus = async (): Promise<ChangelogStatus> => {
  const response = await client.get<ChangelogStatus>('/changelog/status');
  return response.data;
};

/**
 * Fetches the full changelog history, newest-first, independent of the
 * current user's last-seen version. Used by the Settings "browse" modal.
 * @returns Promise resolving to ChangelogAll
 * @throws {AxiosError} If the request fails
 */
export const getChangelogAll = async (): Promise<ChangelogAll> => {
  const response = await client.get<ChangelogAll>('/changelog/all');
  return response.data;
};

/**
 * Acknowledges/dismisses the "What's New" modal, optionally opting the
 * user out of future automatic changelog notifications.
 * @param req - Dismiss action and opt-out flag
 * @throws {AxiosError} If the request fails
 */
export const ackChangelog = async (req: ChangelogAckRequest): Promise<void> => {
  await client.post('/changelog/ack', req);
};

/**
 * Re-enables automatic "What's New" changelog notifications for the
 * current user.
 * @throws {AxiosError} If the request fails
 */
export const optInChangelog = async (): Promise<void> => {
  await client.post('/changelog/opt-in');
};
