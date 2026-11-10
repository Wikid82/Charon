/**
 * Shared setup for E2E cases that refuse the in-container loopback client.
 *
 * The test runner is always admin-whitelisted, so an access list never refuses it. These cases refuse the loopback
 * client instead (see container-exec.ts) after the admin whitelist is narrowed to RUNNER_WHITELIST_NO_LOOPBACK. The
 * runner's bridge address stays admitted, so each case checks both sides: the loopback client is refused and the runner
 * is not.
 */

import { BlockList, isIP } from 'node:net';
import type { APIRequestContext } from '@playwright/test';
import { getSecurityStatus, setSecurityModuleEnabled } from './security-helpers';
import { RUNNER_WHITELIST_NO_LOOPBACK, withRateLimitRetry } from './security-settings-api';

const LOOPBACK_CIDR = '127.0.0.0/8';

/** Sets the admin whitelist through the config API the admin page uses. */
export async function setAdminWhitelist(requestContext: APIRequestContext, whitelist: string): Promise<void> {
  const response = await withRateLimitRetry(() =>
    requestContext.patch('/api/v1/config', {
      data: {
        security: {
          admin_whitelist: whitelist,
        },
      },
    })
  );

  if (!response.ok()) {
    throw new Error(`Failed to set admin whitelist: ${response.status()}`);
  }
}

/** Parses comma-separated IPv4 CIDRs so membership is decided by the runtime, not by string comparison. */
function ipv4BlockList(cidrs: string): BlockList {
  const list = new BlockList();
  for (const cidr of cidrs.split(',')) {
    const [address, prefix] = cidr.trim().split('/');
    list.addSubnet(address, Number(prefix), 'ipv4');
  }
  return list;
}

/** Returns the IPv4 form of an address the API reports, or undefined for anything else (for example plain IPv6). */
function asIpv4(address: string): string | undefined {
  const bare = address.startsWith('::ffff:') ? address.slice('::ffff:'.length) : address;
  return isIP(bare) === 4 ? bare : undefined;
}

/**
 * Fails unless the runner reaches the API from an address the narrowed whitelist admits and that is not loopback.
 * Without this, the two-sided cases would prove nothing about the runner.
 */
async function assertRunnerIsWhitelistedWithoutLoopback(requestContext: APIRequestContext): Promise<void> {
  const response = await withRateLimitRetry(() => requestContext.get('/api/v1/system/my-ip'));
  if (!response.ok()) {
    throw new Error(`GET /api/v1/system/my-ip failed with ${response.status()}`);
  }
  const { ip } = (await response.json()) as { ip: string };
  const ipv4 = asIpv4(ip);
  const admitted = ipv4 !== undefined && ipv4BlockList(RUNNER_WHITELIST_NO_LOOPBACK).check(ipv4, 'ipv4');
  const loopback = ipv4 !== undefined && ipv4BlockList(LOOPBACK_CIDR).check(ipv4, 'ipv4');
  if (!admitted || loopback) {
    throw new Error(
      `Runner reaches the API from ${ip}, which is loopback or outside ${RUNNER_WHITELIST_NO_LOOPBACK}`
    );
  }
}

/**
 * Prepares a case that refuses the loopback client. Call it in beforeAll, after the caller has captured the security
 * state and before creating any host or list, so the first applied config already holds the narrowed admin set.
 *
 * It turns rate limiting off (polling from the loopback client must not meet a rate_limit handler), narrows the admin
 * whitelist, and proves the runner is admitted without loopback. The caller restores the captured state in afterAll.
 */
export async function narrowAdminWhitelistForLoopbackClient(requestContext: APIRequestContext): Promise<void> {
  await setSecurityModuleEnabled(requestContext, 'rateLimit', false);
  await setAdminWhitelist(requestContext, RUNNER_WHITELIST_NO_LOOPBACK);
  await assertRunnerIsWhitelistedWithoutLoopback(requestContext);
  await getSecurityStatus(requestContext);
}

/**
 * Runs every cleanup step even when an earlier one fails, then rethrows the first failure. A failed delete must not
 * leave the whitelist narrowed or the rate limit off for the specs that follow.
 */
export async function runCleanupSteps(steps: Array<() => Promise<unknown>>): Promise<void> {
  let firstError: unknown;
  let failed = false;
  for (const step of steps) {
    try {
      await step();
    } catch (error) {
      if (!failed) {
        firstError = error;
        failed = true;
      }
    }
  }
  if (failed) {
    throw firstError;
  }
}
