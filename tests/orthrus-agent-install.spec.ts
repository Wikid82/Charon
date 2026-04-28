import { test, expect } from './fixtures/test';
import { waitForAPIHealth } from './utils/api-helpers';
import { waitForLoadingComplete } from './utils/wait-helpers';

const REMOTE_SERVERS_API = '**/api/v1/remote-servers*';
const HECATE_STATUS_API = '**/api/v1/hecate/status*';
const ORTHRUS_AGENTS_API = '**/api/v1/orthrus/agents';
const ORTHRUS_AGENT_SNIPPETS_API = '**/api/v1/orthrus/agents/*/snippets';

const MOCK_AGENT_UUID = 'dddddddd-0000-0000-0000-000000000099';
const MOCK_AUTH_KEY = 'oth_test_key_abcdef1234567890abcdef';
const MOCK_AGENT_NAME = 'My Test Orthrus Agent';

const MOCK_SNIPPETS = {
  docker_compose: `services:\n  orthrus:\n    image: ghcr.io/charon/orthrus:latest\n    environment:\n      AUTH_KEY: ${MOCK_AUTH_KEY}`,
  systemd: `[Unit]\nDescription=Orthrus Agent\n\n[Service]\nExecStart=/usr/bin/orthrus --auth-key ${MOCK_AUTH_KEY}`,
  tarball: `curl -fsSL https://charon.example.com/orthrus/install.sh | AUTH_KEY=${MOCK_AUTH_KEY} bash`,
  homebrew: `brew install charon/tap/orthrus\northrus start --auth-key ${MOCK_AUTH_KEY}`,
  kubernetes_daemonset: `apiVersion: apps/v1\nkind: DaemonSet\nmetadata:\n  name: orthrus\nspec:\n  AUTH_KEY: ${MOCK_AUTH_KEY}`,
};

async function openOrthrusWizard(page: import('@playwright/test').Page) {
  await page.route(REMOTE_SERVERS_API, (route) => {
    route.fulfill({ json: [] });
  });
  await page.route(HECATE_STATUS_API, (route) => {
    route.fulfill({ json: [] });
  });
  await page.route(ORTHRUS_AGENT_SNIPPETS_API, (route) => {
    route.fulfill({ json: MOCK_SNIPPETS });
  });
  await page.route(ORTHRUS_AGENTS_API, (route) => {
    if (route.request().method() === 'POST') {
      route.fulfill({
        json: {
          agent: {
            uuid: MOCK_AGENT_UUID,
            name: MOCK_AGENT_NAME,
            enabled: true,
          },
          auth_key: MOCK_AUTH_KEY,
        },
      });
    } else {
      route.fulfill({ json: [] });
    }
  });

  await page.goto('/remote-servers');
  await waitForLoadingComplete(page);

  const addButton = page.getByRole('button', { name: /add server/i }).first();
  await expect(addButton).toBeVisible({ timeout: 10000 });
  await addButton.click();

  await expect(page.getByRole('heading', { name: /add remote server/i })).toBeVisible({ timeout: 5000 });

  const connectionTypeSelect = page.locator('#connection-type');
  await connectionTypeSelect.selectOption('orthrus');

  await expect(page.getByRole('button', { name: /provision.*agent/i })).toBeVisible({ timeout: 5000 });

  const nameInput = page.getByRole('textbox', { name: /name/i }).first();
  if (await nameInput.isVisible()) {
    await nameInput.fill(MOCK_AGENT_NAME);
  }

  const provisionButton = page.getByRole('button', { name: /provision.*agent/i });
  await provisionButton.click();

  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible({ timeout: 8000 });

  return dialog;
}

