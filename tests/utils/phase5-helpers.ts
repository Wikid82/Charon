/**
 * Phase 5: Tasks & Monitoring - Test Helper Functions
 *
 * Provides mock data setup, API mocking utilities, and test helpers
 * for backup, logs, import, and monitoring E2E tests.
 */

import { Page } from '@playwright/test';
import { waitForAPIResponse, waitForWebSocketConnection } from './wait-helpers';

// ============================================================================
// Type Definitions
// ============================================================================

export interface BackupFile {
  filename: string;
  size: number;
  time: string;
  // Optional fields added by the Issue #32 backup v2 format (see docs/plans/current_spec.md §3.3.1).
  // Kept optional so existing mocks/tests that only set filename/size/time keep passing unmodified.
  uuid?: string;
  type?: 'manual' | 'scheduled' | 'pre_restore' | 'uploaded';
  encrypted?: boolean;
  format_version?: number;
  status?: string;
  remote_copies?: Array<{
    target_uuid: string;
    target_name: string;
    status: string;
    uploaded_at?: string;
  }>;
}

export interface LogFile {
  name: string;
  size: number;
  modified: string;
}

export interface CaddyAccessLog {
  level: string;
  ts: number;
  logger: string;
  msg: string;
  request: {
    remote_ip: string;
    method: string;
    host: string;
    uri: string;
    proto: string;
  };
  status: number;
  duration: number;
  size: number;
}

export interface LogResponse {
  entries: CaddyAccessLog[];
  total: number;
  page: number;
  limit: number;
}

export interface UptimeMonitor {
  id: string;
  upstream_host?: string;
  proxy_host_id?: number;
  remote_server_id?: number;
  name: string;
  type: string;
  url: string;
  interval: number;
  enabled: boolean;
  status: string;
  last_check?: string | null;
  latency: number;
  max_retries: number;
}

export interface UptimeHeartbeat {
  id: number;
  monitor_id: string;
  status: string;
  latency: number;
  message: string;
  created_at: string;
}

export interface ImportSession {
  id: string;
  state: 'pending' | 'reviewing' | 'completed' | 'failed' | 'transient';
  created_at: string;
  updated_at: string;
  source_file?: string;
}

export interface ImportPreview {
  session: ImportSession;
  preview: {
    hosts: Array<{ domain_names: string; [key: string]: unknown }>;
    conflicts: string[];
    errors: string[];
  };
  caddyfile_content?: string;
  conflict_details?: Record<string, {
    existing: {
      forward_scheme: string;
      forward_host: string;
      forward_port: number;
      ssl_forced: boolean;
      websocket: boolean;
      enabled: boolean;
    };
    imported: {
      forward_scheme: string;
      forward_host: string;
      forward_port: number;
      ssl_forced: boolean;
      websocket: boolean;
    };
  }>;
}

export interface LiveLogEntry {
  level: string;
  timestamp: string;
  message: string;
  source?: string;
  data?: Record<string, unknown>;
}

export interface SecurityLogEntry {
  timestamp: string;
  level: string;
  logger: string;
  client_ip: string;
  method: string;
  uri: string;
  status: number;
  duration: number;
  size: number;
  user_agent: string;
  host: string;
  source: 'waf' | 'crowdsec' | 'ratelimit' | 'acl' | 'normal';
  blocked: boolean;
  block_reason?: string;
  details?: Record<string, unknown>;
}

// ============================================================================
// Backup Helpers
// ============================================================================

/**
 * Sets up mock backup list for testing
 */
export async function setupBackupsList(page: Page, backups?: BackupFile[]): Promise<void> {
  const defaultBackups: BackupFile[] = backups || [
    { filename: 'backup_2024-01-15_120000.zip', size: 1048576, time: '2024-01-15T12:00:00Z' },
    { filename: 'backup_2024-01-14_120000.zip', size: 2097152, time: '2024-01-14T12:00:00Z' },
  ];

  await page.route('**/api/v1/backups', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, json: defaultBackups });
    } else {
      await route.continue();
    }
  });
}

/**
 * Completes a full backup restore flow for testing post-restore behavior
 */
