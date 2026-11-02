/**
 * CrowdSec bouncer key robustness - E2E (API level)
 *
 * The key-status and bouncer endpoints report how the bouncer key was checked and where it is
 * held. The additive contract is:
 * - `GET /admin/crowdsec/key-status` returns `key_check` (accepted | rejected | unknown | none);
 *   `valid` is true exactly when `key_check` is `accepted`.
 * - `GET /admin/crowdsec/bouncer` returns `key_source` (env_var | file | memory | none).
 *
 * The assertions accept every documented value, so they hold whether or not the CrowdSec engine
 * is reachable (`key_check` is `none` or `unknown` when no engine answers).
 *
 * @see /projects/Charon/docs/plans/crowdsec_hardening_batch5_spec.md (section 3t.5, 3t.6, 3t.14)
 */

import { test, expect } from '@playwright/test';

const ADMIN = '/api/v1/admin/crowdsec';

const KEY_CHECK_VALUES = ['accepted', 'rejected', 'unknown', 'none'];
const KEY_SOURCE_VALUES = ['env_var', 'file', 'memory', 'none'];

test.describe('CrowdSec key robustness: key check and key source (API)', () => {
  test('key-status reports a key_check whose value agrees with valid', async ({ request }) => {
    const response = await request.get(`${ADMIN}/key-status`);
    expect(response.status()).toBe(200);

    const body = await response.json();
    expect(KEY_CHECK_VALUES).toContain(body.key_check);
    expect(body.valid).toBe(body.key_check === 'accepted');
  });

  test('bouncer info reports a key_source from the allowed set', async ({ request }) => {
    const response = await request.get(`${ADMIN}/bouncer`);
    expect(response.status()).toBe(200);

    const body = await response.json();
    expect(KEY_SOURCE_VALUES).toContain(body.key_source);
  });
});
