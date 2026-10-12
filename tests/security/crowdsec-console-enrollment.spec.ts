/**
 * CrowdSec Console Enrollment E2E Tests
 *
 * The E2E container has no CrowdSec engine and the enrollment feature flag is off by
 * default, so the enrollment API is stubbed (see tests/utils/crowdsec-stubs.ts) and the
 * assertions run against known enrollment states:
 * - What the card shows for each state (not enrolled, enrolling, pending acceptance,
 *   enrolled, failed) including timestamps, heartbeat and the last error
 * - Validation before a request is sent
 * - The exact enroll, rotate, retry and re-enroll requests
 * - Outcomes: refreshed status, toasts and server errors
 * - Clearing the local enrollment state
 * - Gating on local API readiness
 *
 * The diagnostics endpoints are covered by crowdsec-diagnostics.spec.ts. The UI does not
 * call the console heartbeat endpoint; heartbeat is read from the console status.
 *
 * @see /projects/Charon/docs/plans/crowdsec_enrollment_debug_spec.md
 */

import type { Page } from '@playwright/test';
import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { getToastLocator } from '../utils/ui-helpers';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import {
  CROWDSEC_ROUTES,
  crowdsecFixtures,
  stubCrowdSecApi,
  type ConsoleEnrollmentState,
  type CrowdSecStubOptions,
} from '../utils/crowdsec-stubs';

/** The LAPI readiness warnings and gating only appear after the page's 3 second start-up grace period. */
const GATING_TIMEOUT = 15_000;
const KEY = 'stub-enrollment-key';

const NOT_ENROLLED = crowdsecFixtures.notEnrolled();
const ENROLLED = crowdsecFixtures.enrollment();
const PENDING = crowdsecFixtures.enrollment({ status: 'pending_acceptance' });
const ENROLLING = crowdsecFixtures.enrollment({ status: 'enrolling' });
const FAILED = crowdsecFixtures.enrollment({ status: 'failed', last_error: 'key rejected by console' });

const card = (page: Page) => page.getByTestId('console-enrollment-card');
const tokenField = (page: Page) => page.getByTestId('console-enrollment-token');
const agentField = (page: Page) => page.getByTestId('console-agent-name');
const tenantField = (page: Page) => page.getByTestId('console-tenant');
const enrollButton = (page: Page) => card(page).getByRole('button', { name: 'Enroll', exact: true });
const rotateButton = (page: Page) => card(page).getByRole('button', { name: 'Rotate Key', exact: true });
const retryButton = (page: Page) => card(page).getByRole('button', { name: 'Retry Enrollment', exact: true });
const hostname = (page: Page) => new URL(page.url()).hostname;

/** Local time formatting exactly as the page renders a timestamp. */
const localTime = (page: Page, iso: string) => page.evaluate((value) => new Date(value).toLocaleString(), iso);

/** Installs the stubs (feature enabled, CrowdSec running) and loads the CrowdSec page. */
async function openEnrollment(
  page: Page,
  state: ConsoleEnrollmentState,
  options: CrowdSecStubOptions = {},
  beforeLoad?: () => Promise<void>,
) {
  const recorder = await stubCrowdSecApi(page, {
    consoleEnrollmentEnabled: true,
    consoleStatus: state,
    status: crowdsecFixtures.runningStatus(),
    ...options,
  });
  await beforeLoad?.();
  await page.goto('/security/crowdsec');
  await waitForLoadingComplete(page);
  await expect(card(page)).toBeVisible();
  return recorder;
}

/**
 * Serves `console/status` from a mutable state and records POST /console/enroll bodies and
 * DELETE /console/enrollment calls. Registered after the base stubs so it takes precedence.
 * `enrollResponse` is applied to the state when an enrollment succeeds.
 */