export async function completeRestoreFlow(page: Page, filename?: string): Promise<void> {
  const targetFilename = filename || 'backup_2024-01-15_120000.zip';

  await page.route(`**/api/v1/backups/${targetFilename}/restore`, (route) => {
    route.fulfill({ status: 200, json: { message: 'Restore completed successfully' } });
  });

  await page.goto('/tasks/backups');

  // Click restore button
  await page.locator('button:has-text("Restore")').first().click();

  // Fill confirmation input
  const confirmInput = page.locator('input[placeholder*="backup name"]');
  if (await confirmInput.isVisible()) {
    await confirmInput.fill('backup_2024-01-15');
  }

  // Confirm restore
  await page.locator('[role="dialog"] button:has-text("Restore")').click();
  await waitForAPIResponse(page, `/api/v1/backups/${targetFilename}/restore`, 200);
}

// ============================================================================
// Remote Storage Target — OAuth Helpers (Issue #32 Phase 2: WebDAV / Dropbox /
// Google Drive)
//
// See docs/plans/current_spec.md §3.2 (config/secrets shapes), §3.3 (OAuth API
// contracts), §3.6 (frontend two-step create->connect lifecycle), §3.9 (error
// paths). Consumed by tests/tasks/backups-remote-targets.spec.ts.
//
// Written in Commit 1 to back `test.fixme` specs — no backend/frontend code
// implementing these routes exists yet (lands in Commits 2-4). Flipped to live
// assertions in Commit 5.
// ============================================================================

/** Mirrors backend `WebDAVConfig` (spec §3.2) — no OAuth, single-step save. */
export interface WebDAVTargetConfig {
  url: string;
  username?: string;
  base_path?: string;
  insecure_skip_verify?: boolean;
}

/** Mirrors backend `DropboxConfig` (spec §3.2) — `app_key` is not secret. */
export interface DropboxTargetConfig {
  app_key: string;
  folder_path?: string;
}

/** Mirrors backend `GoogleDriveConfig` (spec §3.2) — `client_id` is not secret. */
export interface GoogleDriveTargetConfig {
  client_id: string;
  folder_path?: string;
}

/** Mirrors the new `RemoteTarget.oauth_status` enum (spec §3.4) — `''` for s3/sftp/webdav. */
export type RemoteTargetOAuthStatus = 'not_connected' | 'connected' | 'revoked' | '';

/** Fields every `RemoteTarget` API response gains regardless of provider type (spec §3.4). */
export interface RemoteTargetOAuthFields {
  oauth_status: RemoteTargetOAuthStatus;
  oauth_connected_at: string | null;
}

/** `error_code` values the OAuth surface can return (spec §3.3, §3.9). */
export type RemoteTargetOAuthErrorCode =
  | 'oauth_not_connected'
  | 'oauth_revoked'
  | 'public_url_not_configured';

/**
 * Mocks `POST /api/v1/backups/remote-targets/:uuid/oauth/start` — the
 * "Save & Connect" step that returns the provider's `authorize_url` (spec §3.3).
 */
export async function setupRemoteTargetOAuthStart(
  page: Page,
  targetUuid: string,
  response: { authorize_url: string },
  status: number = 200
): Promise<void> {
  await page.route(`**/api/v1/backups/remote-targets/${targetUuid}/oauth/start`, async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ status, json: response });
    } else {
      await route.continue();
    }
  });
}

/**
 * Mocks a failed `oauth/start` call — e.g. `400 public_url_not_configured` when
 * the admin hasn't set `app.public_url` yet (spec §3.3 R10, §3.9).
 */
export async function setupRemoteTargetOAuthStartError(
  page: Page,
  targetUuid: string,
  errorCode: RemoteTargetOAuthErrorCode,
  message: string,
  status: number = 400
): Promise<void> {
  await page.route(`**/api/v1/backups/remote-targets/${targetUuid}/oauth/start`, async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ status, json: { error: message, error_code: errorCode } });
    } else {
      await route.continue();
    }
  });
}

/** Mocks `POST /api/v1/backups/remote-targets/:uuid/oauth/disconnect` (spec §3.3). */
export async function setupRemoteTargetOAuthDisconnect(page: Page, targetUuid: string): Promise<void> {
  await page.route(`**/api/v1/backups/remote-targets/${targetUuid}/oauth/disconnect`, async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({ status: 200, json: { success: true } });
    } else {
      await route.continue();
    }
  });
}

/**
 * Mocks `POST /api/v1/backups/remote-targets/:uuid/test` returning one of the
 * OAuth-specific `409` sentinel errors (spec §3.3, §3.9) — surfaced through the
 * existing generic `toast.error(error.message)` path already used by
 * `ErrEncryptionKeyMissing` today, so no new UI concept is required.
 */
