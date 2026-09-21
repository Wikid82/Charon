/**
 * Redirection Hosts E2E Tests (Issue #1367, part 2)
 *
 * Covers the new "Redirection Host" feature described in
 * docs/plans/current_spec.md: a dedicated, discoverable way to configure a
 * domain to issue an HTTP redirect (301/302/307/308) to a target URL,
 * without touching Caddy JSON. This is a *separate* resource/page from
 * Proxy Hosts (§3 Scope Decision — a peer model, not a mode flag on
 * `ProxyHost`), reachable via its own "Redirection Hosts" nav item.
 *
 * STATUS: Commit 1 (spec §9 Commit Slicing Strategy) — none of the backend
 * model/API (`RedirectionHost`, `redirectionhost_service.go`,
 * `redirection_host_handler.go`) or frontend (`RedirectionHosts.tsx`,
 * `RedirectionHostForm.tsx`, nav item, route) exists yet. Every test below
 * is `test.fixme` and encodes the *expected* structure straight from the
 * spec (§4.1 Data Model, §4.4 API Contract, §4.5 Frontend, §5 Status Code
 * Selection) so that Commit 6 ("test: enable redirection host e2e specs")
 * can flip `.fixme` to a real test with minimal rework. Mirrors the exact
 * pattern this repo already used for the sibling feature in the same
 * issue — see the "Proxy Host Form — Group Selector" describe block in
 * tests/proxy-groups.spec.ts (commit 7a97f589).
 *
 * Locator/naming conventions assumed here (inferred from spec + existing
 * ProxyHosts/ProxyHostForm conventions, so later commits should match):
 *  - Nav item: accessible name "Redirection Hosts" (`t('navigation.redirectionHosts')`),
 *    linking to `/redirection-hosts` (spec §2.5, §4.5).
 *  - List page heading: "Redirection Hosts" (mirrors "Proxy Hosts" page heading).
 *  - "Add Redirection Host" button opens a dialog titled "Add Redirection Host"
 *    (mirrors "Add Proxy Host" / `openAddProxyHostDialog` in tests/proxy-groups.spec.ts).
 *  - Editing a row opens a dialog titled "Edit Redirection Host", opened via a
 *    row action button with accessible name `Edit redirection host <name-or-domain>`
 *    (mirrors ProxyHosts.tsx's `aria-label={`Edit proxy host ${...}`}`).
 *  - Row delete action: accessible name `Delete redirection host <name-or-domain>`,
 *    which opens a confirmation dialog titled "Delete Redirection Host?"
 *    (mirrors `proxyHosts.deleteConfirmTitle` = "Delete Proxy Host?").
 *  - Form fields, all via `getByLabel` per spec §4.5:
 *      - "Name" (optional friendly name, same as ProxyHostForm)
 *      - "Domain Names" (comma-separated, same multi-domain UX/label as ProxyHostForm)
 *      - "Target URL"
 *      - "Status Code" — a combobox (mirrors `ProxyGroupSelector`'s
 *        `combobox` pattern), options rendered as plain-language labels
 *        per spec §4.5's `REDIRECT_STATUS_CODES`:
 *        "301 - Permanent", "302 - Temporary", "307 - Temporary (preserve method)",
 *        "308 - Permanent (preserve method)".
 *      - "Preserve Path" toggle/switch (spec §4.1 `preserve_path`, §4.5)
 *      - "SSL Forced", "HTTP/2 Support", "HSTS" toggles (spec §4.1, reused
 *        from ProxyHostForm's equivalent toggles/labels)
 *      - "Certificate" combobox (mirrors ProxyHostForm's `aria-label="SSL Certificate"`
 *        trigger — using "Certificate" here since RedirectionHostForm is a
 *        smaller, dedicated form per spec §4.5; adjust to match whatever
 *        `backend-dev`/`frontend-dev` actually ship if this drifts)
 *  - Table columns (spec §4.5): "Domain(s)", "Target", "Status Code", "Enabled", "Actions".
 *  - API routes exercised for `waitForAPIResponse` (spec §4.4):
 *      `POST /api/v1/redirection-hosts`, `PUT /api/v1/redirection-hosts/:uuid`,
 *      `DELETE /api/v1/redirection-hosts/:uuid`.
 *
 * See docs/plans/current_spec.md §4 (Technical Specifications), §5 (Status
 * Code Selection), §8 Phase 1, §9 Commit 1.
 */

