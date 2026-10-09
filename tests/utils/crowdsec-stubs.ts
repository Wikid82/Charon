/**
 * Shared CrowdSec API stubs for deterministic Playwright specs.
 *
 * The E2E container has no cscli/LAPI, so CrowdSec specs stub the admin API with
 * `page.route`. Every route matcher here is an ANCHORED regular expression. A loose
 * glob such as `** /crowdsec/file**` also matches `/crowdsec/files`, which once made a
 * file-content stub swallow the file-list request.
 *
 * Install stubs BEFORE navigating to the page under test.
 */

import type { Page, Route } from '@playwright/test';

const ADMIN = String.raw`/api/v1/admin/crowdsec`;
/** Optional query string, then end of URL. */
const END = String.raw`(?:\?.*)?$`;

/** Anchored matchers for each CrowdSec admin endpoint. */
export const CROWDSEC_ROUTES = {
  featureFlags: new RegExp(String.raw`/api/v1/feature-flags${END}`),
  status: new RegExp(`${ADMIN}/status${END}`),
  decisions: new RegExp(`${ADMIN}/decisions${END}`),
  consoleStatus: new RegExp(`${ADMIN}/console/status${END}`),
  consoleEnroll: new RegExp(`${ADMIN}/console/enroll${END}`),
  consoleEnrollment: new RegExp(`${ADMIN}/console/enrollment${END}`),
  files: new RegExp(`${ADMIN}/files${END}`),
  file: new RegExp(`${ADMIN}/file${END}`),
  export: new RegExp(`${ADMIN}/export${END}`),
  diagnosticsConfig: new RegExp(`${ADMIN}/diagnostics/config${END}`),
  diagnosticsConnectivity: new RegExp(`${ADMIN}/diagnostics/connectivity${END}`),
} as const;

export interface CrowdSecProcessStatus {
  running: boolean;
  pid: number;
  lapi_ready: boolean;
}

export interface ConsoleEnrollmentState {
  status: string;
  key_present: boolean;
  tenant?: string;
  agent_name?: string;
  last_error?: string;
  last_attempt_at?: string;
  enrolled_at?: string;
  last_heartbeat_at?: string;
  correlation_id?: string;
}

export interface CrowdSecDecisionFixture {
  id: string;
  ip: string;
  reason: string;
  duration: string;
  created_at: string;
  source: string;
}

/** Fixtures with sensible defaults; override per test. */
export const crowdsecFixtures = {
  runningStatus: (overrides: Partial<CrowdSecProcessStatus> = {}): CrowdSecProcessStatus => ({
    running: true,
    pid: 1234,
    lapi_ready: true,
    ...overrides,
  }),
  notEnrolled: (): ConsoleEnrollmentState => ({ status: 'not_enrolled', key_present: false }),
  enrollment: (overrides: Partial<ConsoleEnrollmentState> = {}): ConsoleEnrollmentState => ({
    status: 'enrolled',
    key_present: true,
    agent_name: 'e2e-agent',
    tenant: 'e2e-tenant',
    ...overrides,
  }),
  decision: (overrides: Partial<CrowdSecDecisionFixture> = {}): CrowdSecDecisionFixture => ({
    id: 'decision-1',
    ip: '203.0.113.7',
    reason: 'e2e brute force',
    duration: '24h',
    created_at: '2026-01-01T00:00:00Z',
    source: 'manual',
    ...overrides,
  }),
  diagnosticsConfig: (overrides: Record<string, unknown> = {}): Record<string, unknown> => ({
    config_exists: true,
    config_valid: true,
    acquis_exists: true,
    lapi_port: '8085',
    errors: [],
    ...overrides,
  }),
  diagnosticsConnectivity: (overrides: Record<string, unknown> = {}): Record<string, unknown> => ({
    lapi_running: true,
    lapi_ready: true,
    capi_registered: false,
    console_enrolled: false,
    ...overrides,
  }),
};