export async function setupRemoteTargetOAuthTestError(
  page: Page,
  targetUuid: string,
  errorCode: Extract<RemoteTargetOAuthErrorCode, 'oauth_not_connected' | 'oauth_revoked'>,
  message: string
): Promise<void> {
  await page.route(`**/api/v1/backups/remote-targets/${targetUuid}/test`, async (route) => {
    await route.fulfill({ status: 409, json: { error: message, error_code: errorCode } });
  });
}

/**
 * Real provider consent-screen hosts are not reachable in CI (spec §4 Phase 1,
 * §6 Commit 1) — glob patterns used to intercept the full-page redirect instead.
 */
export const OAUTH_PROVIDER_AUTHORIZE_HOST_PATTERNS: Record<'dropbox' | 'google_drive', string> = {
  dropbox: 'https://www.dropbox.com/oauth2/authorize**',
  google_drive: 'https://accounts.google.com/o/oauth2/v2/auth**',
};

export interface MockOAuthRoundTripOptions {
  provider: 'dropbox' | 'google_drive';
  /** UUID of the already-saved (not-yet-connected) target being connected. */
  targetUuid: string;
  /** Whether the mocked consent screen "approves" or "denies" the request. */
  outcome: 'approved' | 'denied';
  /** Only used when outcome === 'denied' — becomes the callback's `message` query param. */
  denyMessage?: string;
}

/**
 * Simulates the full OAuth round trip a real browser makes for the Dropbox/
 * Google Drive Connect flow: full-page redirect to the provider's
 * `authorize_url` -> (mocked) consent screen -> Charon's
 * `oauth/:provider/callback` -> final redirect back into the app with
 * `oauth_result` on the query string (spec §3.3, §3.7).
 *
 * Both hops are intercepted via Playwright route interception rather than
 * driven for real, since real Dropbox/Google consent screens aren't reachable
 * in CI (spec §6 Commit 1).
 */
export async function mockOAuthProviderRoundTrip(page: Page, options: MockOAuthRoundTripOptions): Promise<void> {
  const { provider, targetUuid, outcome, denyMessage } = options;

  // playwright.config.js guarantees process.env.PLAYWRIGHT_BASE_URL is
  // always set (it back-fills its own default), so this is safe to read
  // directly rather than threading the base URL through every call site.
  const appBaseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
  const callbackUrl = `${appBaseURL}/api/v1/backups/remote-targets/oauth/${provider}/callback?code=mock-auth-code&state=mock-state`;
  const finalRedirectUrl =
    outcome === 'approved'
      ? `/backups?oauth_result=success&provider=${provider}&target=${targetUuid}`
      : `/backups?oauth_result=error&message=${encodeURIComponent(denyMessage ?? 'authorization_denied')}`;

  // Hop 1: the provider's consent screen, standing in for a real user
  // clicking "Allow"/"Deny". Fulfilled as an HTML document that immediately
  // navigates onward via a client-side script rather than an HTTP 3xx
  // redirect: browser engines' same-navigation redirect-follow for a
  // *cross-origin* Location target does not reliably re-enter Playwright's
  // request interception in this harness (verified empirically — the
  // follow-up request bypasses every registered page/context route and
  // hits the real backend directly), whereas a JS-driven
  // `window.location.href` navigation — exactly how the app itself got here
  // (spec §3.6's full-page-redirect design) — is reliably intercepted.
  await page.route(OAUTH_PROVIDER_AUTHORIZE_HOST_PATTERNS[provider], async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'text/html',
      body: `<script>window.location.href = ${JSON.stringify(callbackUrl)};</script>`,
    });
  });

  // Hop 2: Charon's own callback route, which (once implemented) validates
  // `state`, exchanges `code` for tokens, and 302s back into the app (spec §3.3).
  await page.route(`**/api/v1/backups/remote-targets/oauth/${provider}/callback**`, async (route) => {
    await route.fulfill({ status: 302, headers: { location: finalRedirectUrl } });
  });
}

