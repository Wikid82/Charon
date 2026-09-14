/**
 * Web Push Notification Provider E2E Tests
 *
 * Phase 1 (docs/plans/current_spec.md §5, §9 commit 1): encodes the intended
 * Web Push provider behavior against the API contracts (§3.4) and frontend
 * design (§3.6) before any backend/frontend code for this feature exists.
 * All tests below are `test.fixme` and are flipped to live tests in commit 8
 * (§9), once commits 2-7 (backend model/dispatch/handlers, frontend service
 * worker/API client/UI) have landed.
 *
 * Scenarios covered:
 *  - Provisioning a Web Push provider from the Notifications page, and that
 *    provisioning is admin-only (§3.4.0, §3.4.1).
 *  - Subscribing this device (§3.4.2 VAPID key, §3.4.3 subscribe), mocking
 *    `Notification`/`navigator.serviceWorker`/`PushManager` via
 *    `page.addInitScript` — real push delivery cannot be exercised in CI
 *    (spec §5 Phase 1, §6 Acceptance Criteria #3).
 *  - Unsubscribing removes the device from the subscriptions list (§3.4.5).
 *  - Per-event-type `NotifyXxx` toggles persist for a `webpush` provider row
 *    exactly like every other provider type (regression coverage for the
 *    generic preference UI, §3.6.3's closing note).
 *
 * See docs/plans/current_spec.md §3.4 (API contracts), §3.4.0 (authorization
 * model), §3.6 (frontend design), §9 commit 1.
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';
import type { Page } from '@playwright/test';

const WEBPUSH_BASE = '/api/v1/notifications/providers/webpush';
const PROVIDERS_ENDPOINT = '/api/v1/notifications/providers';
const MOCK_ENDPOINT = 'https://fcm.googleapis.com/fcm/send/mock-endpoint-e2e';
const MOCK_VAPID_PUBLIC_KEY = 'BN_mock_vapid_public_key_0123456789';

/** §3.4.1 response shape for a provisioned `webpush` provider row. */
interface WebPushProviderFixture {
  id: string;
  name: string;
  type: 'webpush';
  enabled: boolean;
  service_config: string;
  notify_proxy_hosts: boolean;
  notify_remote_servers: boolean;
  notify_domains: boolean;
  notify_certs: boolean;
  notify_uptime: boolean;
}

function buildWebPushProviderFixture(
  overrides: Partial<WebPushProviderFixture> = {}
): WebPushProviderFixture {
  return {
    id: 'webpush-provider-1',
    name: 'Browser Push',
    type: 'webpush',
    enabled: true,
    service_config: JSON.stringify({
      vapid_public_key: MOCK_VAPID_PUBLIC_KEY,
      vapid_subject: 'mailto:admin@example.com',
    }),
    notify_proxy_hosts: true,
    notify_remote_servers: false,
    notify_domains: false,
    notify_certs: true,
    notify_uptime: false,
    ...overrides,
  };
}

/** §3.4.4 response shape for one subscribed device. */
interface WebPushSubscriptionFixture {
  id: string;
  endpoint: string;
  user_agent: string;
  created_at: string;
  last_seen_at: string;
}

function buildSubscriptionFixture(
  overrides: Partial<WebPushSubscriptionFixture> = {}
): WebPushSubscriptionFixture {
  return {
    id: 'webpush-sub-1',
    endpoint: MOCK_ENDPOINT,
    user_agent: 'Mozilla/5.0 (E2E Test Runner)',
    created_at: '2026-09-01T00:00:00Z',
    last_seen_at: '2026-09-01T00:00:00Z',
    ...overrides,
  };
}

/**
 * Stubs the browser-side Push API (`Notification`, `navigator.serviceWorker`,
 * `PushManager`) so `Notification.requestPermission()` and
 * `registration.pushManager.subscribe()` resolve deterministically, without a
 * real OS permission prompt or a real round trip to a push service. Real push
 * delivery cannot be exercised in CI (spec §5 Phase 1) — this stub is what
 * lets the subscribe/unsubscribe flow be driven end-to-end anyway.
 */
