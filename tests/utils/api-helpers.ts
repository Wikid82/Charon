/**
 * API Helpers - Common API operations for E2E tests
 *
 * This module provides utility functions for interacting with the Charon API
 * in E2E tests. These helpers abstract common operations and provide
 * consistent error handling.
 *
 * @example
 * ```typescript
 * import { createProxyHostViaAPI, deleteProxyHostViaAPI } from './utils/api-helpers';
 *
 * test('create and delete proxy host', async ({ request }) => {
 *   const auth = await authenticateViaAPI(request, 'admin@test.local', 'TestPass123!');
 *   const { id } = await createProxyHostViaAPI(request, {
 *     domain: 'test.example.com',
 *     forwardHost: '192.168.1.100',
 *     forwardPort: 3000
 *   }, auth.token);
 *   await deleteProxyHostViaAPI(request, id, auth.token);
 * });
 * ```
 */

import { APIRequestContext, APIResponse, expect, request as playwrightRequest, type Page } from '@playwright/test';
import { readFileSync } from 'fs';
import { STORAGE_STATE } from '../constants';

/**
 * Read auth token from storage state and return Authorization headers.
 * Use this for page.request calls that need Bearer token auth.
 */
export function getStorageStateAuthHeaders(): Record<string, string> {
  try {
    const state = JSON.parse(readFileSync(STORAGE_STATE, 'utf-8'));
    for (const origin of state.origins ?? []) {
      for (const entry of origin.localStorage ?? []) {
        if (entry.name === 'charon_auth_token' && entry.value) {
          return { Authorization: `Bearer ${entry.value}` };
        }
      }
    }
    for (const cookie of state.cookies ?? []) {
      if (cookie.name === 'auth_token' && cookie.value) {
        return { Authorization: `Bearer ${cookie.value}` };
      }
    }
  } catch { /* no-op */ }
  return {};
}

/**
 * API error response
 */
export interface APIError {
  status: number;
  message: string;
  details?: Record<string, unknown>;
}

/**
 * Authentication response
 */
export interface AuthResponse {
  token: string;
  user: {
    id: string;
    email: string;
    role: string;
  };
  expiresAt: string;
}

/**
 * Proxy host creation data
 */
export interface ProxyHostCreateData {
  domain: string;
  forwardHost: string;
  forwardPort: number;
  scheme?: 'http' | 'https';
  websocketSupport?: boolean;
  enabled?: boolean;
  certificateId?: string;
  accessListId?: string;
}

/**
 * Proxy host response from API
 */