/** Builds a mock WebDAV `RemoteTarget` fixture (no OAuth — `oauth_status: ''`, spec §3.4). */
export function buildWebDAVTargetFixture(
  overrides: Partial<{ uuid: string; name: string; enabled: boolean; config: WebDAVTargetConfig }> = {}
) {
  return {
    uuid: overrides.uuid ?? 'wd1',
    name: overrides.name ?? 'Nextcloud',
    type: 'webdav' as const,
    enabled: overrides.enabled ?? true,
    config: {
      webdav: overrides.config ?? {
        url: 'https://nas.example.com/remote.php/dav/files/charon/',
        username: 'charon',
        base_path: '/charon-backups',
        insecure_skip_verify: false,
      },
    },
    secrets_set: true,
    oauth_status: '' as RemoteTargetOAuthStatus,
    oauth_connected_at: null,
    last_test_at: null,
    last_test_status: 'never' as const,
    last_error: '',
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-01T00:00:00Z',
  };
}

/** Builds a mock Dropbox `RemoteTarget` fixture in a given OAuth lifecycle state (spec §3.3). */
export function buildDropboxTargetFixture(
  overrides: Partial<{
    uuid: string;
    name: string;
    enabled: boolean;
    config: DropboxTargetConfig;
    oauth_status: RemoteTargetOAuthStatus;
    oauth_connected_at: string | null;
  }> = {}
) {
  return {
    uuid: overrides.uuid ?? 'db1',
    name: overrides.name ?? 'Dropbox',
    type: 'dropbox' as const,
    enabled: overrides.enabled ?? true,
    config: {
      dropbox: overrides.config ?? { app_key: 'abc123', folder_path: '/charon-backups' },
    },
    secrets_set: true,
    oauth_status: overrides.oauth_status ?? ('not_connected' as RemoteTargetOAuthStatus),
    oauth_connected_at: overrides.oauth_connected_at ?? null,
    last_test_at: null,
    last_test_status: 'never' as const,
    last_error: '',
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-01T00:00:00Z',
  };
}

/** Builds a mock Google Drive `RemoteTarget` fixture in a given OAuth lifecycle state (spec §3.3). */
export function buildGoogleDriveTargetFixture(
  overrides: Partial<{
    uuid: string;
    name: string;
    enabled: boolean;
    config: GoogleDriveTargetConfig;
    oauth_status: RemoteTargetOAuthStatus;
    oauth_connected_at: string | null;
  }> = {}
) {
  return {
    uuid: overrides.uuid ?? 'gd1',
    name: overrides.name ?? 'Google Drive',
    type: 'google_drive' as const,
    enabled: overrides.enabled ?? true,
    config: {
      google_drive: overrides.config ?? {
        client_id: 'client-abc123.apps.googleusercontent.com',
        folder_path: 'Charon/Backups',
      },
    },
    secrets_set: true,
    oauth_status: overrides.oauth_status ?? ('not_connected' as RemoteTargetOAuthStatus),
    oauth_connected_at: overrides.oauth_connected_at ?? null,
    last_test_at: null,
    last_test_status: 'never' as const,
    last_error: '',
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-01T00:00:00Z',
  };
}

// ============================================================================
// Log Helpers
// ============================================================================

/**
 * Sets up mock log files list for testing
 */
export async function setupLogFiles(page: Page, files?: LogFile[]): Promise<void> {
  const defaultFiles: LogFile[] = files || [
    { name: 'access.log', size: 1048576, modified: '2024-01-15T12:00:00Z' },
    { name: 'error.log', size: 256000, modified: '2024-01-15T11:30:00Z' },
  ];

  await page.route('**/api/v1/logs', (route) => {
    route.fulfill({ status: 200, json: defaultFiles });
  });
}

/**
 * Selects a log file and waits for content to load
 */
export async function selectLogFile(page: Page, filename: string): Promise<void> {
  await page.click(`button:has-text("${filename}")`);
  await waitForAPIResponse(page, `/api/v1/logs/${filename}`, 200);
}

/**
 * Generates mock log entries for pagination testing
 */
export function generateMockEntries(count: number, pageNum: number): CaddyAccessLog[] {
  return Array.from({ length: count }, (_, i) => ({
    level: 'info',
    ts: Date.now() / 1000 - (pageNum * count + i) * 60,
    logger: 'http.log.access',
    msg: 'handled request',
    request: {
      remote_ip: `192.168.1.${i % 255}`,
      method: 'GET',
      host: 'example.com',
      uri: `/page/${pageNum * count + i}`,
      proto: 'HTTP/2',
    },
    status: 200,
    duration: 0.05,
    size: 1234,
  }));
}

/**
 * Sets up mock log content for a specific file
 */
