/**
 * Requests routed through a proxy host that targets Charon itself.
 *
 * A proxy host whose upstream is Charon's own listen port is a supported
 * configuration. These tests cover how such requests behave end to end:
 *
 * - the app is reachable through the proxy hop (HTTP and a rendered page),
 * - the client address the app resolves for a request that came through the
 *   hop is the connecting peer, and forwarding headers supplied by the client
 *   are not reflected,
 * - the per-process value the proxy attaches to the hop never shows up in API
 *   responses, the proxy access log, the container log or the app log file.
 *
 * The live value is read from the proxy's admin API (published on loopback in
 * the E2E compose files) so the log checks compare against the real value. It
 * is only ever used inside `includes()` checks: never logged, never passed to
 * an assertion message, never attached to the report.
 *
 * The emergency server (a separate listener) is covered by the last describe
 * block. The runner is inside the management networks in this environment, so
 * the "outside the management networks" rejection is covered by Go unit tests;
 * here we assert the listener ignores forwarding headers and keeps requiring
 * credentials.
 */

import { execFileSync } from 'node:child_process';

import { test, expect, loginUser } from '../fixtures/auth-fixtures';
import { EMERGENCY_SERVER } from '../fixtures/security';
import { caddyProxyOrigin, getAuthTokenFromPage } from '../utils/api-helpers';
import { waitForLoadingComplete } from '../utils/wait-helpers';

const CHARON_PORT = 8080;
const ADMIN_API = process.env.PLAYWRIGHT_CADDY_ADMIN_URL || 'http://127.0.0.1:2019';
const CONTAINER_CANDIDATES = [
  process.env.PLAYWRIGHT_CONTAINER_NAME,
  'charon-e2e',
  'charon-playwright',
].filter((n): n is string => Boolean(n));

const HOP_SECRET_HEADER = 'X-Charon-Self-Hop';
const HOP_CLIENT_HEADER = 'X-Charon-Self-Hop-Client';

// Documentation-range addresses that no runner can legitimately have.
const SUPPLIED_FORWARDED_ADDRESS = '203.0.113.77';
const SUPPLIED_HOP_CLIENT_ADDRESS = '198.51.100.23';
const SUPPLIED_HOP_SECRET = 'supplied-value-0123456789abcdef0123456789abcdef';

function resolveContainer(): string {
  for (const name of CONTAINER_CANDIDATES) {
    try {
      execFileSync('docker', ['inspect', '--format', '{{.Id}}', name], { stdio: 'pipe' });
      return name;
    } catch {
      // try the next candidate
    }
  }
  throw new Error(`No E2E container found among: ${CONTAINER_CANDIDATES.join(', ')}`);
}

function dockerLogs(container: string): string {
  return execFileSync('docker', ['logs', container], {
    stdio: 'pipe',
    encoding: 'utf8',
    maxBuffer: 256 * 1024 * 1024,
  }) as string;
}

function containerFile(container: string, path: string): string {
  try {
    return execFileSync('docker', ['exec', container, 'cat', path], {
      stdio: 'pipe',
      encoding: 'utf8',
      maxBuffer: 256 * 1024 * 1024,
    }) as string;
  } catch {
    return '';
  }
}

/** Extract the live hop value from the proxy's loaded configuration. */
async function readLiveHopValue(
  request: import('@playwright/test').APIRequestContext
): Promise<string> {
  const response = await request.get(`${ADMIN_API}/config/`);
  expect(response.ok()).toBeTruthy();
  const config = await response.text();
  const match = new RegExp(`"${HOP_SECRET_HEADER}":\\["([^"]+)"\\]`).exec(config);
  return match ? match[1] : '';
}

