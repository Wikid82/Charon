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
  securityStatus: new RegExp(String.raw`/api/v1/security/status${END}`),
  dashboardSummary: new RegExp(`${ADMIN}/dashboard/summary${END}`),
  dashboardTimeline: new RegExp(`${ADMIN}/dashboard/timeline${END}`),
  dashboardTopIps: new RegExp(`${ADMIN}/dashboard/top-ips${END}`),
  dashboardScenarios: new RegExp(`${ADMIN}/dashboard/scenarios${END}`),
  alerts: new RegExp(`${ADMIN}/alerts${END}`),
  decisionsExport: new RegExp(`${ADMIN}/decisions/export${END}`),
  ban: new RegExp(`${ADMIN}/ban${END}`),
  unban: new RegExp(`${ADMIN}/ban/[^/?]+${END}`),
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

export interface DashboardSummaryFixture {
  total_decisions: number;
  active_decisions: number;
  unique_ips: number;
  top_scenario: string;
  decisions_trend: number;
  range: string;
  cached: boolean;
  generated_at: string;
}

export interface DashboardAlertFixture {
  id: number;
  scenario: string;
  ip: string;
  message: string;
  events_count: number;
  start_at: string;
  stop_at: string;
  created_at: string;
  duration: string;
  type: string;
  origin: string;
}

/** Makes a stubbed endpoint answer with an error status instead of data. */
export interface StubFailure {
  failWith: number;
}