export async function setupLogContent(
  page: Page,
  filename: string,
  entries: CaddyAccessLog[],
  total?: number
): Promise<void> {
  await page.route(`**/api/v1/logs/${filename}*`, (route) => {
    const url = new URL(route.request().url());
    const requestedPage = parseInt(url.searchParams.get('page') || '1');
    const limit = parseInt(url.searchParams.get('limit') || '50');

    route.fulfill({
      status: 200,
      json: {
        entries: entries.slice((requestedPage - 1) * limit, requestedPage * limit),
        total: total || entries.length,
        page: requestedPage,
        limit,
      } as LogResponse,
    });
  });
}

// ============================================================================
// Import Helpers
// ============================================================================

/**
 * Sets up mock import API for Caddyfile testing
 */
export async function mockImportAPI(page: Page): Promise<void> {
  const mockPreview: ImportPreview = {
    session: {
      id: 'test-session',
      state: 'reviewing',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    preview: {
      hosts: [{ domain_names: 'example.com', forward_host: 'localhost', forward_port: 3000 }],
      conflicts: [],
      errors: [],
    },
  };

  await page.route('**/api/v1/import/upload', (route) => {
    route.fulfill({ status: 200, json: mockPreview });
  });

  await page.route('**/api/v1/import/preview', (route) => {
    route.fulfill({ status: 200, json: mockPreview });
  });

  await page.route('**/api/v1/import/status', (route) => {
    route.fulfill({ status: 200, json: { has_pending: true, session: mockPreview.session } });
  });

  await page.route('**/api/v1/import/commit', (route) => {
    route.fulfill({ status: 200, json: { created: 1, updated: 0, skipped: 0, errors: [] } });
  });
}

/**
 * Sets up mock import preview with specific hosts
 */
export async function mockImportPreview(page: Page, preview: ImportPreview): Promise<void> {
  await page.route('**/api/v1/import/upload', (route) => {
    route.fulfill({ status: 200, json: preview });
  });
  await page.route('**/api/v1/import/preview', (route) => {
    route.fulfill({ status: 200, json: preview });
  });
}

/**
 * Uploads a Caddyfile via the UI
 */
export async function uploadCaddyfile(page: Page, content: string): Promise<void> {
  await page.goto('/tasks/import/caddyfile');

  const fileInput = page.locator('input[type="file"]');
  await fileInput.setInputFiles({
    name: 'Caddyfile',
    mimeType: 'text/plain',
    buffer: Buffer.from(content),
  });

  await waitForAPIResponse(page, '/api/v1/import/upload', 200);
}

/**
 * Sets up import review state with specified number of hosts
 */
export async function setupImportReview(page: Page, hostCount: number): Promise<void> {
  const hosts = Array.from({ length: hostCount }, (_, i) => ({
    domain_names: `host${i + 1}.example.com`,
    forward_host: `server${i + 1}`,
    forward_port: 8080 + i,
  }));

  const preview: ImportPreview = {
    session: {
      id: 'test-session',
      state: 'reviewing',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    preview: { hosts, conflicts: [], errors: [] },
  };

  await mockImportPreview(page, preview);
  await page.goto('/tasks/import/caddyfile');

  // Trigger upload to get to review state
  const pasteArea = page.locator('textarea[placeholder*="Paste"]');
  if (await pasteArea.isVisible()) {
    await pasteArea.fill('# mock content');
    await page.click('button:has-text("Upload")');
    await waitForAPIResponse(page, '/api/v1/import/upload', 200);
  }
}

/**
 * Mocks CrowdSec import API
 */
export async function mockCrowdSecImportAPI(page: Page): Promise<void> {
  await page.route('**/api/v1/backups', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({
        status: 201,
        json: { filename: 'pre-import-backup.zip', size: 1000, time: new Date().toISOString() },
      });
    } else {
      await route.continue();
    }
  });

  await page.route('**/api/v1/crowdsec/import', (route) => {
    route.fulfill({ status: 200, json: { message: 'Import successful' } });
  });
}

/**
 * Uploads a CrowdSec config file via the UI
 */
export async function uploadCrowdSecConfig(page: Page): Promise<void> {
  const fileInput = page.locator('input[data-testid="crowdsec-import-file"]');
  await fileInput.setInputFiles({
    name: 'crowdsec-config.tar.gz',
    mimeType: 'application/gzip',
    buffer: Buffer.from('mock tar content'),
  });

  await page.click('button:has-text("Import")');
}