export interface ProxyHostResponse {
  id: string;
  uuid: string;
  domain: string;
  forward_host: string;
  forward_port: number;
  scheme: string;
  websocket_support: boolean;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * Access list creation data
 */
export interface AccessListCreateData {
  name: string;
  rules: Array<{ type: 'allow' | 'deny'; value: string }>;
  description?: string;
  authEnabled?: boolean;
  authUsers?: Array<{ username: string; password: string }>;
}

/**
 * Access list response from API
 */
export interface AccessListResponse {
  id: string;
  name: string;
  rules: Array<{ type: string; value: string }>;
  description?: string;
  auth_enabled: boolean;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * Certificate creation data
 */
export interface CertificateCreateData {
  domains: string[];
  type: 'letsencrypt' | 'custom';
  certificate?: string;
  privateKey?: string;
  intermediates?: string;
  dnsProviderId?: string;
  acmeEmail?: string;
}

/**
 * Certificate response from API
 */
export interface CertificateResponse {
  id: string;
  domains: string[];
  type: string;
  status: string;
  issuer?: string;
  expires_at?: string;
  created_at: string;
  updated_at: string;
}

/**
 * Backup entry response from API
 */
export interface BackupResponse {
  filename: string;
  size: number;
  time: string;
  uuid?: string;
  type?: string;
  encrypted?: boolean;
  format_version?: number;
  sha256?: string;
  status?: string;
  app_version?: string;
}

/**
 * Default request options with authentication
 */
function getAuthHeaders(token?: string): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

/**
 * Parse API response and throw on error
 */
async function parseResponse<T>(response: APIResponse): Promise<T> {
  if (!response.ok()) {
    const text = await response.text();
    let message = `API Error: ${response.status()} ${response.statusText()}`;
    try {
      const error = JSON.parse(text);
      message = error.message || error.error || message;
    } catch {
      message = text || message;
    }
    throw new Error(message);
  }
  return response.json();
}

/**
 * Authenticate via API and return token
 * @param request - Playwright APIRequestContext
 * @param email - User email
 * @param password - User password
 * @returns Authentication response with token
 *
 * @example
 * ```typescript
 * const auth = await authenticateViaAPI(request, 'admin@test.local', 'TestPass123!');
 * console.log(auth.token);
 * ```
 */
export async function authenticateViaAPI(
  request: APIRequestContext,
  email: string,
  password: string
): Promise<AuthResponse> {
  const response = await request.post('/api/v1/auth/login', {
    data: { email, password },
    headers: { 'Content-Type': 'application/json' },
  });

  return parseResponse<AuthResponse>(response);
}

/**
 * Suppress the "What's New" changelog modal for a throwaway user created
 * directly via `POST /api/v1/users` (e.g. an ad-hoc `createUserViaApi`
 * helper), outside the shared `TestDataManager`-managed user pool.
 *
 * New users get the real production defaults — `changelog_opt_out: false`
 * and (on E2E's unversioned "dev" build) `last_seen_version: ""` — so they
 * are eligible to see the blocking "What's New" dialog (see
 * `WhatsNewModal.tsx`) the moment they're logged into and the UI is
 * interacted with. `TestDataManager.createUser()` already guards against
 * this for the shared pool by opting new users out via the self-service
 * `/api/v1/changelog/ack` endpoint immediately after creation; this helper
 * mirrors that exact mechanism for specs that bypass the pool.
 *
 * Logs in as the new user through an **isolated** request context rather
 * than `page.request` / the caller's admin session — `POST /auth/login`
 * sets a same-origin `auth_token` cookie, and reusing the caller's request
 * context (which shares cookies with `page`) would silently swap the
 * browser's authenticated session out from under the admin flow the
 * calling test is otherwise driving.
 *
 * Best-effort: a failure here must not fail the calling test — worst case
 * the modal appears for this throwaway user.
 */
export async function suppressChangelogModal(
  page: Page,
  email: string,
  password: string
): Promise<void> {
  let baseURL: string;
  try {
    baseURL = new URL(page.url()).origin;
  } catch {
    baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
  }

  const loginContext = await playwrightRequest.newContext({
    baseURL,
    extraHTTPHeaders: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
  });

  try {
    const auth = await authenticateViaAPI(loginContext, email, password).catch(() => null);
    if (!auth?.token) {
      console.warn(`Failed to suppress changelog modal for ${email}: login failed`);
      return;
    }

    const ackResponse = await loginContext.post('/api/v1/changelog/ack', {
      headers: { Authorization: `Bearer ${auth.token}` },
      data: { action: 'dismiss_permanent', opt_out: true },
    });
    if (!ackResponse.ok()) {
      console.warn(`Failed to suppress changelog modal for ${email}: ${ackResponse.status()}`);
    }
  } catch (error) {
    console.warn(`Failed to suppress changelog modal for ${email}:`, error);
  } finally {
    await loginContext.dispose();
  }
}

/**
 * Resolve the Caddy reverse-proxy origin for a page.
 *
 * `page.url()`'s origin is the Charon *management* interface (default
 * port 8080) — per ARCHITECTURE.md ("Management Interface (Port 8080)":
 * "NO Cerberus Middleware: Rate limiting, ACL, WAF, and CrowdSec are NOT
 * applied to management interface") that origin never runs WAF/ACL/rate
 * limiting/CrowdSec, and its `/` route unconditionally serves the SPA's
 * `index.html` regardless of query string, so a request built from it can
 * never be blocked no matter what payload is sent. Requests that exercise
 * Cerberus enforcement (WAF, rate limiting, etc.) for a *proxied host* must
 * instead target the Caddy proxy port (80/443 — "Port 80/443 (Proxy)" in
 * ARCHITECTURE.md), with the proxied domain in the `Host` header so Caddy
 * routes to the right upstream and runs its security middleware chain.
 *
 * Defaults to port 80 (matches `docker-compose.playwright-ci.yml`'s
 * `security-tests` profile, which maps host port 80 -> container port 80
 * on the same host as `PLAYWRIGHT_BASE_URL`). Overridable via
 * `PLAYWRIGHT_CADDY_PROXY_PORT` for local investigation setups that remap
 * the host port to avoid colliding with another Caddy/Charon instance.
 *
 * @param page - Playwright Page instance
 */
export function caddyProxyOrigin(page: Page): string {
  const managementUrl = new URL(page.url());
  const proxyPort = process.env.PLAYWRIGHT_CADDY_PROXY_PORT || '80';
  return `${managementUrl.protocol}//${managementUrl.hostname}:${proxyPort}`;
}

/**
 * Read the current session's auth token from the page's localStorage,
 * actively waiting for it rather than taking a single synchronous read.
 *
 * `AuthContext.login()` (`frontend/src/context/AuthContext.tsx`) writes
 * `charon_auth_token` to localStorage synchronously, but a UI login flow
 * (fill email/password, click "Sign in") is a client-side SPA action with
 * no full page navigation. `page.waitForLoadState('networkidle')` called
 * right after that click is therefore not a reliable signal: Playwright's
 * networkidle bookkeeping is tied to the frame's navigation lifecycle, and
 * since no navigation occurs, the call can resolve immediately against the
 * frame's pre-existing idle state — before the click's async login handler
 * has actually run and written the token. Reading localStorage right after
 * `waitForLoadState('networkidle')` is therefore a real race (confirmed via
 * trace inspection: the token write and the follow-up `/auth/me` call both
 * happened tens of milliseconds *after* the read). Poll instead.
 *
 * @param page - Playwright Page instance
 * @param options.timeout - Max time to wait for the token to appear (default 10000ms)
 */
export async function getAuthTokenFromPage(
  page: Page,
  options: { timeout?: number } = {}
): Promise<string> {
  const { timeout = 10000 } = options;

  await page
    .waitForFunction(
      () => !!(localStorage.getItem('charon_auth_token') || localStorage.getItem('token')),
      undefined,
      { timeout }
    )
    .catch(() => {
      // Fall through to the read below so the caller gets a clear
      // "" -> toBeTruthy() failure with real state, not a swallowed timeout.
    });

  const token = await page.evaluate(
    () => localStorage.getItem('charon_auth_token') || localStorage.getItem('token') || ''
  );

  expect(token).toBeTruthy();
  return token;
}

/**
 * Create a user directly via `POST /api/v1/users`, idempotently.
 *
 * The admin "add user" UI moved to an invite-only flow (`InviteModal` in
 * `UsersPage.tsx`) that only collects email + role — there's no "Name" or
 * "Password" field left to drive from a test, so tests that need a user
 * with a known password must create one via this direct API call instead.
 *
 * Idempotent by design: if a user with this email already exists (e.g. a
 * previous run's test crashed before its `afterEach` cleanup ran, leaving
 * the account behind), a 409 from the create call is treated as recoverable
 * — the existing user is looked up and deleted, then creation is retried
 * once. Without this, any prior leftover state permanently wedges every
 * subsequent run that reuses the same fixed test email.
 *
 * @param page - Playwright Page instance (used for the admin's own auth token)
 * @param user - New user's email/name/password/role
 */
export async function createUserViaApi(
  page: Page,
  user: { email: string; name: string; password: string; role: 'admin' | 'user' | 'guest' }
): Promise<{ id: string | number; email: string }> {
  const token = await getAuthTokenFromPage(page);

  const create = async () =>
    page.request.post('/api/v1/users', {
      data: user,
      headers: { Authorization: `Bearer ${token}` },
    });

  let response = await create();

  if (response.status() === 409) {
    const existingResponse = await page.request.get('/api/v1/users', {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (existingResponse.ok()) {
      const existingUsers = await existingResponse.json();
      const existing = Array.isArray(existingUsers)
        ? existingUsers.find((u: { email?: string }) => u.email === user.email)
        : undefined;
      // DELETE /api/v1/users/:id requires the numeric ID (strconv.ParseUint
      // in UserHandler.DeleteUser) — the UUID is not accepted here.
      const existingId = existing?.id;
      if (existingId !== undefined && existingId !== null) {
        await page.request.delete(`/api/v1/users/${existingId}`, {
          headers: { Authorization: `Bearer ${token}` },
        });
      }
    }
    response = await create();
  }

  expect(response.ok()).toBe(true);
  const payload = await response.json();
  expect(payload).toEqual(expect.objectContaining({
    id: expect.anything(),
    email: user.email,
  }));

  // Ad-hoc users created directly via this raw API call (bypassing the
  // shared TestDataManager pool) get the real production changelog
  // defaults, so they're eligible for the blocking "What's New" modal on
  // first login — see suppressChangelogModal's doc comment.
  await suppressChangelogModal(page, user.email, user.password);

  return { id: payload.id, email: payload.email };
}

/**
 * Create a proxy host via API
 * @param request - Playwright APIRequestContext
 * @param data - Proxy host configuration
 * @param token - Authentication token (optional if using cookie auth)
 * @returns Created proxy host details
 *
 * @example
 * ```typescript
 * const { id, domain } = await createProxyHostViaAPI(request, {
 *   domain: 'app.example.com',
 *   forwardHost: '192.168.1.100',
 *   forwardPort: 3000
 * });
 * ```
 */
export async function createProxyHostViaAPI(
  request: APIRequestContext,
  data: ProxyHostCreateData,
  token?: string
): Promise<ProxyHostResponse> {
  const response = await request.post('/api/v1/proxy-hosts', {
    data,
    headers: getAuthHeaders(token),
  });

  return parseResponse<ProxyHostResponse>(response);
}

/**
 * Get all proxy hosts via API
 * @param request - Playwright APIRequestContext
 * @param token - Authentication token (optional)
 * @returns Array of proxy hosts
 *
 * @example
 * ```typescript
 * const hosts = await getProxyHostsViaAPI(request);
 * console.log(`Found ${hosts.length} proxy hosts`);
 * ```
 */
export async function getProxyHostsViaAPI(
  request: APIRequestContext,
  token?: string
): Promise<ProxyHostResponse[]> {
  const response = await request.get('/api/v1/proxy-hosts', {
    headers: getAuthHeaders(token),
  });

  return parseResponse<ProxyHostResponse[]>(response);
}

/**
 * Get a single proxy host by ID via API
 * @param request - Playwright APIRequestContext
 * @param id - Proxy host ID or UUID
 * @param token - Authentication token (optional)
 * @returns Proxy host details
 */
export async function getProxyHostViaAPI(
  request: APIRequestContext,
  id: string,
  token?: string
): Promise<ProxyHostResponse> {
  const response = await request.get(`/api/v1/proxy-hosts/${id}`, {
    headers: getAuthHeaders(token),
  });

  return parseResponse<ProxyHostResponse>(response);
}

/**
 * Update a proxy host via API
 * @param request - Playwright APIRequestContext
 * @param id - Proxy host ID or UUID
 * @param data - Updated configuration
 * @param token - Authentication token (optional)
 * @returns Updated proxy host details
 */
export async function updateProxyHostViaAPI(
  request: APIRequestContext,
  id: string,
  data: Partial<ProxyHostCreateData>,
  token?: string
): Promise<ProxyHostResponse> {
  const response = await request.put(`/api/v1/proxy-hosts/${id}`, {
    data,
    headers: getAuthHeaders(token),
  });

  return parseResponse<ProxyHostResponse>(response);
}

/**
 * Delete a proxy host via API
 * @param request - Playwright APIRequestContext
 * @param id - Proxy host ID or UUID
 * @param token - Authentication token (optional)
 *
 * @example
 * ```typescript
 * await deleteProxyHostViaAPI(request, 'uuid-123');
 * ```
 */
export async function deleteProxyHostViaAPI(
  request: APIRequestContext,
  id: string,
  token?: string
): Promise<void> {
  const response = await request.delete(`/api/v1/proxy-hosts/${id}`, {
    headers: getAuthHeaders(token),
  });

  if (!response.ok() && response.status() !== 404) {
    throw new Error(
      `Failed to delete proxy host: ${response.status()} ${await response.text()}`
    );
  }
}

/**
 * Create an access list via API
 * @param request - Playwright APIRequestContext
 * @param data - Access list configuration
 * @param token - Authentication token (optional)
 * @returns Created access list details
 *
 * @example
 * ```typescript
 * const { id } = await createAccessListViaAPI(request, {
 *   name: 'My ACL',
 *   rules: [{ type: 'allow', value: '192.168.1.0/24' }]
 * });
 * ```
 */
export async function createAccessListViaAPI(
  request: APIRequestContext,
  data: AccessListCreateData,
  token?: string
): Promise<AccessListResponse> {
  const response = await request.post('/api/v1/access-lists', {
    data,
    headers: getAuthHeaders(token),
  });

  return parseResponse<AccessListResponse>(response);
}

/**
 * Get all access lists via API
 * @param request - Playwright APIRequestContext
 * @param token - Authentication token (optional)
 * @returns Array of access lists
 */
export async function getAccessListsViaAPI(
  request: APIRequestContext,
  token?: string
): Promise<AccessListResponse[]> {
  const response = await request.get('/api/v1/access-lists', {
    headers: getAuthHeaders(token),
  });

  return parseResponse<AccessListResponse[]>(response);
}

/**
 * Get a single access list by ID via API
 * @param request - Playwright APIRequestContext
 * @param id - Access list ID
 * @param token - Authentication token (optional)
 * @returns Access list details
 */
export async function getAccessListViaAPI(
  request: APIRequestContext,
  id: string,
  token?: string
): Promise<AccessListResponse> {
  const response = await request.get(`/api/v1/access-lists/${id}`, {
    headers: getAuthHeaders(token),
  });

  return parseResponse<AccessListResponse>(response);
}

/**
 * Update an access list via API
 * @param request - Playwright APIRequestContext
 * @param id - Access list ID
 * @param data - Updated configuration
 * @param token - Authentication token (optional)
 * @returns Updated access list details
 */
export async function updateAccessListViaAPI(
  request: APIRequestContext,
  id: string,
  data: Partial<AccessListCreateData>,
  token?: string
): Promise<AccessListResponse> {
  const response = await request.put(`/api/v1/access-lists/${id}`, {
    data,
    headers: getAuthHeaders(token),
  });

  return parseResponse<AccessListResponse>(response);
}

/**
 * Delete an access list via API
 * @param request - Playwright APIRequestContext
 * @param id - Access list ID
 * @param token - Authentication token (optional)
 */
export async function deleteAccessListViaAPI(
  request: APIRequestContext,
  id: string,
  token?: string
): Promise<void> {
  const response = await request.delete(`/api/v1/access-lists/${id}`, {
    headers: getAuthHeaders(token),
  });

  if (!response.ok() && response.status() !== 404) {
    throw new Error(
      `Failed to delete access list: ${response.status()} ${await response.text()}`
    );
  }
}

/**
 * Create a certificate via API
 * @param request - Playwright APIRequestContext
 * @param data - Certificate configuration
 * @param token - Authentication token (optional)
 * @returns Created certificate details
 *
 * @example
 * ```typescript
 * const { id } = await createCertificateViaAPI(request, {
 *   domains: ['app.example.com'],
 *   type: 'letsencrypt'
 * });
 * ```
 */
export async function createCertificateViaAPI(
  request: APIRequestContext,
  data: CertificateCreateData,
  token?: string
): Promise<CertificateResponse> {
  const response = await request.post('/api/v1/certificates', {
    data,
    headers: getAuthHeaders(token),
  });

  return parseResponse<CertificateResponse>(response);
}

/**
 * Get all certificates via API
 * @param request - Playwright APIRequestContext
 * @param token - Authentication token (optional)
 * @returns Array of certificates
 */
export async function getCertificatesViaAPI(
  request: APIRequestContext,
  token?: string
): Promise<CertificateResponse[]> {
  const response = await request.get('/api/v1/certificates', {
    headers: getAuthHeaders(token),
  });

  return parseResponse<CertificateResponse[]>(response);
}

/**
 * Get a single certificate by ID via API
 * @param request - Playwright APIRequestContext
 * @param id - Certificate ID
 * @param token - Authentication token (optional)
 * @returns Certificate details
 */
export async function getCertificateViaAPI(
  request: APIRequestContext,
  id: string,
  token?: string
): Promise<CertificateResponse> {
  const response = await request.get(`/api/v1/certificates/${id}`, {
    headers: getAuthHeaders(token),
  });

  return parseResponse<CertificateResponse>(response);
}

/**
 * Delete a certificate via API
 * @param request - Playwright APIRequestContext
 * @param id - Certificate ID
 * @param token - Authentication token (optional)
 */
export async function deleteCertificateViaAPI(
  request: APIRequestContext,
  id: string,
  token?: string
): Promise<void> {
  const response = await request.delete(`/api/v1/certificates/${id}`, {
    headers: getAuthHeaders(token),
  });

  if (!response.ok() && response.status() !== 404) {
    throw new Error(
      `Failed to delete certificate: ${response.status()} ${await response.text()}`
    );
  }
}

/**
 * Renew a certificate via API
 * @param request - Playwright APIRequestContext
 * @param id - Certificate ID
 * @param token - Authentication token (optional)
 * @returns Updated certificate details
 */
export async function renewCertificateViaAPI(
  request: APIRequestContext,
  id: string,
  token?: string
): Promise<CertificateResponse> {
  const response = await request.post(`/api/v1/certificates/${id}/renew`, {
    headers: getAuthHeaders(token),
  });

  return parseResponse<CertificateResponse>(response);
}

/**
 * Get all backups via API
 * @param request - Playwright APIRequestContext
 * @param token - Authentication token (optional)
 * @returns Array of backups
 */
export async function getBackupsViaAPI(
  request: APIRequestContext,
  token?: string
): Promise<BackupResponse[]> {
  const response = await request.get('/api/v1/backups', {
    headers: getAuthHeaders(token),
  });

  return parseResponse<BackupResponse[]>(response);
}

/**
 * Check API health
 * @param request - Playwright APIRequestContext
 * @returns Health status
 */
export async function checkAPIHealth(
  request: APIRequestContext
): Promise<{ status: string; database: string; version?: string }> {
  const response = await request.get('/api/v1/health');
  return parseResponse(response);
}

/**
 * Wait for API to be healthy
 * @param request - Playwright APIRequestContext
 * @param timeout - Maximum time to wait in ms (default: 30000)
 * @param interval - Polling interval in ms (default: 1000)
 */
export async function waitForAPIHealth(
  request: APIRequestContext,
  timeout: number = 30000,
  interval: number = 1000
): Promise<void> {
  const startTime = Date.now();

  while (Date.now() - startTime < timeout) {
    try {
      const health = await checkAPIHealth(request);
      if (health.status === 'healthy' || health.status === 'ok') {
        return;
      }
    } catch {
      // API not ready yet
    }
    await new Promise((resolve) => setTimeout(resolve, interval));
  }

  throw new Error(`API not healthy after ${timeout}ms`);
}

/**
 * Make an authenticated API request with automatic error handling
 * @param request - Playwright APIRequestContext
 * @param method - HTTP method
 * @param path - API path
 * @param options - Request options
 * @returns Parsed response
 */
export async function apiRequest<T>(
  request: APIRequestContext,
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE',
  path: string,
  options: {
    data?: unknown;
    token?: string;
    params?: Record<string, string>;
  } = {}
): Promise<T> {
  const { data, token, params } = options;

  const requestOptions: Parameters<APIRequestContext['fetch']>[1] = {
    method,
    headers: getAuthHeaders(token),
  };

  if (data) {
    requestOptions.data = data;
  }

  if (params) {
    requestOptions.params = params;
  }

  const response = await request.fetch(path, requestOptions);
  return parseResponse<T>(response);
}