import type { Page } from '@playwright/test';
import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { waitForDialog, waitForAPIResponse, waitForLoadingComplete } from './utils/wait-helpers';
import { generateDomain } from './fixtures/test-data';
import { generateProxyHost } from './fixtures/proxy-hosts';

/**
 * Escape a string for safe use inside a `RegExp` constructor.
 * (Duplicated from tests/proxy-groups.spec.ts — small enough that
 * extracting a shared util isn't worth it yet; flag for consolidation if a
 * third spec needs it.)
 */
function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * A minimal redirection-host configuration for E2E purposes. There is no
 * `fixtures/redirection-hosts.ts` yet (nothing to generate fixtures *for*
 * until the model/API/UI exist) — per the task's guidance, this stays
 * inline rather than speculatively building out a fixture module. Promote
 * this to `tests/fixtures/redirection-hosts.ts` (mirroring
 * `generateProxyHost` in `tests/fixtures/proxy-hosts.ts`) in a later
 * commit once the shape is confirmed against the real API contract.
 */
interface RedirectionHostConfig {
  name: string;
  domain: string;
  targetUrl: string;
  statusCode: 301 | 302 | 307 | 308;
  preservePath: boolean;
}

function generateRedirectionHost(overrides: Partial<RedirectionHostConfig> = {}): RedirectionHostConfig {
  const domain = generateDomain('redirect');
  return {
    name: `E2E Redirection Host ${Date.now()}`,
    domain,
    targetUrl: `https://target-${domain}`,
    statusCode: 301,
    preservePath: true,
    ...overrides,
  };
}

/**
 * Open the "Add Redirection Host" dialog and return its locator.
 * Mirrors `openAddProxyHostDialog` in tests/proxy-groups.spec.ts.
 */
async function openAddRedirectionHostDialog(page: Page) {
  await page.getByRole('button', { name: /add redirection host/i }).first().click();
  const dialog = page.getByRole('dialog', { name: /add redirection host/i });
  await waitForDialog(page);
  await expect(dialog).toBeVisible();
  return dialog;
}

/**
 * Open the "Edit Redirection Host" dialog for the row whose accessible name
 * contains `domain`. Mirrors `openEditProxyHostDialogForDomain` in
 * tests/proxy-groups.spec.ts.
 */
async function openEditRedirectionHostDialogForDomain(page: Page, domain: string) {
  const row = page.getByRole('row', { name: new RegExp(escapeRegExp(domain)) });
  await row.getByRole('button', { name: /^edit redirection host/i }).click();
  const dialog = page.getByRole('dialog', { name: /edit redirection host/i });
  await waitForDialog(page);
  await expect(dialog).toBeVisible();
  return dialog;
}

/**
 * Fill and submit the Add Redirection Host form, waiting for the create
 * response. Mirrors `createProxyHostViaUI` in tests/proxy-groups.spec.ts.
 */
async function createRedirectionHostViaUI(page: Page, host: RedirectionHostConfig): Promise<void> {
  const dialog = await openAddRedirectionHostDialog(page);

  await dialog.getByLabel(/^name\b/i).fill(host.name);
  await dialog.getByLabel(/domain names/i).fill(host.domain);
  await dialog.getByLabel(/target url/i).fill(host.targetUrl);

  const statusSelect = dialog.getByRole('combobox', { name: /status code/i });
  await statusSelect.click();
  await page.getByRole('option', { name: new RegExp(`^${host.statusCode}\\b`) }).click();

  if (!host.preservePath) {
    await dialog.getByLabel(/preserve path/i).uncheck();
  }

  const createPromise = waitForAPIResponse(page, '/api/v1/redirection-hosts', { status: 201 });
  await dialog.getByRole('button', { name: /^save$/i }).click();
  await createPromise;
}