export interface CrowdSecStubOptions {
  /** Enables the console enrollment feature flag. Default false. */
  consoleEnrollmentEnabled?: boolean;
  /** Body for GET /status. */
  status?: CrowdSecProcessStatus;
  /** Body for GET /console/status. */
  consoleStatus?: ConsoleEnrollmentState;
  /** Body for GET /decisions. */
  decisions?: CrowdSecDecisionFixture[];
  /** File names returned by GET /files. */
  files?: string[];
  /** Map of file path to content for GET /file?path=. Unknown paths get 404. */
  fileContents?: Record<string, string>;
  /** Archive bytes returned by GET /export. */
  exportBody?: Buffer;
  diagnosticsConfig?: Record<string, unknown>;
  diagnosticsConnectivity?: Record<string, unknown>;
}

/** Requests recorded by the stubs, for assertions on what the UI actually sent. */
export interface CrowdSecStubRecorder {
  fileReads: string[];
  exportRequests: number;
}

async function fulfillJson(route: Route, json: unknown, status = 200): Promise<void> {
  await route.fulfill({ status, contentType: 'application/json', json });
}

/**
 * Stubs the CrowdSec admin API endpoints for the options provided. Endpoints whose
 * option is omitted are left untouched (they hit the real backend).
 * Only GET requests are stubbed unless noted; other methods fall through.
 */
export async function stubCrowdSecApi(
  page: Page,
  options: CrowdSecStubOptions = {},
): Promise<CrowdSecStubRecorder> {
  const recorder: CrowdSecStubRecorder = { fileReads: [], exportRequests: 0 };

  if (options.consoleEnrollmentEnabled !== undefined) {
    const enabled = options.consoleEnrollmentEnabled;
    await page.route(CROWDSEC_ROUTES.featureFlags, async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      const response = await route.fetch();
      const flags = (await response.json()) as Record<string, boolean>;
      await fulfillJson(route, { ...flags, 'feature.crowdsec.console_enrollment': enabled });
    });
  }

  if (options.status) {
    const body = options.status;
    await page.route(CROWDSEC_ROUTES.status, (route) =>
      route.request().method() === 'GET' ? fulfillJson(route, body) : route.fallback(),
    );
  }

  if (options.consoleStatus) {
    const body = options.consoleStatus;
    await page.route(CROWDSEC_ROUTES.consoleStatus, (route) =>
      route.request().method() === 'GET' ? fulfillJson(route, body) : route.fallback(),
    );
  }

  if (options.decisions) {
    const decisions = options.decisions;
    await page.route(CROWDSEC_ROUTES.decisions, (route) =>
      route.request().method() === 'GET' ? fulfillJson(route, { decisions }) : route.fallback(),
    );
  }

  if (options.files) {
    const files = options.files;
    await page.route(CROWDSEC_ROUTES.files, (route) =>
      route.request().method() === 'GET' ? fulfillJson(route, { files }) : route.fallback(),
    );
  }

  if (options.fileContents) {
    const contents = options.fileContents;
    await page.route(CROWDSEC_ROUTES.file, async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      const path = new URL(route.request().url()).searchParams.get('path') ?? '';
      recorder.fileReads.push(path);
      if (!(path in contents)) return fulfillJson(route, { error: 'file not found' }, 404);
      return fulfillJson(route, { content: contents[path] });
    });
  }

  if (options.exportBody) {
    const body = options.exportBody;
    await page.route(CROWDSEC_ROUTES.export, async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      recorder.exportRequests += 1;
      await route.fulfill({ status: 200, contentType: 'application/gzip', body });
    });
  }

  if (options.diagnosticsConfig) {
    const body = options.diagnosticsConfig;
    await page.route(CROWDSEC_ROUTES.diagnosticsConfig, (route) => fulfillJson(route, body));
  }

  if (options.diagnosticsConnectivity) {
    const body = options.diagnosticsConnectivity;
    await page.route(CROWDSEC_ROUTES.diagnosticsConnectivity, (route) => fulfillJson(route, body));
  }

  return recorder;
}