async function trackConsole(
  page: Page,
  initial: ConsoleEnrollmentState,
  behaviour: { enrollResponse?: ConsoleEnrollmentState; enrollError?: { status: number; error: string } } = {},
) {
  const tracker = { state: initial, enrollments: [] as unknown[], clears: 0 };
  await page.route(CROWDSEC_ROUTES.consoleStatus, (route) =>
    route.request().method() === 'GET'
      ? route.fulfill({ status: 200, contentType: 'application/json', json: tracker.state })
      : route.fallback(),
  );
  await page.route(CROWDSEC_ROUTES.consoleEnroll, (route) => {
    if (route.request().method() !== 'POST') return route.fallback();
    tracker.enrollments.push(route.request().postDataJSON());
    if (behaviour.enrollError) {
      return route.fulfill({
        status: behaviour.enrollError.status,
        contentType: 'application/json',
        json: { error: behaviour.enrollError.error },
      });
    }
    tracker.state = behaviour.enrollResponse ?? tracker.state;
    return route.fulfill({ status: 200, contentType: 'application/json', json: tracker.state });
  });
  await page.route(CROWDSEC_ROUTES.consoleEnrollment, (route) => {
    if (route.request().method() !== 'DELETE') return route.fallback();
    tracker.clears += 1;
    tracker.state = NOT_ENROLLED;
    return route.fulfill({ status: 200, contentType: 'application/json', json: { message: 'enrollment state cleared' } });
  });
  return tracker;
}

/** Opens the page with the tracked console API in place of the plain status stub. */
async function openTracked(
  page: Page,
  initial: ConsoleEnrollmentState,
  behaviour: Parameters<typeof trackConsole>[2] = {},
  options: CrowdSecStubOptions = {},
) {
  let tracker!: Awaited<ReturnType<typeof trackConsole>>;
  // Tracking routes must be registered after the base stubs so they take precedence.
  await openEnrollment(page, initial, options, async () => {
    tracker = await trackConsole(page, initial, behaviour);
  });
  return tracker;
}

