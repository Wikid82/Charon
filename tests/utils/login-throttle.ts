/**
 * Login throttle handling for E2E fixtures.
 *
 * Charon throttles sign-in attempts per client (issue #1317; see
 * docs/features/login-protection.md). All Playwright traffic reaches the
 * container from a single peer address, so a fixture-heavy run can exhaust
 * that budget. Fixtures route their login calls through
 * `sendLoginHonoringThrottle` so a 429 is waited out once and, if it
 * persists, fails loudly instead of surfacing as an unrelated downstream
 * flake (for example an empty token).
 */

import type { APIResponse } from '@playwright/test';

/** Error thrown when a login is still throttled after honoring Retry-After once. */
export const LOGIN_THROTTLED_ERROR =
  'login throttled (429): E2E auth budgets too low for this stack?';

/** Upper bound on how long a fixture waits for a throttled login. */
const MAX_RETRY_AFTER_MS = 10_000;

/** Fallback wait when the server omits or sends an unparseable Retry-After. */
const DEFAULT_RETRY_AFTER_MS = 1_000;

/**
 * Convert a response's `Retry-After` header (delta-seconds or HTTP-date) into
 * a wait in milliseconds, clamped to [1 s, 10 s].
 */
export function retryAfterDelayMs(response: APIResponse, nowMs: number = Date.now()): number {
  const raw = response.headers()['retry-after']?.trim();
  let delayMs = DEFAULT_RETRY_AFTER_MS;

  if (raw && /^\d+$/.test(raw)) {
    delayMs = Number(raw) * 1000;
  } else if (raw) {
    const dateMs = Date.parse(raw);
    if (!Number.isNaN(dateMs)) {
      delayMs = dateMs - nowMs;
    }
  }

  return Math.min(MAX_RETRY_AFTER_MS, Math.max(DEFAULT_RETRY_AFTER_MS, delayMs));
}

/**
 * Send a login request. On 429, wait for `Retry-After` (capped at 10 s) and
 * retry once; if the retry is still 429, throw `LOGIN_THROTTLED_ERROR`.
 * Every other status is returned unchanged for the caller to handle.
 */
export async function sendLoginHonoringThrottle(
  send: () => Promise<APIResponse>
): Promise<APIResponse> {
  const first = await send();
  if (first.status() !== 429) {
    return first;
  }

  await new Promise((resolve) => setTimeout(resolve, retryAfterDelayMs(first)));

  const retry = await send();
  if (retry.status() === 429) {
    throw new Error(LOGIN_THROTTLED_ERROR);
  }
  return retry;
}