test.describe('Orthrus Agent Install Wizard', () => {
  test.beforeEach(async ({ request }) => {
    await waitForAPIHealth(request);
  });

  test.describe('Dialog Structure and Accessibility', () => {
    test('should display wizard dialog with correct title after provisioning', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Verify dialog title is "Install Orthrus Agent"', async () => {
        const dialogTitle = dialog.getByRole('heading', { name: /install orthrus agent/i });
        await expect(dialogTitle).toBeVisible({ timeout: 5000 });
      });
    });

    test('should display authentication key in read-only input', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Verify auth key input has correct value', async () => {
        const authKeyInput = dialog.locator('#auth-key-input');
        await expect(authKeyInput).toBeVisible({ timeout: 5000 });
        await expect(authKeyInput).toHaveValue(MOCK_AUTH_KEY);
      });

      await test.step('Verify auth key input is read-only', async () => {
        const authKeyInput = dialog.locator('#auth-key-input');
        await expect(authKeyInput).toHaveAttribute('readonly', '');
      });

      await test.step('Verify auth key input has accessible label', async () => {
        const authKeyInput = dialog.locator('#auth-key-input');
        const ariaLabel = await authKeyInput.getAttribute('aria-label');
        expect(ariaLabel).toMatch(/authentication key/i);
      });
    });

    test('should display "shown only once" warning for auth key', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Verify one-time key warning is visible', async () => {
        await expect(dialog.getByText(/shown only once/i)).toBeVisible({ timeout: 5000 });
      });
    });

    test('should have accessible Copy authentication key button', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Verify copy auth key button is accessible', async () => {
        const copyKeyButton = dialog.getByRole('button', { name: /copy authentication key/i });
        await expect(copyKeyButton).toBeVisible({ timeout: 5000 });
      });
    });

    test('should have accessible Done button to close wizard', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Verify Done button is visible', async () => {
        const doneButton = dialog.getByRole('button', { name: /done/i });
        await expect(doneButton).toBeVisible({ timeout: 5000 });
      });

      await test.step('Done button closes the dialog', async () => {
        const doneButton = dialog.getByRole('button', { name: /done/i });
        await doneButton.click();
        await expect(dialog).not.toBeVisible({ timeout: 5000 });
      });
    });
  });

  test.describe('Install Tabs Navigation', () => {
    test('should display all five installation method tabs', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Verify all tabs are present', async () => {
        const tabList = dialog.getByRole('tablist');
        await expect(tabList).toBeVisible({ timeout: 5000 });

        await expect(dialog.getByRole('tab', { name: /docker compose/i })).toBeVisible();
        await expect(dialog.getByRole('tab', { name: /^systemd$/i })).toBeVisible();
        await expect(dialog.getByRole('tab', { name: /^tarball$/i })).toBeVisible();
        await expect(dialog.getByRole('tab', { name: /^homebrew$/i })).toBeVisible();
        await expect(dialog.getByRole('tab', { name: /^kubernetes$/i })).toBeVisible();
      });
    });

    test('Docker Compose tab is selected by default', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Verify Docker Compose tab is active by default', async () => {
        const dockerTab = dialog.getByRole('tab', { name: /docker compose/i });
        await expect(dockerTab).toHaveAttribute('data-state', 'active');
      });

      await test.step('Verify Docker Compose snippet content is shown', async () => {
        const activePanel = dialog.locator('[role="tabpanel"][data-state="active"]');
        await expect(activePanel).toBeVisible({ timeout: 5000 });
        await expect(activePanel.getByText(/services:/i)).toBeVisible();
      });
    });

    test('Systemd tab shows systemd snippet content', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Click Systemd tab', async () => {
        await dialog.getByRole('tab', { name: /^systemd$/i }).click();
      });

      await test.step('Verify systemd snippet content is displayed', async () => {
        const activePanel = dialog.locator('[role="tabpanel"][data-state="active"]');
        await expect(activePanel).toBeVisible({ timeout: 5000 });
        await expect(activePanel.getByText(/ExecStart|systemd|Unit/i)).toBeVisible();
      });
    });

    test('Tarball tab shows tarball snippet content', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Click Tarball tab', async () => {
        await dialog.getByRole('tab', { name: /^tarball$/i }).click();
      });

      await test.step('Verify tarball snippet content is displayed', async () => {
        const activePanel = dialog.locator('[role="tabpanel"][data-state="active"]');
        await expect(activePanel).toBeVisible({ timeout: 5000 });
        await expect(activePanel.getByText(/curl|install/i)).toBeVisible();
      });
    });

    test('Homebrew tab shows homebrew snippet content', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Click Homebrew tab', async () => {
        await dialog.getByRole('tab', { name: /^homebrew$/i }).click();
      });

      await test.step('Verify homebrew snippet content is displayed', async () => {
        const activePanel = dialog.locator('[role="tabpanel"][data-state="active"]');
        await expect(activePanel).toBeVisible({ timeout: 5000 });
        await expect(activePanel.getByText(/brew/i)).toBeVisible();
      });
    });

    test('Kubernetes tab shows kubernetes snippet content', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Click Kubernetes tab', async () => {
        await dialog.getByRole('tab', { name: /^kubernetes$/i }).click();
      });

      await test.step('Verify kubernetes snippet content is displayed', async () => {
        const activePanel = dialog.locator('[role="tabpanel"][data-state="active"]');
        await expect(activePanel).toBeVisible({ timeout: 5000 });
        await expect(activePanel.getByText(/DaemonSet|daemonset|apiVersion/i)).toBeVisible();
      });
    });

    test('Copy snippet button is accessible in each tab', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);
      const tabs = [
        { name: /docker compose/i },
        { name: /^systemd$/i },
        { name: /^tarball$/i },
        { name: /^homebrew$/i },
        { name: /^kubernetes$/i },
      ];

      for (const tab of tabs) {
        await test.step(`Verify Copy snippet button is accessible on ${tab.name} tab`, async () => {
          await dialog.getByRole('tab', tab).click();
          const copySnippetButton = dialog.getByRole('button', { name: /copy snippet/i });
          await expect(copySnippetButton).toBeVisible({ timeout: 3000 });
        });
      }
    });
  });

  test.describe('Snippet Content Validation', () => {
    test('snippets contain the actual auth key value (backend substitutes placeholder before returning)', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Check Docker Compose tab snippet contains auth key value', async () => {
        const activePanel = dialog.locator('[role="tabpanel"][data-state="active"]');
        await expect(activePanel).toBeVisible({ timeout: 5000 });
        const snippetText = await activePanel.textContent();
        // Backend returns snippets with the actual key already substituted
        expect(snippetText).toContain(MOCK_AUTH_KEY);
      });
    });
  });

  test.describe('Troubleshooting Panel', () => {
    test('should display collapsed troubleshooting section', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Verify troubleshooting details element is present', async () => {
        const troubleshootingDetails = dialog.locator('details');
        await expect(troubleshootingDetails).toBeVisible({ timeout: 5000 });
      });

      await test.step('Verify troubleshooting summary text is visible', async () => {
        const summary = dialog.locator('details > summary');
        await expect(summary).toBeVisible();
        await expect(summary).toContainText(/troubleshooting/i);
      });

      await test.step('Verify troubleshooting hints are hidden by default', async () => {
        const troubleshootingDetails = dialog.locator('details');
        const isOpen = await troubleshootingDetails.getAttribute('open');
        // details element should be closed by default (no "open" attribute)
        expect(isOpen).toBeNull();
      });
    });

    test('clicking troubleshooting summary expands the hints', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Click the troubleshooting summary to expand', async () => {
        const summary = dialog.locator('details > summary');
        await summary.click();
      });

      await test.step('Verify troubleshooting hints become visible', async () => {
        const troubleshootingDetails = dialog.locator('details');
        const isOpen = await troubleshootingDetails.getAttribute('open');
        expect(isOpen).not.toBeNull();

        await expect(dialog.getByText(/orthrus agent service is running/i)).toBeVisible({ timeout: 3000 });
        await expect(dialog.getByText(/authentication key was copied/i)).toBeVisible();
        await expect(dialog.getByText(/ports are open/i)).toBeVisible();
      });
    });
  });

  test.describe('Accessibility Snapshot', () => {
    test('wizard dialog has correct ARIA structure', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Verify dialog role and heading', async () => {
        await expect(dialog).toMatchAriaSnapshot(`
          - dialog "Install Orthrus Agent":
            - heading "Install Orthrus Agent" [level=2]
        `);
      });
    });

    test('auth key section has correct accessibility structure', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Verify auth key input accessibility', async () => {
        const authKeyInput = dialog.locator('#auth-key-input');
        await expect(authKeyInput).toMatchAriaSnapshot(`
          - textbox "Authentication Key"
        `);
        await expect(authKeyInput).toHaveValue(MOCK_AUTH_KEY);
      });
    });

    test('tab list aria structure is correct', async ({ page }) => {
      const dialog = await openOrthrusWizard(page);

      await test.step('Verify tabs aria snapshot', async () => {
        const tabList = dialog.getByRole('tablist');
        await expect(tabList).toMatchAriaSnapshot(`
          - tablist:
            - tab "Docker Compose" [selected]
            - tab "Systemd"
            - tab "Tarball"
            - tab "Homebrew"
            - tab "Kubernetes"
        `);
      });
    });
  });
});