async function stubBrowserPushApis(page: Page): Promise<void> {
  await page.addInitScript((mockEndpoint: string) => {
    const mockSubscription = {
      endpoint: mockEndpoint,
      toJSON() {
        return {
          endpoint: mockEndpoint,
          keys: { p256dh: 'mock-p256dh-key', auth: 'mock-auth-secret' },
        };
      },
      unsubscribe: async () => true,
    };

    const mockRegistration = {
      pushManager: {
        subscribe: async () => mockSubscription,
        getSubscription: async () => null,
      },
    };

    class MockNotification {
      static permission = 'default';
      static requestPermission = async () => {
        MockNotification.permission = 'granted';
        return 'granted';
      };
    }

    Object.defineProperty(window, 'Notification', {
      configurable: true,
      value: MockNotification,
    });

    Object.defineProperty(window.navigator, 'serviceWorker', {
      configurable: true,
      value: {
        register: async () => mockRegistration,
        ready: Promise.resolve(mockRegistration),
      },
    });
  }, MOCK_ENDPOINT);
}

/** Locator matching the §3.6.3 "Enable push notifications on this device"
 * control, tolerant of either a checkbox (matching every other toggle in
 * this form) or a switch-styled control, since the exact implementation
 * hasn't landed yet. */
function deviceToggleLocator(page: Page) {
  return page
    .getByRole('checkbox', { name: /enable push notifications on this device/i })
    .or(page.getByRole('switch', { name: /enable push notifications on this device/i }));
}

