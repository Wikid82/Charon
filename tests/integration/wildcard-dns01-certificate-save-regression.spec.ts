/**
 * Wildcard DNS-01 Certificate Save — Regression Test (GitHub #1361)
 *
 * Repro: saving a wildcard proxy host with a built-in DNS provider (e.g.
 * Cloudflare) attached fails because the root `Dockerfile`'s xcaddy build
 * never compiles in any `github.com/caddy-dns/*` module — Caddy's admin API
 * rejects the resulting config with `unknown module: dns.providers.cloudflare`
 * (400), and because `ApplyConfig` does an atomic `/load`,
 * `internal/caddy/manager.go` rolls back the *entire* config, poisoning
 * unrelated saves too. See docs/plans/current_spec.md §1.1 / §2 for the full
 * root-cause analysis.
 *
 * Written as `test.fixme` in Commit 1 (docs/plans/current_spec.md §6) since
 * the fix (Commit 2: Dockerfile `--with` additions) hasn't landed on this
 * branch yet — this spec is expected to fail against the current toolchain
 * image. Un-`fixme`'d in Commit 4 once Commits 2/2b/3 have landed and the
 * `dns.providers.*` module IDs actually resolve.
 *
 * See docs/plans/current_spec.md §6 Commit 1 / Commit 4, and §5 Acceptance
 * Criteria ("A wildcard proxy host with a Cloudflare ... DNS provider
 * attached, saved through the UI ... results in a successful Caddy /load").
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { generateDomain } from '../fixtures/test-data';
import { generateProxyHost } from '../fixtures/proxy-hosts';
import { waitForLoadingComplete, waitForDebounce } from '../utils/wait-helpers';

/**
 * Dismiss the "New Base Domain Detected" dialog if it appears after typing
 * a domain into the proxy host form — same helper pattern as
 * tests/core/proxy-hosts.spec.ts.
 */
async function dismissDomainDialog(page: import('@playwright/test').Page): Promise<void> {
  const noThanksBtn = page.getByRole('button', { name: /No, thanks/i });
  if (await noThanksBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
    await noThanksBtn.click();
    await waitForDebounce(page, { delay: 300 });
  }
}

const getAddHostButton = (page: import('@playwright/test').Page) =>
  page.getByRole('button', { name: /add.*proxy.*host/i }).first();

const getSaveButton = (page: import('@playwright/test').Page) =>
  page.getByRole('button', { name: 'Save', exact: true });

test.describe('Wildcard DNS-01 Certificate Save (Regression #1361)', () => {
  test('should save a wildcard proxy host with a Cloudflare DNS provider without a Caddy /load rollback error', async ({
    page,
    adminUser,
    testData,
  }) => {
    await loginUser(page, adminUser);

    let dnsProvider: { id: string; name: string };
    await test.step('Create a Cloudflare DNS provider via API', async () => {
      dnsProvider = await testData.createDNSProvider({
        providerType: 'cloudflare',
        name: 'Wildcard-Save-Cloudflare',
        credentials: {
          api_token: 'test-cloudflare-token-placeholder',
        },
      });
      expect(dnsProvider.id).toBeTruthy();
    });

    const wildcardBaseDomain = generateDomain('dns1361');
    const wildcardDomain = `*.${wildcardBaseDomain}`;
    const proxyInput = generateProxyHost({ scheme: 'https' });

    await test.step('Navigate to Proxy Hosts and open the create form', async () => {
      await page.goto('/proxy-hosts');
      await waitForLoadingComplete(page);

      await getAddHostButton(page).click();
      await expect(page.getByRole('dialog')).toBeVisible();
    });

    const dialog = page.getByRole('dialog');

    await test.step('Fill in a wildcard domain and forward target', async () => {
      const nameInput = page.locator('#proxy-name');
      await nameInput.fill(`Wildcard DNS-01 Test ${Date.now()}`);

      const domainCombobox = page.locator('#domain-names');
      await domainCombobox.click();
      await page.keyboard.type(wildcardDomain);
      await page.keyboard.press('Tab');

      await dismissDomainDialog(page);

      const forwardHostInput = page.locator('#forward-host');
      await forwardHostInput.fill(proxyInput.forwardHost);

      const forwardPortInput = page.locator('#forward-port');
      await forwardPortInput.clear();
      await forwardPortInput.fill(String(proxyInput.forwardPort));
    });

    await test.step('Verify the wildcard DNS-01 challenge notice is shown', async () => {
      const dnsProviderSection = page.locator('[data-testid="dns-provider-section"]');
      await expect(dnsProviderSection).toBeVisible();
      await expect(dnsProviderSection.getByRole('alert')).toContainText(
        /wildcard certificate required/i
      );
    });

    await test.step('Select the Cloudflare DNS provider', async () => {
      const providerCombobox = dialog.getByRole('combobox', { name: /dns provider/i });
      await providerCombobox.click();
      await page.getByRole('option', { name: new RegExp(dnsProvider.name, 'i') }).click();
    });

    await test.step('Submit the form', async () => {
      await dismissDomainDialog(page);

      await getSaveButton(page).click();

      await dismissDomainDialog(page);

      const confirmDialog = page.getByRole('button', { name: /yes.*save/i });
      if (await confirmDialog.isVisible({ timeout: 2000 }).catch(() => false)) {
        await confirmDialog.click();
      }

      await waitForLoadingComplete(page);
    });

    await test.step('Verify the save succeeded with no Caddy /load rollback error', async () => {
      // This is the actual regression assertion for #1361: before the fix,
      // Caddy's admin API 400s with "unknown module: dns.providers.cloudflare"
      // and internal/caddy/manager.go rolls back the whole config, which the
      // frontend surfaces as an error toast / inline form error (see
      // ProxyHostForm.tsx's `toast.error(message)` / `setError(message)` on a
      // failed `onSubmit`).
      const rollbackError = page.getByText(/unknown module|failed to save host|failed to (apply|load)/i);
      await expect(rollbackError).toHaveCount(0);

      await waitForDebounce(page, { delay: 1000 });

      const hostInList = page.getByText(wildcardDomain);
      await page.waitForURL(/\/proxy-hosts(?!\/)/, { timeout: 5000 }).catch(() => {});
      await waitForLoadingComplete(page);
      await expect(hostInList).toBeVisible();
    });
  });
});