test.describe('Requests through a proxy host that targets Charon @security', () => {
  test.describe.configure({ mode: 'serial' });

  let domain: string;
  let token: string;
  let origin: string;

  test.beforeEach(async ({ page, adminUser, testData }) => {
    await loginUser(page, adminUser);
    await waitForLoadingComplete(page);
    token = await getAuthTokenFromPage(page);
    origin = caddyProxyOrigin(page);

    const host = await testData.createProxyHost({
      domain: `self-target-${Date.now()}.example.test`,
      forwardHost: '127.0.0.1',
      forwardPort: CHARON_PORT,
      name: 'Self target host',
    });
    domain = host.domain;

    // Wait until the proxy has loaded the route for the new host.
    await expect
      .poll(
        async () => {
          const r = await page.request.get(`${origin}/api/v1/health`, {
            headers: { Host: domain },
            failOnStatusCode: false,
          });
          return r.status();
        },
        { timeout: 30_000, message: 'proxy host route should become reachable' }
      )
      .toBe(200);
  });

  test('serves the health endpoint through a proxy host', async ({ page }) => {
    const response = await page.request.get(`${origin}/api/v1/health`, {
      headers: { Host: domain },
    });
    expect(response.status()).toBe(200);
    await expect(response).toBeOK();
  });

  test('renders the sign-in page through a proxy host', async ({ browser }) => {
    const context = await browser.newContext();
    const page = await context.newPage();

    // The browser cannot resolve the test hostname, so each request for it is
    // forwarded to the proxy port with the hostname as the Host header.
    await page.route(`http://${domain}/**`, async (route) => {
      const url = new URL(route.request().url());
      const response = await route.fetch({
        url: `${origin}${url.pathname}${url.search}`,
        headers: { ...route.request().headers(), host: domain },
      });
      await route.fulfill({ response });
    });

    try {
      await page.goto(`http://${domain}/login`);
      await expect(page.getByRole('button', { name: /sign in|log in|login/i })).toBeVisible();
      await expect(page.getByRole('textbox', { name: /email/i })).toBeVisible();
      await expect(page.getByLabel(/password/i).first()).toBeVisible();
      await expect(page.locator('form').first()).toMatchAriaSnapshot(`
        - text: Email
        - textbox "Email"
        - text: Password
        - textbox "Password"
        - button "Sign In"
      `);
    } finally {
      await context.close();
    }
  });

  test('resolves the connecting peer as the client address through a proxy host', async ({
    page,
  }) => {
    let direct: { ip: string; source: string };
    let throughHop: { ip: string; source: string };

    await test.step('Read the address the app reports for a direct request', async () => {
      const response = await page.request.get('/api/v1/system/my-ip');
      expect(response.status()).toBe(200);
      direct = await response.json();
      expect(direct.ip).toMatch(/^[0-9a-f.:]+$/i);
    });

    await test.step('Read the address reported for a request through the hop', async () => {
      const response = await page.request.get(`${origin}/api/v1/system/my-ip`, {
        headers: { Host: domain, Authorization: `Bearer ${token}` },
      });
      expect(response.status()).toBe(200);
      throughHop = await response.json();
      expect(throughHop.ip).toMatch(/^[0-9a-f.:]+$/i);
      expect(throughHop.source).toBe('forwarded');
    });

    await test.step('The address through the hop is the connecting peer', async () => {
      expect(throughHop.ip).not.toBe('');
      expect(throughHop.ip).toBe(direct.ip);
    });
  });

  test('ignores forwarding headers supplied by the client through a proxy host', async ({
    page,
  }) => {
    const baseline = await (
      await page.request.get(`${origin}/api/v1/system/my-ip`, {
        headers: { Host: domain, Authorization: `Bearer ${token}` },
      })
    ).json();

    // The E2E compose file lists loopback and the private ranges in
    // CHARON_TRUSTED_PROXIES (needed by the auth-rate-limit spec). A peer the
    // operator configured as trusted keeps the configured forwarded-header
    // handling by design, so a supplied X-Forwarded-For is not asserted here;
    // the default (no trusted proxies) path is covered by the Go unit tests.
    // The proxy always overwrites X-Real-IP with the connecting peer, which
    // holds under any configuration.
    await test.step('A supplied real-IP value does not change the resolved address', async () => {
      const response = await page.request.get(`${origin}/api/v1/system/my-ip`, {
        headers: {
          Host: domain,
          Authorization: `Bearer ${token}`,
          'X-Real-IP': SUPPLIED_FORWARDED_ADDRESS,
        },
      });
      expect(response.status()).toBe(200);
      const body = await response.json();
      expect(body.ip).toBe(baseline.ip);
      expect(JSON.stringify(body)).not.toContain(SUPPLIED_FORWARDED_ADDRESS);
    });

    await test.step('Supplied hop headers are not reflected and do not change the address', async () => {
      const response = await page.request.get(`${origin}/api/v1/system/my-ip`, {
        headers: {
          Host: domain,
          Authorization: `Bearer ${token}`,
          [HOP_SECRET_HEADER]: SUPPLIED_HOP_SECRET,
          [HOP_CLIENT_HEADER]: SUPPLIED_HOP_CLIENT_ADDRESS,
        },
      });
      expect(response.status()).toBe(200);
      const body = await response.json();
      expect(body.ip).toBe(baseline.ip);
      const reflected = JSON.stringify(body) + JSON.stringify(response.headers());
      expect(reflected).not.toContain(SUPPLIED_HOP_SECRET);
      expect(reflected).not.toContain(SUPPLIED_HOP_CLIENT_ADDRESS);
    });

    await test.step('Supplied hop headers sent straight to the app are ignored', async () => {
      const response = await page.request.get('/api/v1/system/my-ip', {
        headers: {
          [HOP_SECRET_HEADER]: SUPPLIED_HOP_SECRET,
          [HOP_CLIENT_HEADER]: SUPPLIED_HOP_CLIENT_ADDRESS,
        },
      });
      expect(response.status()).toBe(200);
      const body = await response.json();
      expect(body.ip).not.toBe(SUPPLIED_HOP_CLIENT_ADDRESS);
      expect(JSON.stringify(body)).not.toContain(SUPPLIED_HOP_SECRET);
    });
  });

  test('keeps the per-process hop value out of responses and logs', async ({ page, request }) => {
    const container = resolveContainer();

    // The value is read from the proxy's admin API (loopback). If that read
    // ever stops working this assertion fails rather than silently weakening
    // the log checks below.
    const live = await readLiveHopValue(request);
    expect(live, 'live hop value should be readable from the proxy configuration').toMatch(
      /^[0-9a-f]{32,}$/i
    );

    const responseBodies: string[] = [];
    const collect = async (r: import('@playwright/test').APIResponse): Promise<void> => {
      responseBodies.push(await r.text(), JSON.stringify(r.headers()));
    };

    await test.step('Exercise the app through the hop and directly', async () => {
      const through = { Host: domain, Authorization: `Bearer ${token}` };
      for (const path of [
        '/api/v1/system/my-ip',
        '/api/v1/health',
        '/api/v1/proxy-hosts',
        '/api/v1/security/status',
        '/login',
      ]) {
        await collect(
          await page.request.get(`${origin}${path}`, {
            headers: through,
            failOnStatusCode: false,
          })
        );
      }
      // Requests carrying supplied hop headers also leave log lines behind.
      await collect(
        await page.request.get(`${origin}/api/v1/system/my-ip`, {
          headers: { ...through, [HOP_SECRET_HEADER]: SUPPLIED_HOP_SECRET },
          failOnStatusCode: false,
        })
      );
      for (const path of ['/api/v1/system/my-ip', '/api/v1/proxy-hosts', '/api/v1/security/status']) {
        await collect(await page.request.get(path, { failOnStatusCode: false }));
      }
    });

    await test.step('The value is absent from every collected API response', async () => {
      for (const body of responseBodies) {
        expect(body.includes(live)).toBe(false);
      }
    });

    await test.step('The value is absent from the proxy access log', async () => {
      const accessLog = containerFile(container, '/var/log/caddy/access.log');
      // The log must actually contain the requests made above, otherwise an
      // empty file would pass trivially.
      expect(accessLog).toContain(domain);
      expect(accessLog.includes(live)).toBe(false);
    });

    await test.step('The value is absent from the container and app logs', async () => {
      const containerLog = dockerLogs(container);
      expect(containerLog.length).toBeGreaterThan(0);
      expect(containerLog.includes(live)).toBe(false);

      const appLog = containerFile(container, '/app/data/logs/charon.log');
      expect(appLog.length).toBeGreaterThan(0);
      expect(appLog.includes(live)).toBe(false);
    });
  });
});