test.describe('CrowdSec Console Enrollment', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
  });

  test.describe('Enrollment States', () => {
    const STATES = [
      {
        name: 'not enrolled',
        state: NOT_ENROLLED,
        label: 'Status: not enrolled',
        token: 'Not stored',
        rotate: false,
        retry: false,
        reenroll: false,
        pending: false,
      },
      {
        name: 'enrolling',
        state: ENROLLING,
        label: 'Status: enrolling',
        token: 'Stored (masked)',
        rotate: false,
        retry: false,
        reenroll: false,
        pending: false,
      },
      {
        name: 'pending acceptance',
        state: PENDING,
        label: 'Status: pending acceptance',
        token: 'Stored (masked)',
        rotate: true,
        retry: false,
        reenroll: true,
        pending: true,
      },
      {
        name: 'enrolled',
        state: ENROLLED,
        label: 'Status: enrolled',
        token: 'Stored (masked)',
        rotate: true,
        retry: false,
        reenroll: true,
        pending: false,
      },
      {
        name: 'failed',
        state: FAILED,
        label: 'Status: degraded',
        token: 'Stored (masked)',
        rotate: true,
        retry: true,
        reenroll: false,
        pending: false,
      },
    ];

    for (const { name, state, label, token, rotate, retry, reenroll, pending } of STATES) {
      test(`should show the ${name} state and only the actions that apply`, async ({ page }) => {
        await openEnrollment(page, state);

        await expect(page.getByTestId('console-status-label')).toHaveText(label);
        await expect(page.getByTestId('console-token-state')).toHaveText(token);
        await expect(enrollButton(page)).toBeVisible();
        if (rotate) {
          await expect(rotateButton(page)).toBeEnabled();
        } else {
          await expect(rotateButton(page)).toBeDisabled();
        }
        await expect(retryButton(page)).toHaveCount(retry ? 1 : 0);
        await expect(page.getByTestId('reenroll-section')).toHaveCount(reenroll ? 1 : 0);
        await expect(page.getByTestId('pending-acceptance-info')).toHaveCount(pending ? 1 : 0);
      });
    }

    test('should show the agent, tenant and every timestamp of an enrolled agent', async ({ page }) => {
      const enrollment = crowdsecFixtures.enrollment({
        last_attempt_at: '2026-01-01T00:00:00Z',
        enrolled_at: '2026-01-01T00:01:00Z',
        last_heartbeat_at: '2026-01-02T03:04:05Z',
        correlation_id: 'corr-1234',
      });
      await openEnrollment(page, enrollment);

      await expect(card(page).getByText(`Last heartbeat: ${await localTime(page, '2026-01-02T03:04:05Z')}`, { exact: true })).toBeVisible();
      await expect(card(page).getByText(`Last attempt: ${await localTime(page, '2026-01-01T00:00:00Z')}`, { exact: true })).toBeVisible();
      await expect(card(page).getByText(`Enrolled: ${await localTime(page, '2026-01-01T00:01:00Z')}`, { exact: true })).toBeVisible();
      await expect(card(page).getByText('Correlation ID: corr-1234', { exact: true })).toBeVisible();
      await expect(card(page).getByText('e2e-agent', { exact: true })).toBeVisible();
      await expect(card(page).getByText('e2e-tenant', { exact: true }).first()).toBeVisible();
    });

    test('should show dashes for the times of an agent that was never enrolled', async ({ page }) => {
      await openEnrollment(page, NOT_ENROLLED);

      await expect(card(page).getByText('Last heartbeat: —', { exact: true })).toBeVisible();
      await expect(card(page).getByText('Last attempt: —', { exact: true })).toBeVisible();
      await expect(card(page).getByText('Enrolled: —', { exact: true })).toBeVisible();
      await expect(card(page).getByText(/Correlation ID/)).toHaveCount(0);
    });

    test('should mask secret-looking words in the last error', async ({ page }) => {
      await openEnrollment(
        page,
        crowdsecFixtures.enrollment({ status: 'failed', last_error: 'rejected key abcdefghij1234567890 by console' }),
      );

      const error = page.getByTestId('console-status-error');
      await expect(error).toContainText('rejected key *** by console');
      await expect(error).not.toContainText('abcdefghij1234567890');
    });

    test('should fall back to the not enrolled view when the status cannot be read', async ({ page }) => {
      await openEnrollment(page, NOT_ENROLLED);
      await page.route(CROWDSEC_ROUTES.consoleStatus, (route) =>
        route.fulfill({ status: 500, contentType: 'application/json', json: { error: 'failed to read enrollment status' } }),
      );
      await page.reload();
      await waitForLoadingComplete(page);

      await expect(page.getByTestId('console-status-label')).toHaveText('Status: not enrolled');
      await expect(page.getByTestId('console-token-state')).toHaveText('—');
      await expect(rotateButton(page)).toBeDisabled();
    });

    test('should describe the card for assistive technology', async ({ page }) => {
      await openEnrollment(page, NOT_ENROLLED);

      await expect(card(page)).toMatchAriaSnapshot(`
        - heading "Console Enrollment" [level=3]
        - textbox "Agent Name"
        - textbox "Tenant/Organization (optional)"
        - checkbox "I understand this will rotate my LAPI credentials if already enrolled"
        - button "Enroll" [disabled]
        - button "Rotate Key" [disabled]
      `);
    });
  });

  test.describe('Token Handling', () => {
    test('should mask the token as it is typed and allow revealing it', async ({ page }) => {
      await openEnrollment(page, NOT_ENROLLED);

      await tokenField(page).fill(KEY);
      await expect(tokenField(page)).toHaveAttribute('type', 'password');

      await card(page).getByRole('button', { name: 'Show password' }).click();
      await expect(tokenField(page)).toHaveAttribute('type', 'text');
      await expect(tokenField(page)).toHaveValue(KEY);
    });

    test('should keep Enroll disabled and explain why until a token is entered', async ({ page }) => {
      await openEnrollment(page, NOT_ENROLLED);

      await expect(enrollButton(page)).toBeDisabled();
      await expect(enrollButton(page)).toHaveAttribute('title', 'Enrollment token is required');

      await tokenField(page).fill('   ');
      await expect(enrollButton(page)).toBeDisabled();

      await tokenField(page).fill(KEY);
      await expect(enrollButton(page)).toBeEnabled();
      await expect(enrollButton(page)).not.toHaveAttribute('title');
    });
  });

  test.describe('Validation', () => {
    test('should require an agent name', async ({ page }) => {
      const tracker = await openTracked(page, NOT_ENROLLED);

      await tokenField(page).fill(KEY);
      await agentField(page).fill('');
      await tenantField(page).fill('e2e-tenant');
      await card(page).getByRole('checkbox', { name: /I understand this will rotate/ }).check();
      await enrollButton(page).click();

      await expect(card(page).getByText('Agent name is required', { exact: true })).toBeVisible();
      expect(tracker.enrollments).toEqual([]);
    });

    test('should list every missing required field at once', async ({ page }) => {
      const tracker = await openTracked(page, NOT_ENROLLED);

      await tokenField(page).fill(KEY);
      await agentField(page).fill('');
      await enrollButton(page).click();

      await expect(card(page).getByText('Agent name is required', { exact: true })).toBeVisible();
      await expect(card(page).getByText('You must acknowledge the console data-sharing notice', { exact: true })).toBeVisible();
      await expect(card(page).getByText('Tenant / organization is required')).toHaveCount(0);
      expect(tracker.enrollments).toEqual([]);
    });

    test('should enroll without a tenant and send the agent name as the tenant', async ({ page }) => {
      const tracker = await openTracked(page, NOT_ENROLLED, { enrollResponse: PENDING });

      await tokenField(page).fill(KEY);
      await agentField(page).fill('e2e-agent');
      await card(page).getByRole('checkbox', { name: /I understand this will rotate/ }).check();
      await enrollButton(page).click();

      await expect.poll(() => tracker.enrollments).toEqual([
        { enrollment_key: KEY, tenant: 'e2e-agent', agent_name: 'e2e-agent', force: false },
      ]);
      await expect(card(page).getByText('Tenant / organization is required')).toHaveCount(0);
    });

    test('should send the enrolled tenant when rotating with the tenant field cleared', async ({ page }) => {
      const tracker = await openTracked(page, ENROLLED, { enrollResponse: PENDING });

      await tokenField(page).fill(KEY);
      await tenantField(page).fill('');
      await rotateButton(page).click();

      await expect.poll(() => tracker.enrollments).toEqual([
        { enrollment_key: KEY, tenant: 'e2e-tenant', agent_name: 'e2e-agent', force: true },
      ]);
    });
  });

  test.describe('Enrolling', () => {
    test('should send the trimmed values, mark the request as non-forced and show the pending state', async ({ page }) => {
      const tracker = await openTracked(page, NOT_ENROLLED, { enrollResponse: PENDING });

      await tokenField(page).fill(`  ${KEY}  `);
      await agentField(page).fill('  e2e-agent  ');
      await tenantField(page).fill('  e2e-tenant  ');
      await card(page).getByRole('checkbox', { name: /I understand this will rotate/ }).check();
      await enrollButton(page).click();

      await expect(
        getToastLocator(page, 'Enrollment request sent! Accept the enrollment on app.crowdsec.net to complete registration.', {
          type: 'success',
        }),
      ).toBeVisible();
      expect(tracker.enrollments).toEqual([
        { enrollment_key: KEY, tenant: 'e2e-tenant', agent_name: 'e2e-agent', force: false },
      ]);

      await test.step('The status is refreshed and the token is cleared', async () => {
        await expect(page.getByTestId('console-status-label')).toHaveText('Status: pending acceptance');
        await expect(page.getByTestId('pending-acceptance-info')).toContainText('Action Required');
        await expect(tokenField(page)).toHaveValue('');
      });
    });

    test('should default the agent name to the host name of the page', async ({ page }) => {
      const tracker = await openTracked(page, NOT_ENROLLED, { enrollResponse: PENDING });

      await expect(agentField(page)).toHaveValue(hostname(page));
      await tokenField(page).fill(KEY);
      await tenantField(page).fill('e2e-tenant');
      await card(page).getByRole('checkbox', { name: /I understand this will rotate/ }).check();
      await enrollButton(page).click();

      await expect.poll(() => tracker.enrollments).toEqual([
        { enrollment_key: KEY, tenant: 'e2e-tenant', agent_name: hostname(page), force: false },
      ]);
    });

    test('should show the server error inline and as a toast, with secrets masked', async ({ page }) => {
      const tracker = await openTracked(page, NOT_ENROLLED, {
        enrollError: { status: 400, error: 'bad key abcdefghij1234567890 rejected' },
      });

      await tokenField(page).fill(KEY);
      await tenantField(page).fill('e2e-tenant');
      await card(page).getByRole('checkbox', { name: /I understand this will rotate/ }).check();
      await enrollButton(page).click();

      await expect(card(page).getByText('bad key *** rejected', { exact: true })).toBeVisible();
      await expect(getToastLocator(page, 'bad key *** rejected', { type: 'error' })).toBeVisible();
      await expect(getToastLocator(page, /Enrollment request sent/, { type: 'success' })).toHaveCount(0);
      await expect(page.getByTestId('console-status-label')).toHaveText('Status: not enrolled');
      expect(tracker.enrollments).toHaveLength(1);
    });
  });

  test.describe('Rotating the Key', () => {
    test('should require a token before rotating', async ({ page }) => {
      const tracker = await openTracked(page, ENROLLED);

      await rotateButton(page).click();

      await expect(card(page).getByText('Enrollment token is required', { exact: true })).toBeVisible();
      expect(tracker.enrollments).toEqual([]);
    });

    test('should send a forced request without asking for the acknowledgement', async ({ page }) => {
      const tracker = await openTracked(page, ENROLLED, { enrollResponse: PENDING });

      await tokenField(page).fill(KEY);
      await agentField(page).fill('e2e-agent');
      await rotateButton(page).click();

      await expect(
        getToastLocator(page, 'Enrollment submitted! Accept the request on app.crowdsec.net to complete.', {
          type: 'success',
        }),
      ).toBeVisible();
      await expect(card(page).getByText('You must acknowledge the console data-sharing notice')).toHaveCount(0);
      expect(tracker.enrollments).toEqual([
        { enrollment_key: KEY, tenant: 'e2e-tenant', agent_name: 'e2e-agent', force: true },
      ]);
      await expect(page.getByTestId('console-status-label')).toHaveText('Status: pending acceptance');
    });

    test('should keep the enrolled agent name when rotating without editing it', async ({ page }) => {
      const tracker = await openTracked(page, ENROLLED, { enrollResponse: PENDING });

      await expect(agentField(page)).toHaveValue('e2e-agent');
      await tokenField(page).fill(KEY);
      await rotateButton(page).click();

      await expect.poll(() => tracker.enrollments).toEqual([
        { enrollment_key: KEY, tenant: 'e2e-tenant', agent_name: 'e2e-agent', force: true },
      ]);
    });
  });

  test.describe('Retrying a Failed Enrollment', () => {
    test('should require a token before retrying', async ({ page }) => {
      const tracker = await openTracked(page, FAILED);

      await retryButton(page).click();

      await expect(card(page).getByText('Enrollment token is required', { exact: true })).toBeVisible();
      expect(tracker.enrollments).toEqual([]);
    });

    test('should send a forced request and move on to the pending state', async ({ page }) => {
      const tracker = await openTracked(page, FAILED, { enrollResponse: PENDING });

      await tokenField(page).fill(KEY);
      await agentField(page).fill('e2e-agent');
      await retryButton(page).click();

      await expect(page.getByTestId('console-status-label')).toHaveText('Status: pending acceptance');
      expect(tracker.enrollments).toEqual([
        { enrollment_key: KEY, tenant: 'e2e-tenant', agent_name: 'e2e-agent', force: true },
      ]);
      await expect(retryButton(page)).toHaveCount(0);
    });
  });

  test.describe('Re-enrollment', () => {
    const section = (page: Page) => page.getByTestId('reenroll-section');

    test('should reveal and hide the re-enrollment form', async ({ page }) => {
      await openEnrollment(page, ENROLLED);

      await expect(section(page).getByRole('heading', { name: 'Re-enrollment Options' })).toBeVisible();
      await expect(section(page).getByLabel('New Enrollment Key')).toHaveCount(0);

      await section(page).getByRole('button', { name: 'Re-enroll with new key' }).click();
      await expect(section(page).getByLabel('New Enrollment Key')).toBeVisible();
      await expect(section(page).getByRole('button', { name: 'Re-enroll', exact: true })).toBeDisabled();

      await section(page).getByRole('button', { name: 'Cancel' }).click();
      await expect(section(page).getByLabel('New Enrollment Key')).toHaveCount(0);
    });

    test('should send a forced request with the new key and close the form', async ({ page }) => {
      const tracker = await openTracked(page, ENROLLED, { enrollResponse: PENDING });

      await section(page).getByRole('button', { name: 'Re-enroll with new key' }).click();
      await section(page).getByLabel('New Enrollment Key').fill('new-enrollment-key');
      await section(page).getByLabel('Agent Name').fill('e2e-agent');
      await section(page).getByRole('button', { name: 'Re-enroll', exact: true }).click();

      await expect(
        getToastLocator(page, 'Enrollment submitted! Accept the request on app.crowdsec.net to complete.', {
          type: 'success',
        }),
      ).toBeVisible();
      expect(tracker.enrollments).toEqual([
        { enrollment_key: 'new-enrollment-key', tenant: 'e2e-tenant', agent_name: 'e2e-agent', force: true },
      ]);
      await expect(section(page).getByLabel('New Enrollment Key')).toHaveCount(0);
    });

    test('should link to the CrowdSec console for a new key', async ({ page }) => {
      await openEnrollment(page, ENROLLED);

      await expect(section(page).getByRole('link', { name: 'Get new enrollment key from Console' })).toHaveAttribute(
        'href',
        'https://app.crowdsec.net/security-engines',
      );
    });
  });

  test.describe('Clearing the Enrollment State', () => {
    const CONFIRMATION = 'This will clear local enrollment state. You may need to re-enroll. Continue?';

    test('should leave the state untouched when the confirmation is declined', async ({ page }) => {
      const tracker = await openTracked(page, ENROLLED);

      page.once('dialog', (dialog) => {
        expect(dialog.type()).toBe('confirm');
        expect(dialog.message()).toBe(CONFIRMATION);
        void dialog.dismiss();
      });
      await page.getByRole('button', { name: 'Clear enrollment state' }).click();

      await expect(page.getByTestId('console-status-label')).toHaveText('Status: enrolled');
      expect(tracker.clears).toBe(0);
    });

    test('should clear the state once confirmed and return to the not enrolled view', async ({ page }) => {
      const tracker = await openTracked(page, ENROLLED);

      page.once('dialog', (dialog) => {
        expect(dialog.message()).toBe(CONFIRMATION);
        void dialog.accept();
      });
      await page.getByRole('button', { name: 'Clear enrollment state' }).click();

      await expect(page.getByTestId('console-status-label')).toHaveText('Status: not enrolled');
      await expect(page.getByTestId('reenroll-section')).toHaveCount(0);
      await expect(page.getByTestId('console-token-state')).toHaveText('Not stored');
      expect(tracker.clears).toBe(1);
    });

    test('should show an error toast and keep the state when clearing fails', async ({ page }) => {
      const tracker = await openTracked(page, ENROLLED);
      await page.route(CROWDSEC_ROUTES.consoleEnrollment, (route) =>
        route.request().method() === 'DELETE'
          ? route.fulfill({ status: 500, contentType: 'application/json', json: { error: 'database is locked' } })
          : route.fallback(),
      );

      page.once('dialog', (dialog) => void dialog.accept());
      await page.getByRole('button', { name: 'Clear enrollment state' }).click();

      await expect(
        getToastLocator(page, 'Failed to clear enrollment state: database is locked', { type: 'error' }),
      ).toBeVisible();
      await expect(page.getByTestId('console-status-label')).toHaveText('Status: enrolled');
      expect(tracker.clears).toBe(0);
    });
  });

  test.describe('Local API Readiness', () => {
    const NOT_READY = crowdsecFixtures.runningStatus({ lapi_ready: false });

    test('should block enrolling and explain why while the local API is not ready', async ({ page }) => {
      await openEnrollment(page, NOT_ENROLLED, { status: NOT_READY });
      await tokenField(page).fill(KEY);

      await expect(enrollButton(page)).toBeDisabled({ timeout: GATING_TIMEOUT });
      await expect(enrollButton(page)).toHaveAttribute('title', 'LAPI must be ready to enroll');
    });

    test('should block rotating the key while the local API is not ready', async ({ page }) => {
      await openEnrollment(page, ENROLLED, { status: NOT_READY });

      await expect(rotateButton(page)).toBeDisabled({ timeout: GATING_TIMEOUT });
      await expect(rotateButton(page)).toHaveAttribute('title', 'LAPI must be ready to rotate key');
    });

    test('should block retrying while the local API is not ready', async ({ page }) => {
      await openEnrollment(page, FAILED, { status: NOT_READY });

      await expect(retryButton(page)).toBeDisabled({ timeout: GATING_TIMEOUT });
      await expect(retryButton(page)).toHaveAttribute('title', 'LAPI must be ready to retry enrollment');
    });
  });
});