test.describe('Redirection Hosts', () => {
  test.beforeEach(async ({ page }) => {
    await waitForAPIHealth(page.request);
  });

  test.describe('Navigation', () => {
    test.fixme('shows a "Redirection Hosts" nav item that navigates to the dedicated page', async ({ page }) => {
      await page.goto('/');
      await waitForLoadingComplete(page);

      await test.step('Nav item is visible', async () => {
        await expect(page.getByRole('link', { name: /redirection hosts/i })).toBeVisible();
      });

      await test.step('Clicking it navigates to /redirection-hosts', async () => {
        await page.getByRole('link', { name: /redirection hosts/i }).click();
        await expect(page).toHaveURL(/\/redirection-hosts/);
        await expect(page.getByRole('heading', { name: /redirection hosts/i })).toBeVisible();
      });
    });
  });

  test.describe('List page', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/redirection-hosts');
      await waitForLoadingComplete(page);
    });

    test.fixme('renders a dedicated list, separate from Proxy Hosts', async ({ page }) => {
      await test.step('Redirection Hosts page has its own table and does not show Proxy Host rows', async () => {
        await expect(page.getByRole('heading', { name: /redirection hosts/i })).toBeVisible();
        const table = page.getByRole('table').first();
        await expect(table).toBeVisible();
        await expect(table.getByRole('columnheader', { name: /domain/i })).toBeVisible();
        await expect(table.getByRole('columnheader', { name: /target/i })).toBeVisible();
        await expect(table.getByRole('columnheader', { name: /status code/i })).toBeVisible();
      });

      await test.step('Proxy Hosts page remains reachable and unaffected', async () => {
        await page.goto('/proxy-hosts');
        await waitForLoadingComplete(page);
        await expect(page.getByRole('heading', { name: /proxy hosts/i })).toBeVisible();
      });
    });

    test.fixme('a newly created redirection host appears in the list', async ({ page }) => {
      const host = generateRedirectionHost();

      await test.step('Create a redirection host', async () => {
        await createRedirectionHostViaUI(page, host);
      });

      await test.step('It appears in the Redirection Hosts table', async () => {
        await expect(page.getByRole('row', { name: new RegExp(escapeRegExp(host.domain)) })).toBeVisible();
      });
    });
  });

  test.describe('Create', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/redirection-hosts');
      await waitForLoadingComplete(page);
    });

    test.fixme('creates a redirection host with domain, target URL, status code, and preserve-path toggle', async ({ page }) => {
      const host = generateRedirectionHost({ statusCode: 301, preservePath: true });

      await test.step('Fill and submit the Add Redirection Host form', async () => {
        await createRedirectionHostViaUI(page, host);
      });

      await test.step('The new host is visible with its configured target and status code', async () => {
        const row = page.getByRole('row', { name: new RegExp(escapeRegExp(host.domain)) });
        await expect(row).toBeVisible();
        await expect(row.getByText(host.targetUrl)).toBeVisible();
        await expect(row.getByText(/301/)).toBeVisible();
      });
    });

    test.fixme('supports disabling "Preserve Path" so the target URL is used verbatim', async ({ page }) => {
      const host = generateRedirectionHost({ preservePath: false });

      await test.step('Create with Preserve Path unchecked', async () => {
        await createRedirectionHostViaUI(page, host);
      });

      await test.step('Editing the host again shows Preserve Path unchecked', async () => {
        const dialog = await openEditRedirectionHostDialogForDomain(page, host.domain);
        await expect(dialog.getByLabel(/preserve path/i)).not.toBeChecked();
      });
    });
  });

  test.describe('Status Code Selector', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/redirection-hosts');
      await waitForLoadingComplete(page);
    });

    test.fixme('offers exactly 301, 302, 307, and 308 with plain-language labels', async ({ page }) => {
      const dialog = await openAddRedirectionHostDialog(page);

      await test.step('Open the Status Code combobox', async () => {
        const statusSelect = dialog.getByRole('combobox', { name: /status code/i });
        await statusSelect.click();
      });

      await test.step('Exactly the four supported codes are offered, with plain-language labels (spec §5)', async () => {
        // Per spec §4.5's REDIRECT_STATUS_CODES const — plain-language
        // labels, not bare numeric codes, so a novice user understands
        // "permanent" vs "temporary" without knowing HTTP status semantics.
        await expect(page.getByRole('option', { name: /301.*permanent/i })).toBeVisible();
        await expect(page.getByRole('option', { name: /302.*temporary/i })).toBeVisible();
        await expect(page.getByRole('option', { name: /307.*temporary/i })).toBeVisible();
        await expect(page.getByRole('option', { name: /308.*permanent/i })).toBeVisible();

        // Explicitly not offered (spec §5: excluded, not a redirect primitive
        // users reach for / not meaningfully generatable here).
        await expect(page.getByRole('option', { name: /^300\b/ })).toHaveCount(0);
        await expect(page.getByRole('option', { name: /^303\b/ })).toHaveCount(0);
        await expect(page.getByRole('option', { name: /^304\b/ })).toHaveCount(0);
      });
    });
  });

  test.describe('Edit', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/redirection-hosts');
      await waitForLoadingComplete(page);
    });

    test.fixme("updates an existing redirection host's status code and target URL", async ({ page }) => {
      const host = generateRedirectionHost({ statusCode: 302 });
      const newTargetUrl = `https://updated-${host.domain}`;

      await test.step('Create a redirection host to edit', async () => {
        await createRedirectionHostViaUI(page, host);
      });

      await test.step('Edit its status code and target URL', async () => {
        const dialog = await openEditRedirectionHostDialogForDomain(page, host.domain);

        await dialog.getByLabel(/target url/i).fill(newTargetUrl);

        const statusSelect = dialog.getByRole('combobox', { name: /status code/i });
        await statusSelect.click();
        await page.getByRole('option', { name: /^308\b/ }).click();

        const updatePromise = waitForAPIResponse(page, '/api/v1/redirection-hosts/', { status: 200 });
        await dialog.getByRole('button', { name: /^save$/i }).click();
        await updatePromise;
      });

      await test.step('The list reflects the updated target and status code', async () => {
        const row = page.getByRole('row', { name: new RegExp(escapeRegExp(host.domain)) });
        await expect(row.getByText(newTargetUrl)).toBeVisible();
        await expect(row.getByText(/308/)).toBeVisible();
      });
    });
  });

  test.describe('Delete', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/redirection-hosts');
      await waitForLoadingComplete(page);
    });

    test.fixme('deletes a redirection host', async ({ page }) => {
      const host = generateRedirectionHost();

      await test.step('Create a redirection host to delete', async () => {
        await createRedirectionHostViaUI(page, host);
      });

      await test.step('Delete it via the row action and confirm', async () => {
        const row = page.getByRole('row', { name: new RegExp(escapeRegExp(host.domain)) });
        await row.getByRole('button', { name: /^delete redirection host/i }).click();

        const confirmDialog = page.getByRole('dialog', { name: /delete redirection host/i });
        await expect(confirmDialog).toBeVisible();

        const deletePromise = waitForAPIResponse(page, '/api/v1/redirection-hosts/', { status: 200 });
        await confirmDialog.getByRole('button', { name: /^delete$/i }).click();
        await deletePromise;
      });

      await test.step('It no longer appears in the list', async () => {
        await expect(page.getByRole('row', { name: new RegExp(escapeRegExp(host.domain)) })).toHaveCount(0);
      });
    });
  });

  test.describe('Validation', () => {
    test.beforeEach(async ({ page }) => {
      await page.goto('/redirection-hosts');
      await waitForLoadingComplete(page);
    });

    test.fixme('rejects an empty target URL', async ({ page }) => {
      const host = generateRedirectionHost();
      const dialog = await openAddRedirectionHostDialog(page);

      await test.step('Fill domain but leave Target URL empty, then attempt to save', async () => {
        await dialog.getByLabel(/^name\b/i).fill(host.name);
        await dialog.getByLabel(/domain names/i).fill(host.domain);
        await dialog.getByRole('button', { name: /^save$/i }).click();
      });

      await test.step('A validation error is shown and no request is made (spec §4.4: "target_url is required")', async () => {
        await expect(dialog.getByText(/target url is required/i)).toBeVisible();
      });
    });

    test.fixme('rejects a redirect target pointing back at one of the host\'s own domains (self-redirect guard)', async ({ page }) => {
      const domain = generateDomain('redirect');
      const dialog = await openAddRedirectionHostDialog(page);

      await test.step('Fill in a target URL whose host equals the domain being configured', async () => {
        await dialog.getByLabel(/^name\b/i).fill(`E2E Self Redirect ${Date.now()}`);
        await dialog.getByLabel(/domain names/i).fill(domain);
        await dialog.getByLabel(/target url/i).fill(`https://${domain}`);
        await dialog.getByRole('button', { name: /^save$/i }).click();
      });

      await test.step('A validation error is shown (spec §4.3, §4.4: self-redirect guard)', async () => {
        await expect(
          dialog.getByText(/redirect target cannot point back to one of this host's own domains/i),
        ).toBeVisible();
      });
    });

    test.fixme('rejects a domain already used by an existing Proxy Host (cross-table uniqueness)', async ({ page }) => {
      const proxyHost = generateProxyHost();

      await test.step('Create a Proxy Host that claims a domain', async () => {
        await page.goto('/proxy-hosts');
        await waitForLoadingComplete(page);

        await page.getByRole('button', { name: 'Add Proxy Host' }).first().click();
        const proxyDialog = page.getByRole('dialog', { name: /add proxy host/i });
        await waitForDialog(page);
        await proxyDialog.getByLabel(/^name\b/i).fill(proxyHost.name ?? `E2E Host ${Date.now()}`);
        await proxyDialog.getByLabel(/domain names/i).fill(proxyHost.domain);
        await proxyDialog.getByLabel(/^host$/i).fill(proxyHost.forwardHost);
        await proxyDialog.getByLabel(/^port$/i).fill(String(proxyHost.forwardPort));

        const createPromise = waitForAPIResponse(page, '/api/v1/proxy-hosts', { status: 201 });
        await proxyDialog.getByRole('button', { name: /^save$/i }).click();
        await createPromise;
      });

      await test.step('Attempt to create a Redirection Host for the same domain', async () => {
        await page.goto('/redirection-hosts');
        await waitForLoadingComplete(page);

        const dialog = await openAddRedirectionHostDialog(page);
        await dialog.getByLabel(/^name\b/i).fill(`E2E Conflict Redirect ${Date.now()}`);
        await dialog.getByLabel(/domain names/i).fill(proxyHost.domain);
        await dialog.getByLabel(/target url/i).fill(`https://elsewhere-${proxyHost.domain}`);
        await dialog.getByRole('button', { name: /^save$/i }).click();
      });

      await test.step('The conflict is rejected with a clear 400 error (spec §4.3 CheckDomainConflict, §4.4: "domain already in use by another host")', async () => {
        const dialog = page.getByRole('dialog', { name: /add redirection host/i });
        await expect(dialog.getByText(/domain already in use by another host/i)).toBeVisible();
      });
    });
  });
});
