import { test as setup, expect, request as playwrightRequest } from '@playwright/test';

const SECURITY_RESET_PROPAGATION_MS = 750;

function getBaseURL(baseURL?: string): string {
  return baseURL || process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
}

function getEmergencyServerURL(baseURL: string): string {
  const parsed = new URL(baseURL);
  parsed.port = process.env.EMERGENCY_SERVER_PORT || '2020';
  return parsed.toString().replace(/\/$/, '');
}

function validateEmergencyTokenForSecurityShard(): string {
  const token = process.env.CHARON_EMERGENCY_TOKEN;
  if (!token) {
    throw new Error('CHARON_EMERGENCY_TOKEN is required for security shard setup');
  }

  if (token.length < 64) {
    throw new Error(`CHARON_EMERGENCY_TOKEN must be at least 64 characters (got ${token.length})`);
  }

  if (!/^[a-f0-9]+$/i.test(token)) {
    throw new Error('CHARON_EMERGENCY_TOKEN must be hexadecimal');
  }

  return token;
}

async function emergencySecurityReset(baseURL: string, emergencyToken: string): Promise<void> {
  const emergencyBaseURL = getEmergencyServerURL(baseURL);
  const emergencyContext = await playwrightRequest.newContext({
    baseURL: emergencyBaseURL,
    httpCredentials: {
      username: process.env.CHARON_EMERGENCY_USERNAME || 'admin',
      password: process.env.CHARON_EMERGENCY_PASSWORD || 'changeme',
    },
  });

  try {
    const response = await emergencyContext.post('/emergency/security-reset', {
      headers: {
        'X-Emergency-Token': emergencyToken,
        'Content-Type': 'application/json',
      },
      data: { reason: 'Security shard setup baseline reset' },
      timeout: 8000,
    });

    const body = await response.text();
    expect(response.ok(), `Security shard emergency reset failed: ${response.status()} ${body}`).toBeTruthy();
  } finally {
    await emergencyContext.dispose();
  }
}

async function verifySecurityDisabled(baseURL: string, emergencyToken: string): Promise<void> {
  const statusContext = await playwrightRequest.newContext({
    baseURL,
    extraHTTPHeaders: {
      'X-Emergency-Token': emergencyToken,
    },
  });

  try {
    const response = await statusContext.get('/api/v1/security/status', { timeout: 5000 });
    expect(response.ok()).toBeTruthy();

    const status = await response.json();
    expect(status.acl?.enabled).toBeFalsy();
    expect(status.waf?.enabled).toBeFalsy();
    expect(status.rate_limit?.enabled).toBeFalsy();
  } finally {
    await statusContext.dispose();
  }
}

setup('prepare-security-shard-baseline', async ({ baseURL }) => {
  const resolvedBaseURL = getBaseURL(baseURL);
  const emergencyToken = validateEmergencyTokenForSecurityShard();

  await emergencySecurityReset(resolvedBaseURL, emergencyToken);
  await new Promise((resolve) => setTimeout(resolve, SECURITY_RESET_PROPAGATION_MS));
  await verifySecurityDisabled(resolvedBaseURL, emergencyToken);
});
