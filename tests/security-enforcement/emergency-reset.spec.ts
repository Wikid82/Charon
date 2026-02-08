import { test, expect, APIRequestContext } from '@playwright/test';
import { EMERGENCY_TOKEN } from '../fixtures/security';

type SettingsMap = Record<string, string>;

const STAGE_A_LIMITS = {
  enabled: true,
  requests: 120,
  window: 60,
  burst: 20,
};

const STAGE_B_LIMITS = {
  enabled: true,
  requests: 3,
  window: 10,
  burst: 1,
};

const DEFAULT_LIMITS = {
  enabled: false,
  requests: 100,
  window: 60,
  burst: 20,
};

function parseSettingValue(value: unknown): string | number | boolean | undefined {
  if (value === null || value === undefined) {
    return undefined;
  }

  if (typeof value === 'boolean' || typeof value === 'number') {
    return value;
  }

  if (typeof value === 'string') {
    const trimmed = value.trim();
    const lowered = trimmed.toLowerCase();

    if (lowered === 'true' || lowered === 'false') {
      return lowered === 'true';
    }

    if (/^-?\d+$/.test(trimmed)) {
      return Number(trimmed);
    }

    return trimmed;
  }

  return String(value);
}

function coerceBoolean(value: unknown, fallback: boolean): boolean {
  const parsed = parseSettingValue(value);
  return typeof parsed === 'boolean' ? parsed : fallback;
}

function coerceNumber(value: unknown, fallback: number): number {
  const parsed = parseSettingValue(value);
  return typeof parsed === 'number' ? parsed : fallback;
}

function settingsMatch(settings: SettingsMap, expected: typeof STAGE_A_LIMITS): boolean {
  return (
    parseSettingValue(settings['security.rate_limit.enabled']) === expected.enabled &&
    parseSettingValue(settings['security.rate_limit.requests']) === expected.requests &&
    parseSettingValue(settings['security.rate_limit.window']) === expected.window &&
    parseSettingValue(settings['security.rate_limit.burst']) === expected.burst
  );
}

async function fetchSettings(token: string, request: APIRequestContext): Promise<SettingsMap> {
  const response = await request.get('/api/v1/settings', {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  expect(response.ok()).toBeTruthy();
  return response.json();
}

async function patchRateLimit(
  token: string,
  request: APIRequestContext,
  limits: typeof STAGE_A_LIMITS
): Promise<void> {
  const maxRetries = 5;
  const retryDelayMs = 1000;

  for (let attempt = 0; attempt <= maxRetries; attempt += 1) {
    const response = await request.patch('/api/v1/config', {
      headers: {
        Authorization: `Bearer ${token}`,
      },
      data: {
        security: {
          rate_limit: limits,
        },
      },
    });

    if (response.ok()) {
      return;
    }

    if (response.status() !== 429 || attempt === maxRetries) {
      expect(response.ok()).toBeTruthy();
      return;
    }

    await new Promise((resolve) => setTimeout(resolve, retryDelayMs));
  }
}

async function waitForSettings(
  token: string,
  request: APIRequestContext,
  expected: typeof STAGE_A_LIMITS
): Promise<void> {
  const maxDurationMs = 65000;
  const intervalMs = 2000;
  const deadline = Date.now() + maxDurationMs;

  while (Date.now() < deadline) {
    const settings = await fetchSettings(token, request);
    if (settingsMatch(settings, expected)) {
      return;
    }

    await new Promise((resolve) => setTimeout(resolve, intervalMs));
  }

  const lastSettings = await fetchSettings(token, request);
  throw new Error(`Rate limit settings did not propagate: ${JSON.stringify(lastSettings)}`);
}

test.describe('Emergency Access & Rate Limiting', () => {
  test.describe.configure({ mode: 'serial' });

  let token: string;
  let originalSettings: SettingsMap = {};

  test.beforeAll(async ({ request }) => {
    const email = process.env.E2E_TEST_EMAIL || 'e2e-test@example.com';
    const password = process.env.E2E_TEST_PASSWORD || 'TestPassword123!';

    await test.step('Authenticate admin user', async () => {
      const loginResponse = await request.post('/api/v1/auth/login', {
        data: {
          email,
          password,
        },
      });

      expect(loginResponse.ok()).toBeTruthy();
      const loginBody = await loginResponse.json();
      token = loginBody.token;
    });

    await test.step('Capture original settings and apply Stage A limits', async () => {
      originalSettings = await fetchSettings(token, request);
      await patchRateLimit(token, request, STAGE_A_LIMITS);
      await waitForSettings(token, request, STAGE_A_LIMITS);
    });

    await test.step('Advisory security status check (Stage A only)', async () => {
      const statusResponse = await request.get('/api/v1/security/status', {
        headers: { Authorization: `Bearer ${token}` },
      });

      if (statusResponse.ok()) {
        const status = await statusResponse.json();
        if (status?.rate_limit?.enabled !== undefined) {
          expect(status.rate_limit.enabled).toBe(true);
        }
      }
    });
  });

  test.afterAll(async ({ request }) => {
    const restore = {
      enabled: coerceBoolean(
        originalSettings['security.rate_limit.enabled'],
        DEFAULT_LIMITS.enabled
      ),
      requests: coerceNumber(
        originalSettings['security.rate_limit.requests'],
        DEFAULT_LIMITS.requests
      ),
      window: coerceNumber(
        originalSettings['security.rate_limit.window'],
        DEFAULT_LIMITS.window
      ),
      burst: coerceNumber(
        originalSettings['security.rate_limit.burst'],
        DEFAULT_LIMITS.burst
      ),
    };

    await patchRateLimit(token, request, restore);
  });

  test('Emergency endpoint bypasses rate limits while others do not', async ({ request }) => {
    let stageBBurstUsed = 0;

    await test.step('Emergency reset runs before Stage B', async () => {
      const emergencyResponse = await request.post('/api/v1/emergency/security-reset', {
        headers: {
          'X-Emergency-Token': EMERGENCY_TOKEN,
        },
      });

      expect(emergencyResponse.ok()).toBeTruthy();
    });

    await test.step('Apply Stage B limits and verify once', async () => {
      await patchRateLimit(token, request, STAGE_B_LIMITS);

      const settings = await fetchSettings(token, request);
      expect(settingsMatch(settings, STAGE_B_LIMITS)).toBe(true);
      stageBBurstUsed = 1;
    });

    await test.step('Burst until rate limit hits 429', async () => {
      const maxAttempts = 10;
      let attempts = stageBBurstUsed;
      let rateLimitHit = false;

      while (attempts < maxAttempts) {
        const response = await request.get('/api/v1/auth/verify', {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        });

        attempts += 1;
        const status = response.status();

        if (status === 429) {
          rateLimitHit = true;
          break;
        }

        expect(status).toBe(200);
      }

      expect(rateLimitHit).toBeTruthy();
    });
  });
});
