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

import { APIRequestContext, APIResponse } from '@playwright/test';
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