// ============================================================================
// Uptime Monitor Helpers
// ============================================================================

/**
 * Sets up mock monitors list for testing
 */
export async function setupMonitorsList(page: Page, monitors?: UptimeMonitor[]): Promise<void> {
  const defaultMonitors: UptimeMonitor[] = monitors || [
    {
      id: '1',
      name: 'API Server',
      type: 'http',
      url: 'https://api.example.com',
      interval: 60,
      enabled: true,
      status: 'up',
      latency: 45,
      max_retries: 3,
    },
    {
      id: '2',
      name: 'Database',
      type: 'tcp',
      url: 'tcp://db:5432',
      interval: 30,
      enabled: true,
      status: 'down',
      latency: 0,
      max_retries: 3,
    },
  ];

  await page.route('**/api/v1/uptime/monitors', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, json: defaultMonitors });
    } else {
      await route.continue();
    }
  });
}

/**
 * Sets up mock monitor history (heartbeats)
 */
export async function setupMonitorHistory(
  page: Page,
  monitorId: string,
  heartbeats: UptimeHeartbeat[]
): Promise<void> {
  await page.route(`**/api/v1/uptime/monitors/${monitorId}/history*`, (route) => {
    route.fulfill({ status: 200, json: heartbeats });
  });
}

/**
 * Sets up monitors with history data
 */
export async function mockMonitorsWithHistory(page: Page, history: UptimeHeartbeat[]): Promise<void> {
  await setupMonitorsList(page);
  await setupMonitorHistory(page, '1', history);
}

/**
 * Generates mock heartbeat history
 */
export function generateMockHeartbeats(count: number, monitorId: string): UptimeHeartbeat[] {
  return Array.from({ length: count }, (_, i) => ({
    id: i,
    monitor_id: monitorId,
    status: i % 5 === 0 ? 'down' : 'up',
    latency: Math.random() * 100,
    message: i % 5 === 0 ? 'Connection timeout' : 'OK',
    created_at: new Date(Date.now() - i * 60000).toISOString(),
  }));
}

// ============================================================================
// WebSocket / Real-time Log Helpers
// ============================================================================

/**
 * Sets up mock live logs with initial data
 */
export async function setupLiveLogsWithMockData(
  page: Page,
  entries: Partial<SecurityLogEntry>[]
): Promise<void> {
  const fullEntries: SecurityLogEntry[] = entries.map((entry, i) => ({
    timestamp: new Date().toISOString(),
    level: 'info',
    logger: 'http',
    client_ip: `192.168.1.${i + 1}`,
    method: 'GET',
    uri: '/test',
    status: 200,
    duration: 0.05,
    size: 1000,
    user_agent: 'Mozilla/5.0',
    host: 'example.com',
    source: 'normal',
    blocked: false,
    ...entry,
  }));

  // Store entries for later retrieval in tests
  await page.evaluate((data) => {
    (window as any).__mockLiveLogEntries = data;
  }, fullEntries);
}

/**
 * Sends a mock log entry via custom event (for WebSocket simulation)
 */
export async function sendMockLogEntry(page: Page, entry?: Partial<SecurityLogEntry>): Promise<void> {
  const fullEntry: SecurityLogEntry = {
    timestamp: new Date().toISOString(),
    level: 'info',
    logger: 'http',
    client_ip: '192.168.1.100',
    method: 'GET',
    uri: '/test',
    status: 200,
    duration: 0.05,
    size: 1000,
    user_agent: 'Mozilla/5.0',
    host: 'example.com',
    source: 'normal',
    blocked: false,
    ...entry,
  };

  await page.evaluate((data) => {
    window.dispatchEvent(new CustomEvent('mock-ws-message', { detail: data }));
  }, fullEntry);
}

/**
 * Simulates WebSocket network interruption for reconnection testing
 */
export async function simulateNetworkInterruption(page: Page, durationMs: number = 1000): Promise<void> {
  // Block WebSocket endpoints
  await page.route('**/api/v1/logs/live', (route) => route.abort());
  await page.route('**/api/v1/cerberus/logs/ws', (route) => route.abort());

  await page.waitForTimeout(durationMs);

  // Restore WebSocket endpoints
  await page.unroute('**/api/v1/logs/live');
  await page.unroute('**/api/v1/cerberus/logs/ws');
}

// ============================================================================
// Selector Constants
// ============================================================================