test.describe('Web Push Notification Provider', () => {
  test.describe('Provisioning (§3.4.1, admin-only per §3.4.0)', () => {
    test.fixme(
      'admin can provision a Web Push provider from the Notifications page',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);

        let capturedPayload: Record<string, unknown> | null = null;
        let providers: WebPushProviderFixture[] = [];
        const provisioned = buildWebPushProviderFixture();

        await test.step('Mock the provision endpoint and the provider list', async () => {
          await page.route(`**${WEBPUSH_BASE}/provision`, async (route) => {
            if (route.request().method() === 'POST') {
              capturedPayload = route.request().postDataJSON();
              providers = [provisioned];
              await route.fulfill({ status: 201, json: provisioned });
            } else {
              await route.continue();
            }
          });

          await page.route(`**${PROVIDERS_ENDPOINT}`, async (route) => {
            if (route.request().method() === 'GET') {
              await route.fulfill({ status: 200, json: providers });
            } else {
              await route.continue();
            }
          });
        });

        await page.goto('/settings/notifications');
        await waitForLoadingComplete(page);

        await test.step('Open Add Provider form and select Web Push', async () => {
          await page.getByRole('button', { name: /add.*provider/i }).click();
          await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
          await page.getByTestId('provider-type').selectOption('webpush');
        });

        await test.step('Verify the dedicated provisioning flow replaces the generic URL/token fields (§3.6.3)', async () => {
          await expect(page.getByTestId('provider-url')).toHaveCount(0);
          await expect(page.getByTestId('provider-gotify-token')).toHaveCount(0);

          const provisionButton = page.getByRole('button', { name: /provision web push/i });
          await expect(provisionButton).toBeVisible();
          await expect(provisionButton).toMatchAriaSnapshot(`
            - button "Provision Web Push"
          `);
        });

        await test.step('Fill provisioning details and provision', async () => {
          await page.getByTestId('provider-name').fill('Browser Push');
          await page.getByLabel(/vapid subject/i).fill('mailto:admin@example.com');

          await Promise.all([
            page.waitForResponse(
              (resp) => resp.url().includes(`${WEBPUSH_BASE}/provision`) && resp.status() === 201
            ),
            page.getByRole('button', { name: /provision web push/i }).click(),
          ]);
        });

        await test.step('Verify the outgoing payload matches the §3.4.1 contract', () => {
          expect(capturedPayload).toBeTruthy();
          expect(capturedPayload?.name).toBe('Browser Push');
          expect(capturedPayload?.vapid_subject).toBe('mailto:admin@example.com');
        });

        await test.step('Verify the provisioned provider appears in the list', async () => {
          const row = page.getByTestId(`provider-row-${provisioned.id}`);
          await expect(row).toBeVisible({ timeout: 10000 });
          await expect(row).toContainText('Browser Push');
        });
      }
    );

    test.fixme(
      'a non-admin user is forbidden from provisioning a Web Push provider',
      async ({ page, regularUser }) => {
        await loginUser(page, regularUser);

        await test.step('Call the provision endpoint directly as a RoleUser (management access, not admin)', async () => {
          const response = await page.request.post(`${WEBPUSH_BASE}/provision`, {
            headers: { Authorization: `Bearer ${regularUser.token}` },
            data: { name: 'Browser Push', vapid_subject: 'mailto:admin@example.com' },
          });

          // §3.4 row 1: provisioning additionally requires RequireRole(admin),
          // mirroring Test/Preview's existing admin-only pattern.
          expect(response.status()).toBe(403);
        });
      }
    );
  });

  test.describe('Device subscription (§3.4.2 VAPID key, §3.4.3 subscribe)', () => {
    test.fixme(
      'an authenticated user can subscribe this device to Web Push notifications',
      async ({ page, regularUser }) => {
        await stubBrowserPushApis(page);
        await loginUser(page, regularUser);

        let subscriptions: WebPushSubscriptionFixture[] = [];
        let capturedSubscribePayload: Record<string, unknown> | null = null;

        await test.step('Mock the provisioned provider, VAPID key, and subscriptions endpoints', async () => {
          await page.route(`**${PROVIDERS_ENDPOINT}`, async (route) => {
            if (route.request().method() === 'GET') {
              await route.fulfill({ status: 200, json: [buildWebPushProviderFixture()] });
            } else {
              await route.continue();
            }
          });

          await page.route(`**${WEBPUSH_BASE}/vapid-public-key`, async (route) => {
            await route.fulfill({ status: 200, json: { vapid_public_key: MOCK_VAPID_PUBLIC_KEY } });
          });

          await page.route(`**${WEBPUSH_BASE}/subscriptions`, async (route) => {
            if (route.request().method() === 'POST') {
              capturedSubscribePayload = route.request().postDataJSON();
              const created = buildSubscriptionFixture();
              subscriptions = [created];
              await route.fulfill({
                status: 201,
                json: { id: created.id, endpoint: created.endpoint },
              });
            } else if (route.request().method() === 'GET') {
              await route.fulfill({ status: 200, json: subscriptions });
            } else {
              await route.continue();
            }
          });
        });

        await page.goto('/settings/notifications');
        await waitForLoadingComplete(page);

        await test.step('Enable push notifications on this device', async () => {
          await Promise.all([
            page.waitForResponse(
              (resp) =>
                resp.url().includes(`${WEBPUSH_BASE}/subscriptions`) &&
                resp.request().method() === 'POST' &&
                resp.status() === 201
            ),
            deviceToggleLocator(page).click(),
          ]);
        });

        await test.step('Verify the browser PushSubscription is forwarded verbatim (§3.4.3)', () => {
          expect(capturedSubscribePayload).toBeTruthy();
          expect(capturedSubscribePayload?.endpoint).toBe(MOCK_ENDPOINT);
          const keys = capturedSubscribePayload?.keys as Record<string, unknown>;
          expect(keys?.p256dh).toBe('mock-p256dh-key');
          expect(keys?.auth).toBe('mock-auth-secret');
        });

        await test.step("Verify the device now appears in this user's subscribed devices list", async () => {
          await expect(page.getByText(MOCK_ENDPOINT).or(page.getByText(/this device/i)).first()).toBeVisible({
            timeout: 10000,
          });
        });
      }
    );
  });

  test.describe('Device unsubscription (§3.4.5)', () => {
    test.fixme(
      'unsubscribing removes the device from the subscriptions list',
      async ({ page, regularUser }) => {
        await stubBrowserPushApis(page);
        await loginUser(page, regularUser);

        let subscriptions: WebPushSubscriptionFixture[] = [buildSubscriptionFixture()];
        let deleteCalled = false;

        await test.step("Mock the provisioned provider and this device's existing subscription", async () => {
          await page.route(`**${PROVIDERS_ENDPOINT}`, async (route) => {
            if (route.request().method() === 'GET') {
              await route.fulfill({ status: 200, json: [buildWebPushProviderFixture()] });
            } else {
              await route.continue();
            }
          });

          await page.route(`**${WEBPUSH_BASE}/subscriptions`, async (route) => {
            if (route.request().method() === 'GET') {
              await route.fulfill({ status: 200, json: subscriptions });
            } else {
              await route.continue();
            }
          });

          await page.route(`**${WEBPUSH_BASE}/subscriptions/*`, async (route) => {
            if (route.request().method() === 'DELETE') {
              deleteCalled = true;
              subscriptions = [];
              await route.fulfill({ status: 204 });
            } else {
              await route.continue();
            }
          });
        });

        await page.goto('/settings/notifications');
        await waitForLoadingComplete(page);

        await test.step('Verify the subscribed device is listed', async () => {
          await expect(page.getByText(MOCK_ENDPOINT).or(page.getByText(/this device/i)).first()).toBeVisible({
            timeout: 10000,
          });
        });

        await test.step('Verify the enabled toggle reflects the existing subscription', async () => {
          await expect(deviceToggleLocator(page)).toMatchAriaSnapshot(`
            - checkbox "Enable push notifications on this device" [checked]
          `);
        });

        await test.step('Unsubscribe this device', async () => {
          await Promise.all([
            page.waitForResponse(
              (resp) =>
                resp.url().includes(`${WEBPUSH_BASE}/subscriptions/`) &&
                resp.request().method() === 'DELETE' &&
                resp.status() === 204
            ),
            deviceToggleLocator(page).click(),
          ]);
        });

        await test.step('Verify the device is removed from the list and the backend delete fired (§3.4.5)', async () => {
          expect(deleteCalled).toBe(true);
          await expect(page.getByText(MOCK_ENDPOINT)).toHaveCount(0);
        });
      }
    );
  });

  test.describe('Per-event-type toggle regression (§3.4.0 closing note, §3.6.3)', () => {
    test.fixme(
      'per-event-type notification toggles persist for a webpush provider row identically to other provider types',
      async ({ page, adminUser }) => {
        await loginUser(page, adminUser);

        let updatedPayload: Record<string, unknown> | null = null;
        let providers: WebPushProviderFixture[] = [
          buildWebPushProviderFixture({
            notify_proxy_hosts: true,
            notify_remote_servers: false,
            notify_domains: false,
            notify_certs: true,
            notify_uptime: false,
          }),
        ];

        await test.step('Mock the existing webpush provider row', async () => {
          await page.route(`**${PROVIDERS_ENDPOINT}`, async (route) => {
            if (route.request().method() === 'GET') {
              await route.fulfill({ status: 200, json: providers });
            } else {
              await route.continue();
            }
          });

          await page.route(`**${PROVIDERS_ENDPOINT}/*`, async (route) => {
            if (route.request().method() === 'PUT') {
              updatedPayload = route.request().postDataJSON();
              providers = providers.map((p) =>
                p.id === 'webpush-provider-1' ? { ...p, ...updatedPayload } : p
              );
              await route.fulfill({ status: 200, json: { success: true } });
            } else {
              await route.continue();
            }
          });
        });

        await page.goto('/settings/notifications');
        await waitForLoadingComplete(page);

        await test.step('Open the webpush provider row for editing', async () => {
          const providerRow = page.getByTestId('provider-row-webpush-provider-1');
          await expect(providerRow).toBeVisible({ timeout: 10000 });
          await providerRow.getByRole('button', { name: /edit/i }).click();
          await expect(page.getByTestId('provider-name')).toBeVisible({ timeout: 5000 });
        });

        await test.step('Verify the loaded toggle state matches the mocked webpush row', async () => {
          await expect(page.getByTestId('notify-proxy-hosts')).toBeChecked();
          await expect(page.getByTestId('notify-remote-servers')).not.toBeChecked();
          await expect(page.getByTestId('notify-domains')).not.toBeChecked();
          await expect(page.getByTestId('notify-certs')).toBeChecked();
          await expect(page.getByTestId('notify-uptime')).not.toBeChecked();
        });

        await test.step('Flip every event-type toggle to its opposite state', async () => {
          await page.getByTestId('notify-proxy-hosts').uncheck();
          await page.getByTestId('notify-remote-servers').check();
          await page.getByTestId('notify-domains').check();
          await page.getByTestId('notify-certs').uncheck();
          await page.getByTestId('notify-uptime').check();
        });

        await test.step('Save and verify the PUT payload persists every toggle for the webpush type', async () => {
          await Promise.all([
            page.waitForResponse(
              (resp) =>
                resp.url().includes(`${PROVIDERS_ENDPOINT}/webpush-provider-1`) &&
                resp.request().method() === 'PUT' &&
                resp.status() === 200
            ),
            page.getByTestId('provider-save-btn').click(),
          ]);

          expect(updatedPayload?.notify_proxy_hosts).toBe(false);
          expect(updatedPayload?.notify_remote_servers).toBe(true);
          expect(updatedPayload?.notify_domains).toBe(true);
          expect(updatedPayload?.notify_certs).toBe(false);
          expect(updatedPayload?.notify_uptime).toBe(true);
        });

        await test.step('Reload and verify the flipped toggles survive a refetch, exactly like every other provider type', async () => {
          await page.reload();
          await waitForLoadingComplete(page);

          const providerRow = page.getByTestId('provider-row-webpush-provider-1');
          await providerRow.getByRole('button', { name: /edit/i }).click();
          await expect(page.getByTestId('notify-proxy-hosts')).not.toBeChecked();
          await expect(page.getByTestId('notify-remote-servers')).toBeChecked();
          await expect(page.getByTestId('notify-domains')).toBeChecked();
          await expect(page.getByTestId('notify-certs')).not.toBeChecked();
          await expect(page.getByTestId('notify-uptime')).toBeChecked();
        });
      }
    );
  });
});
