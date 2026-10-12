/**
 * Best-effort removal of application backups that a spec created.
 *
 * The CrowdSec import UI creates a real application backup (POST /api/v1/backups) before every
 * import. On the local E2E container /app/data is a tmpfs, so repeated runs against one container
 * fill it. Call `cleanupBackupsCreatedSince` from `test.afterAll` with the time the spec started.
 *
 * This is best effort: `afterAll` does not run when a worker is killed or the run is interrupted,
 * so the larger /app/data tmpfs in docker-compose.playwright-local.yml remains the backstop.
 */
import { existsSync } from 'fs';
import { request as playwrightRequest } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { STORAGE_STATE } from '../constants';

interface BackupEntry {
  filename: string;
  time: string;
}

/**
 * Deletes every backup whose timestamp is at or after `isoTime`.
 * Never throws: cleanup problems must not fail a spec.
 *
 * @returns the number of backups removed
 */
export async function deleteBackupsCreatedSince(
  request: APIRequestContext,
  isoTime: string,
): Promise<number> {
  const since = Date.parse(isoTime);
  let removed = 0;
  try {
    const list = await request.get('/api/v1/backups');
    if (!list.ok()) {
      return 0;
    }
    const backups = (await list.json()) as BackupEntry[];
    for (const backup of backups) {
      if (Date.parse(backup.time) < since) {
        continue;
      }
      const response = await request.delete(`/api/v1/backups/${encodeURIComponent(backup.filename)}`);
      if (response.ok()) {
        removed++;
      }
    }
  } catch {
    // Best effort by design; see the file comment.
  }
  return removed;
}

/**
 * `afterAll` hooks have no test-scoped fixtures, so this opens its own admin API context from the
 * stored auth state (the auth_token cookie authenticates it), runs `deleteBackupsCreatedSince`, and disposes the context.
 */
export async function cleanupBackupsCreatedSince(isoTime: string): Promise<number> {
  if (!existsSync(STORAGE_STATE)) {
    return 0;
  }
  const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
  const context = await playwrightRequest.newContext({
    baseURL,
    storageState: STORAGE_STATE,
  });
  try {
    return await deleteBackupsCreatedSince(context, isoTime);
  } finally {
    await context.dispose();
  }
}
