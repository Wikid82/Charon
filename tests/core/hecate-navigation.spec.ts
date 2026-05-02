/**
 * Hecate Navigation E2E Tests
 *
 * Verifies the Hecate collapsible navigation section:
 * - Sidebar has a collapsible "Hecate" group
 * - Expanding the group reveals: Remote Servers, Tunnels, Providers, Agent
 * - Each item navigates to the correct /hecate/* route
 * - Legacy /remote-servers redirects to /hecate/remote-servers
 */

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { waitForLoadingComplete } from '../utils/wait-helpers';

test.describe('Hecate Navigation', () => {
  test.beforeEach(async ({ page, adminUser }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);

    await page.goto('/');
    await waitForLoadingComplete(page);

    if (page.url().includes('/login')) {
      await loginUser(page, adminUser);
      await waitForLoadingComplete(page);
      await page.goto('/');
      await waitForLoadingComplete(page);
    }
  });

  test('sidebar has a collapsible "Hecate" group', async ({ page }) => {
    await test.step('Verify the Hecate group trigger is visible', async () => {
      const hecateGroup = page
        .getByRole('button', { name: /^hecate$/i })
        .or(page.getByRole('link', { name: /^hecate$/i }))
        .first();

      await expect(hecateGroup).toBeVisible();
    });
  });

  test('expanding the Hecate group reveals all 4 sub-items', async ({ page }) => {
    await test.step('Expand the Hecate group', async () => {
      const hecateGroup = page
        .getByRole('button', { name: /^hecate$/i })
        .or(page.getByRole('link', { name: /^hecate$/i }))
        .first();

      const isExpanded = await hecateGroup.getAttribute('aria-expanded');
      if (isExpanded !== 'true') {
        await hecateGroup.click();
        await waitForLoadingComplete(page);
      }
    });

    await test.step('Verify all 4 Hecate sub-items are visible', async () => {
      await expect(
        page.getByRole('link', { name: /^tunnels$/i }).or(page.getByRole('button', { name: /^tunnels$/i })).first()
      ).toBeVisible();

      await expect(
        page.getByRole('link', { name: /^providers$/i }).or(page.getByRole('button', { name: /^providers$/i })).first()
      ).toBeVisible();

      await expect(
        page.getByRole('link', { name: /^agent$/i }).or(page.getByRole('button', { name: /^agent$/i })).first()
      ).toBeVisible();

      await expect(
        page.getByRole('link', { name: /^remote servers$/i }).or(page.getByRole('button', { name: /^remote servers$/i })).first()
      ).toBeVisible();
    });
  });

  test('clicking "Tunnels" navigates to /hecate/tunnels', async ({ page }) => {
    await test.step('Expand Hecate group and click Tunnels', async () => {
      const hecateGroup = page
        .getByRole('button', { name: /^hecate$/i })
        .or(page.getByRole('link', { name: /^hecate$/i }))
        .first();

      const isExpanded = await hecateGroup.getAttribute('aria-expanded');
      if (isExpanded !== 'true') {
        await hecateGroup.click();
        await waitForLoadingComplete(page);
      }

      const tunnelsLink = page.getByRole('link', { name: /^tunnels$/i }).first();
      await tunnelsLink.click();
      await waitForLoadingComplete(page);
    });

    await test.step('Verify navigation to /hecate/tunnels', async () => {
      await expect(page).toHaveURL(/\/hecate\/tunnels/i);
    });
  });

  test('clicking "Providers" navigates to /hecate/providers', async ({ page }) => {
    await test.step('Expand Hecate group and click Providers', async () => {
      const hecateGroup = page
        .getByRole('button', { name: /^hecate$/i })
        .or(page.getByRole('link', { name: /^hecate$/i }))
        .first();

      const isExpanded = await hecateGroup.getAttribute('aria-expanded');
      if (isExpanded !== 'true') {
        await hecateGroup.click();
        await waitForLoadingComplete(page);
      }

      const providersLink = page.getByRole('link', { name: /^providers$/i }).first();
      await providersLink.click();
      await waitForLoadingComplete(page);
    });

    await test.step('Verify navigation to /hecate/providers', async () => {
      await expect(page).toHaveURL(/\/hecate\/providers/i);
    });
  });

  test('clicking "Agent" navigates to /hecate/agent', async ({ page }) => {
    await test.step('Expand Hecate group and click Agent', async () => {
      const hecateGroup = page
        .getByRole('button', { name: /^hecate$/i })
        .or(page.getByRole('link', { name: /^hecate$/i }))
        .first();

      const isExpanded = await hecateGroup.getAttribute('aria-expanded');
      if (isExpanded !== 'true') {
        await hecateGroup.click();
        await waitForLoadingComplete(page);
      }

      const agentLink = page.getByRole('link', { name: /^agent$/i }).first();
      await agentLink.click();
      await waitForLoadingComplete(page);
    });

    await test.step('Verify navigation to /hecate/agent', async () => {
      await expect(page).toHaveURL(/\/hecate\/agent/i);
    });
  });

  test('clicking "Remote Servers" navigates to /hecate/remote-servers', async ({ page }) => {
    await test.step('Expand Hecate group and click Remote Servers', async () => {
      const hecateGroup = page
        .getByRole('button', { name: /^hecate$/i })
        .or(page.getByRole('link', { name: /^hecate$/i }))
        .first();

      const isExpanded = await hecateGroup.getAttribute('aria-expanded');
      if (isExpanded !== 'true') {
        await hecateGroup.click();
        await waitForLoadingComplete(page);
      }

      const remoteServersLink = page.getByRole('link', { name: /^remote servers$/i }).first();
      await remoteServersLink.click();
      await waitForLoadingComplete(page);
    });

    await test.step('Verify navigation to /hecate/remote-servers', async () => {
      await expect(page).toHaveURL(/\/hecate\/remote-servers/i);
    });
  });

  test('navigating directly to /remote-servers redirects to /hecate/remote-servers', async ({ page }) => {
    await test.step('Navigate to legacy /remote-servers path', async () => {
      await page.goto('/remote-servers');
      await waitForLoadingComplete(page);
    });

    await test.step('Verify redirect to /hecate/remote-servers', async () => {
      await expect(page).toHaveURL(/\/hecate\/remote-servers/i);
    });
  });
});