export const BACKUP_SELECTORS = {
  pageTitle: 'h1 >> text=Backups',
  createBackupButton: 'button:has-text("Create Backup")',
  backupTable: '[role="table"]',
  backupRows: '[role="row"]',
  emptyState: '[data-testid="empty-state"]',
  restoreButton: 'button:has-text("Restore")',
  deleteButton: 'button:has-text("Delete")',
  downloadButton: 'button:has([data-icon="download"])',
  confirmDialog: '[role="dialog"]',
  confirmButton: 'button:has-text("Confirm")',
  cancelButton: 'button:has-text("Cancel")',
} as const;

export const LOG_SELECTORS = {
  pageTitle: 'h1 >> text=Logs',
  logFileList: '[data-testid="log-file-list"]',
  logFileButton: 'button[data-log-file]',
  logTable: '[data-testid="log-table"]',
  searchInput: 'input[placeholder*="Search"]',
  levelSelect: 'select[name="level"]',
  prevPageButton: 'button[aria-label="Previous page"]',
  nextPageButton: 'button[aria-label="Next page"]',
  pageInfo: '[data-testid="page-info"]',
  emptyLogState: '[data-testid="empty-log"]',
} as const;

export const UPTIME_SELECTORS = {
  pageTitle: 'h1 >> text=Uptime',
  monitorCard: '[data-testid="monitor-card"]',
  statusBadge: '[data-testid="status-badge"]',
  refreshButton: 'button[aria-label="Check now"]',
  settingsDropdown: 'button[aria-label="Settings"]',
  editOption: '[role="menuitem"]:has-text("Edit")',
  deleteOption: '[role="menuitem"]:has-text("Delete")',
  editModal: '[role="dialog"]',
  nameInput: 'input[name="name"]',
  urlInput: 'input[name="url"]',
  saveButton: 'button:has-text("Save")',
  createButton: 'button:has-text("Add Monitor")',
  syncButton: 'button:has-text("Sync")',
  emptyState: '[data-testid="empty-state"]',
  confirmDialog: '[role="dialog"]',
  confirmDelete: 'button:has-text("Delete")',
  heartbeatBar: '[data-testid="heartbeat-bar"]',
} as const;

export const LIVE_LOG_SELECTORS = {
  connectionStatus: '[data-testid="connection-status"]',
  connectedIndicator: '.bg-green-900',
  disconnectedIndicator: '.bg-red-900',
  connectionError: '[data-testid="connection-error"]',
  modeToggle: '[data-testid="mode-toggle"]',
  applicationModeButton: 'button:has-text("App")',
  securityModeButton: 'button:has-text("Security")',
  pauseButton: 'button[title="Pause"]',
  playButton: 'button[title="Resume"]',
  clearButton: 'button[title="Clear logs"]',
  textFilter: 'input[placeholder*="Filter by text"]',
  levelSelect: 'select >> text=All Levels',
  sourceSelect: 'select >> text=All Sources',
  blockedOnlyCheckbox: 'input[type="checkbox"]',
  logContainer: '.font-mono.text-xs',
  logEntry: '[data-testid="log-entry"]',
  blockedEntry: '.bg-red-900\\/30',
  logCount: '[data-testid="log-count"]',
  pausedIndicator: '.text-yellow-400 >> text=Paused',
} as const;

export const IMPORT_SELECTORS = {
  fileDropzone: '[data-testid="file-dropzone"]',
  fileInput: 'input[type="file"]',
  pasteTextarea: 'textarea[placeholder*="Paste"]',
  uploadButton: 'button:has-text("Upload")',
  importBanner: '[data-testid="import-banner"]',
  continueButton: 'button:has-text("Continue")',
  cancelButton: 'button:has-text("Cancel")',
  reviewTable: '[data-testid="import-review-table"]',
  hostRow: '[data-testid="import-host-row"]',
  hostCheckbox: 'input[type="checkbox"][name="selected"]',
  conflictBadge: '[data-testid="conflict-badge"]',
  errorBadge: '[data-testid="error-badge"]',
  commitButton: 'button:has-text("Commit")',
  selectAllCheckbox: 'input[type="checkbox"][name="select-all"]',
  successModal: '[data-testid="import-success-modal"]',
  viewHostsButton: 'button:has-text("View Hosts")',
  expiryWarning: '[data-testid="session-expiry-warning"]',
} as const;