test.describe('Emergency server request handling @security', () => {
  test('uses the connecting address and requires credentials regardless of forwarding headers', async ({
    request,
  }) => {
    const container = resolveContainer();

    await test.step('Health answers without credentials when forwarding headers are supplied', async () => {
      const response = await request.get(`${EMERGENCY_SERVER.baseURL}/health`, {
        headers: {
          'X-Forwarded-For': SUPPLIED_FORWARDED_ADDRESS,
          [HOP_SECRET_HEADER]: SUPPLIED_HOP_SECRET,
          [HOP_CLIENT_HEADER]: SUPPLIED_HOP_CLIENT_ADDRESS,
        },
      });
      expect(response.status()).toBe(200);
      expect((await response.json()).server).toBe('emergency');
    });

    await test.step('Protected routes still require credentials', async () => {
      const response = await request.post(`${EMERGENCY_SERVER.baseURL}/emergency/security-reset`, {
        headers: {
          'X-Forwarded-For': SUPPLIED_FORWARDED_ADDRESS,
          [HOP_SECRET_HEADER]: SUPPLIED_HOP_SECRET,
          [HOP_CLIENT_HEADER]: SUPPLIED_HOP_CLIENT_ADDRESS,
        },
        failOnStatusCode: false,
      });
      expect(response.status()).toBe(401);
    });

    await test.step('The request log records the connecting address only', async () => {
      const appLog = containerFile(container, '/app/data/logs/charon.log');
      const emergencyLines = appLog.split('\n').filter((l) => l.includes('Emergency server request'));
      expect(emergencyLines.length).toBeGreaterThan(0);
      for (const line of emergencyLines) {
        expect(line).not.toContain(SUPPLIED_FORWARDED_ADDRESS);
        expect(line).not.toContain(SUPPLIED_HOP_CLIENT_ADDRESS);
      }
    });
  });
});
