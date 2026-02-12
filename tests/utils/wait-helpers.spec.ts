/**
 * Unit tests for wait-helpers.ts - Semantic Wait Helpers
 *
 * These tests verify the behavior of deterministic wait utilities
 * that replace arbitrary `page.waitForTimeout()` calls.
 */

import { test, expect, Page } from '@playwright/test';
import {
  waitForDialog,
  waitForFormFields,
  waitForDebounce,
  waitForConfigReload,
  waitForNavigation,
} from './wait-helpers';

test.describe('wait-helpers - Semantic Wait Functions', () => {
  test.describe('waitForDialog', () => {
    test('should wait for dialog to be visible and interactive', async ({ page }) => {
      // Create a test page with dialog
      await page.setContent(`
        <button id="open-dialog">Open Dialog</button>
        <div role="dialog" id="test-dialog" style="display: none;">
          <h2>Test Dialog</h2>
          <button id="close-dialog">Close</button>
        </div>
        <script>
          document.getElementById('open-dialog').onclick = () => {
            setTimeout(() => {
              document.getElementById('test-dialog').style.display = 'block';
            }, 100);
          };
        </script>
      `);

      await test.step('Open dialog and wait for it to be interactive', async () => {
        await page.click('#open-dialog');
        const dialog = await waitForDialog(page);

        // Verify dialog is visible and interactive
        await expect(dialog).toBeVisible();
        await expect(dialog.getByRole('heading')).toHaveText('Test Dialog');
      });
    });

    test('should handle dialog with aria-busy attribute', async ({ page }) => {
      // Create a dialog that starts busy then becomes interactive
      await page.setContent(`
        <button id="open-dialog">Open Dialog</button>
        <div role="dialog" id="test-dialog" style="display: none;" aria-busy="true">
          <h2>Loading Dialog</h2>
        </div>
        <script>
          document.getElementById('open-dialog').onclick = () => {
            const dialog = document.getElementById('test-dialog');
            dialog.style.display = 'block';
            // Simulate loading complete after 200ms
            setTimeout(() => {
              dialog.removeAttribute('aria-busy');
            }, 200);
          };
        </script>
      `);

      await page.click('#open-dialog');
      const dialog = await waitForDialog(page);

      // Verify dialog is no longer busy
      await expect(dialog).not.toHaveAttribute('aria-busy', 'true');
    });

    test('should handle alertdialog role', async ({ page }) => {
      await page.setContent(`
        <button id="open-alert">Open Alert</button>
        <div role="alertdialog" id="alert-dialog" style="display: none;">
          <h2>Alert Dialog</h2>
        </div>
        <script>
          document.getElementById('open-alert').onclick = () => {
            document.getElementById('alert-dialog').style.display = 'block';
          };
        </script>
      `);

      await page.click('#open-alert');
      const dialog = await waitForDialog(page, { role: 'alertdialog' });

      await expect(dialog).toBeVisible();
      await expect(dialog).toHaveAttribute('role', 'alertdialog');
    });

    test('should timeout if dialog never appears', async ({ page }) => {
      await page.setContent(`<div>No dialog here</div>`);

      await expect(
        waitForDialog(page, { timeout: 1000 })
      ).rejects.toThrow();
    });
  });

  test.describe('waitForFormFields', () => {
    test('should wait for dynamically loaded form fields', async ({ page }) => {
      await page.setContent(`
        <select id="form-type">
          <option value="basic">Basic</option>
          <option value="advanced">Advanced</option>
        </select>
        <div id="dynamic-fields"></div>
        <script>
          document.getElementById('form-type').onchange = (e) => {
            const container = document.getElementById('dynamic-fields');
            if (e.target.value === 'advanced') {
              setTimeout(() => {
                container.innerHTML = '<input type="text" name="advanced-field" id="advanced-field" />';
              }, 100);
            }
          };
        </script>
      `);

      await test.step('Select form type and wait for fields', async () => {
        await page.selectOption('#form-type', 'advanced');
        await waitForFormFields(page, '#advanced-field');

        const field = page.locator('#advanced-field');
        await expect(field).toBeVisible();
        await expect(field).toBeEnabled();
      });
    });

    test('should wait for field to be enabled', async ({ page }) => {
      await page.setContent(`
        <button id="enable-field">Enable Field</button>
        <input type="text" id="test-field" disabled />
        <script>
          document.getElementById('enable-field').onclick = () => {
            setTimeout(() => {
              document.getElementById('test-field').disabled = false;
            }, 100);
          };
        </script>
      `);

      await page.click('#enable-field');
      await waitForFormFields(page, '#test-field', { shouldBeEnabled: true, timeout: 2000 });

      const field = page.locator('#test-field');
      await expect(field).toBeEnabled({ timeout: 2000 });
    });

    test('should handle disabled fields when shouldBeEnabled is false', async ({ page }) => {
      await page.setContent(`
        <input type="text" id="disabled-field" disabled />
      `);

      // Should not throw even though field is disabled
      await waitForFormFields(page, '#disabled-field', { shouldBeEnabled: false });

      const field = page.locator('#disabled-field');
      await expect(field).toBeVisible();
    });
  });

  test.describe('waitForDebounce', () => {
    test('should wait for network idle after input', async ({ page }) => {
      // Create a page with a search that triggers API call
      await page.route('**/api/search*', async (route) => {
        await new Promise(resolve => setTimeout(resolve, 200));
        await route.fulfill({ json: { results: [] } });
      });

      await page.setContent(`
        <input type="text" id="search-input" />
        <script>
          let timeout;
          document.getElementById('search-input').oninput = (e) => {
            clearTimeout(timeout);
            timeout = setTimeout(() => {
              fetch('/api/search?q=' + e.target.value);
            }, 300);
          };
        </script>
      `);

      await test.step('Type and wait for debounce to settle', async () => {
        await page.fill('#search-input', 'test query');
        await waitForDebounce(page);

        // Network should be idle and API called
        // Verify by checking if input is still interactive
        const input = page.locator('#search-input');
        await expect(input).toHaveValue('test query');
      });
    });

    test('should wait for loading indicator', async ({ page }) => {
      await page.setContent(`
        <input type="text" id="search-input" />
        <div class="search-loading" style="display: none;">Searching...</div>
        <script>
          let timeout;
          document.getElementById('search-input').oninput = (e) => {
            const loader = document.querySelector('.search-loading');
            clearTimeout(timeout);
            loader.style.display = 'block';
            timeout = setTimeout(() => {
              loader.style.display = 'none';
            }, 200);
          };
        </script>
      `);

      await page.fill('#search-input', 'test');
      await waitForDebounce(page, { indicatorSelector: '.search-loading' });

      const loader = page.locator('.search-loading');
      await expect(loader).not.toBeVisible();
    });
  });

  test.describe('waitForConfigReload', () => {
    test('should wait for config reload overlay to disappear', async ({ page }) => {
      await page.setContent(`
        <button id="save-settings">Save</button>
        <div role="status" data-testid="config-reload-overlay" style="display: none;">
          Reloading configuration...
        </div>
        <script>
          document.getElementById('save-settings').onclick = () => {
            const overlay = document.querySelector('[data-testid="config-reload-overlay"]');
            overlay.style.display = 'block';
            setTimeout(() => {
              overlay.style.display = 'none';
            }, 500);
          };
        </script>
      `);

      await test.step('Save settings and wait for reload', async () => {
        await page.click('#save-settings');
        await waitForConfigReload(page);

        const overlay = page.locator('[data-testid="config-reload-overlay"]');
        await expect(overlay).not.toBeVisible();
      });
    });

    test('should handle instant reload (no overlay)', async ({ page }) => {
      await page.setContent(`
        <button id="save-settings">Save</button>
        <div>Settings saved</div>
      `);

      // Should not throw even if overlay never appears
      await page.click('#save-settings');
      await waitForConfigReload(page);
    });

    test('should wait for DOM to be interactive after reload', async ({ page }) => {
      await page.setContent(`
        <button id="save-settings">Save</button>
        <div role="status" style="display: none;">Reloading...</div>
        <script>
          document.getElementById('save-settings').onclick = () => {
            const overlay = document.querySelector('[role="status"]');
            overlay.style.display = 'block';
            setTimeout(() => {
              overlay.style.display = 'none';
            }, 300);
          };
        </script>
      `);

      await page.click('#save-settings');
      await waitForConfigReload(page);

      // Page should be interactive
      const button = page.locator('#save-settings');
      await expect(button).toBeEnabled();
    });
  });

  test.describe('waitForNavigation', () => {
    test('should wait for URL change with string match', async ({ page }) => {
      await page.route('**/test-page', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'text/html',
          body: '<h1>New Page</h1>',
        });
      });

      await page.goto('about:blank');
      await page.setContent(`
        <a href="http://127.0.0.1:8080/test-page" id="nav-link">Navigate</a>
      `);

      const link = page.locator('#nav-link');
      await link.click();

      // Wait for navigation to complete
      await waitForNavigation(page, /\/test-page$/);

      // Verify new page loaded
      await expect(page.locator('h1')).toHaveText('New Page');
    });

    test('should wait for URL change with RegExp match', async ({ page }) => {
      await page.goto('about:blank');

      // Navigate to a data URL
      await page.goto('data:text/html,<div id="content">Test Page</div>');

      await waitForNavigation(page, /data:text\/html/);

      const content = page.locator('#content');
      await expect(content).toHaveText('Test Page');
    });

    test('should wait for specified load state', async ({ page }) => {
      await page.goto('about:blank');

      // Navigate with domcontentloaded state
      const navigationPromise = page.goto('data:text/html,<h1>Page</h1>');

      await waitForNavigation(page, /data:text\/html/, { waitUntil: 'domcontentloaded' });

      await navigationPromise;
      await expect(page.locator('h1')).toHaveText('Page');
    });

    test('should timeout if navigation never completes', async ({ page }) => {
      await page.goto('about:blank');

      await expect(
        waitForNavigation(page, /never-matching-url/, { timeout: 1000 })
      ).rejects.toThrow();
    });
  });

  test.describe('Integration tests - Multiple wait helpers', () => {
    test('should handle dialog with form fields and debounced search', async ({ page }) => {
      await page.setContent(`
        <button id="open-dialog">Open Dialog</button>
        <div role="dialog" id="test-dialog" style="display: none;">
          <h2>Search Dialog</h2>
          <input type="text" id="search" />
          <div class="search-loading" style="display: none;">Loading...</div>
          <div id="results"></div>
        </div>
        <script>
          document.getElementById('open-dialog').onclick = () => {
            const dialog = document.getElementById('test-dialog');
            dialog.style.display = 'block';
          };

          let timeout;
          document.getElementById('search').oninput = (e) => {
            const loader = document.querySelector('.search-loading');
            const results = document.getElementById('results');
            clearTimeout(timeout);
            loader.style.display = 'block';
            timeout = setTimeout(() => {
              loader.style.display = 'none';
              results.textContent = 'Results for: ' + e.target.value;
            }, 200);
          };
        </script>
      `);

      await test.step('Open dialog', async () => {
        await page.click('#open-dialog');
        const dialog = await waitForDialog(page);
        await expect(dialog).toBeVisible();
      });

      await test.step('Wait for search field', async () => {
        await waitForFormFields(page, '#search');
        const searchField = page.locator('#search');
        await expect(searchField).toBeEnabled();
      });

      await test.step('Search with debounce', async () => {
        await page.fill('#search', 'test query');
        await waitForDebounce(page, { indicatorSelector: '.search-loading' });

        const results = page.locator('#results');
        await expect(results).toHaveText('Results for: test query');
      });
    });

    test('should handle form submission with config reload', async ({ page }) => {
      await page.setContent(`
        <form id="settings-form">
          <input type="text" id="setting-name" />
          <button type="submit">Save</button>
        </form>
        <div role="status" data-testid="config-reload-overlay" style="display: none;">
          Reloading configuration...
        </div>
        <script>
          document.getElementById('settings-form').onsubmit = (e) => {
            e.preventDefault();
            const overlay = document.querySelector('[data-testid="config-reload-overlay"]');
            overlay.style.display = 'block';
            setTimeout(() => {
              overlay.style.display = 'none';
            }, 300);
          };
        </script>
      `);

      await test.step('Wait for form field and fill', async () => {
        await waitForFormFields(page, '#setting-name');
        await page.fill('#setting-name', 'test value');
      });

      await test.step('Submit and wait for config reload', async () => {
        await page.click('button[type="submit"]');
        await waitForConfigReload(page);

        const overlay = page.locator('[data-testid="config-reload-overlay"]');
        await expect(overlay).not.toBeVisible();
      });
    });
  });

  test.describe('Error handling and edge cases', () => {
    test('waitForDialog should handle multiple dialogs', async ({ page }) => {
      await page.setContent(`
        <div role="dialog" class="dialog-1">Dialog 1</div>
        <div role="dialog" class="dialog-2" style="display: none;">Dialog 2</div>
      `);

      // Should find the first visible dialog
      const dialog = await waitForDialog(page);
      await expect(dialog).toHaveClass(/dialog-1/);
    });

    test('waitForFormFields should handle detached elements', async ({ page }) => {
      await page.setContent(`
        <button id="add-field">Add Field</button>
        <div id="container"></div>
        <script>
          document.getElementById('add-field').onclick = () => {
            const container = document.getElementById('container');
            container.innerHTML = '';
            setTimeout(() => {
              container.innerHTML = '<input type="text" id="new-field" />';
            }, 100);
          };
        </script>
      `);

      await page.click('#add-field');
      await waitForFormFields(page, '#new-field');

      const field = page.locator('#new-field');
      await expect(field).toBeAttached();
    });

    test('waitForDebounce should handle rapid consecutive inputs', async ({ page }) => {
      await page.setContent(`
        <input type="text" id="rapid-input" />
        <div class="loading" style="display: none;">Loading...</div>
        <script>
          let timeout;
          document.getElementById('rapid-input').oninput = () => {
            const loader = document.querySelector('.loading');
            clearTimeout(timeout);
            loader.style.display = 'block';
            timeout = setTimeout(() => {
              loader.style.display = 'none';
            }, 200);
          };
        </script>
      `);

      // Rapid typing simulation
      await page.fill('#rapid-input', 'a');
      await page.fill('#rapid-input', 'ab');
      await page.fill('#rapid-input', 'abc');

      await waitForDebounce(page, { indicatorSelector: '.loading' });

      const loader = page.locator('.loading');
      await expect(loader).not.toBeVisible();
    });
  });
});