function isFailure(value: unknown): value is StubFailure {
  return typeof value === 'object' && value !== null && 'failWith' in value;
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
  dashboardSummary: (overrides: Partial<DashboardSummaryFixture> = {}): DashboardSummaryFixture => ({
    total_decisions: 1234,
    active_decisions: 56,
    unique_ips: 78,
    top_scenario: 'crowdsecurity/http-probing',
    decisions_trend: 12.5,
    range: '24h',
    cached: false,
    generated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }),
  alert: (overrides: Partial<DashboardAlertFixture> = {}): DashboardAlertFixture => ({
    id: 1,
    scenario: 'crowdsecurity/ssh-bf',
    ip: '198.51.100.10',
    message: 'ssh brute force',
    events_count: 7,
    start_at: '2026-01-01T00:00:00Z',
    stop_at: '2026-01-01T00:05:00Z',
    created_at: '2026-01-01T00:05:00Z',
    duration: '4h',
    type: 'ban',
    origin: 'crowdsec',
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
  decisions?: CrowdSecDecisionFixture[] | StubFailure;
  /**
   * Overrides `crowdsec.mode` in the real GET /security/status response, which gates the
   * banned IPs and whitelist sections. Other fields stay as the backend reports them.
   */
  crowdsecMode?: 'local' | 'disabled';
  /** Dashboard endpoint bodies; omitted entries hit the real backend. */
  dashboard?: {
    summary?: DashboardSummaryFixture | StubFailure;
    timeline?: { buckets: unknown[]; range: string; interval: string; cached: boolean } | StubFailure;
    topIps?: { ips: unknown[]; range: string; cached: boolean } | StubFailure;
    scenarios?: { scenarios: unknown[]; total: number; range: string; cached: boolean } | StubFailure;
    /** Receives the query params of each request and returns the alerts page to serve. */
    alerts?: ((query: URLSearchParams) => { alerts: DashboardAlertFixture[]; total: number; source: string; cached: boolean }) | StubFailure;
  };
  /** Body for GET /decisions/export (any format); a failure status is also accepted. */
  decisionsExport?: { body: string; contentType: string } | StubFailure;
  /** Stubs POST /ban and DELETE /ban/:ip, recording what the UI sent. */
  banApi?: { banFailure?: StubFailure; unbanFailure?: StubFailure };
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
  /** Number of GET /decisions requests. */
  decisionsRequests: number;
  /** Query params of each GET /decisions/export request. */
  decisionsExportQueries: Record<string, string>[];
  /** Query params of each dashboard request, keyed by endpoint (summary, timeline, top-ips, scenarios). */
  dashboardQueries: Record<string, Record<string, string>[]>;
  /** Query params of each GET /alerts request. */
  alertQueries: Record<string, string>[];
  /** JSON bodies of POST /ban requests. */
  bans: unknown[];
  /** IPs of DELETE /ban/:ip requests (decoded). */
  unbans: string[];
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
  const recorder: CrowdSecStubRecorder = {
    fileReads: [],
    exportRequests: 0,
    decisionsRequests: 0,
    decisionsExportQueries: [],
    dashboardQueries: {},
    alertQueries: [],
    bans: [],
    unbans: [],
  };

  if (options.crowdsecMode !== undefined) {
    const mode = options.crowdsecMode;
    await page.route(CROWDSEC_ROUTES.securityStatus, async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      const response = await route.fetch();
      const status = (await response.json()) as { crowdsec?: Record<string, unknown> };
      await fulfillJson(route, { ...status, crowdsec: { ...status.crowdsec, mode, enabled: mode === 'local' } });
    });
  }

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
    await page.route(CROWDSEC_ROUTES.decisions, (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      recorder.decisionsRequests += 1;
      return isFailure(decisions)
        ? fulfillJson(route, { error: 'stubbed failure' }, decisions.failWith)
        : fulfillJson(route, { decisions });
    });
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

  const dashboardRoutes = [
    ['summary', CROWDSEC_ROUTES.dashboardSummary, options.dashboard?.summary],
    ['timeline', CROWDSEC_ROUTES.dashboardTimeline, options.dashboard?.timeline],
    ['top-ips', CROWDSEC_ROUTES.dashboardTopIps, options.dashboard?.topIps],
    ['scenarios', CROWDSEC_ROUTES.dashboardScenarios, options.dashboard?.scenarios],
  ] as const;
  for (const [name, matcher, body] of dashboardRoutes) {
    if (!body) continue;
    recorder.dashboardQueries[name] = [];
    await page.route(matcher, (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      recorder.dashboardQueries[name].push(Object.fromEntries(new URL(route.request().url()).searchParams));
      return isFailure(body)
        ? fulfillJson(route, { error: 'stubbed failure' }, body.failWith)
        : fulfillJson(route, body);
    });
  }

  if (options.dashboard?.alerts) {
    const alerts = options.dashboard.alerts;
    await page.route(CROWDSEC_ROUTES.alerts, (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      const query = new URL(route.request().url()).searchParams;
      recorder.alertQueries.push(Object.fromEntries(query));
      return isFailure(alerts)
        ? fulfillJson(route, { error: 'stubbed failure' }, alerts.failWith)
        : fulfillJson(route, alerts(query));
    });
  }

  if (options.decisionsExport) {
    const exported = options.decisionsExport;
    await page.route(CROWDSEC_ROUTES.decisionsExport, async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      recorder.decisionsExportQueries.push(Object.fromEntries(new URL(route.request().url()).searchParams));
      if (isFailure(exported)) return fulfillJson(route, { error: 'stubbed failure' }, exported.failWith);
      return route.fulfill({ status: 200, contentType: exported.contentType, body: exported.body });
    });
  }

  if (options.banApi) {
    const { banFailure, unbanFailure } = options.banApi;
    await page.route(CROWDSEC_ROUTES.ban, (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      recorder.bans.push(route.request().postDataJSON());
      return banFailure
        ? fulfillJson(route, { error: 'ban rejected' }, banFailure.failWith)
        : fulfillJson(route, { status: 'banned' });
    });
    await page.route(CROWDSEC_ROUTES.unban, (route) => {
      if (route.request().method() !== 'DELETE') return route.fallback();
      const ip = new URL(route.request().url()).pathname.split('/').pop() ?? '';
      recorder.unbans.push(decodeURIComponent(ip));
      return unbanFailure
        ? fulfillJson(route, { error: 'unban rejected' }, unbanFailure.failWith)
        : fulfillJson(route, { status: 'unbanned' });
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
