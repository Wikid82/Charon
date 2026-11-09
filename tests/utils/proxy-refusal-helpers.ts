/**
 * Helpers for security E2E cases that send requests through the Caddy proxy hop and check refused responses
 * (403 from an access list, 429 from rate limiting).
 *
 * The proxy hop is reached on PLAYWRIGHT_CADDY_PROXY_PORT (default 80) with a Host header, so a case can target a
 * proxy host it created without DNS. The shared setup keeps proxy hosts, header profiles and country lists in one
 * place for the ACL, security headers and rate limit specs.
 */

import { expect, type APIRequestContext, type APIResponse } from '@playwright/test';
import { PROXY_HOSTS_PATH } from './security-settings-api';

const APP_BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';
const ACCESS_LISTS_PATH = '/api/v1/access-lists';
const SECURITY_HEADER_PROFILES_PATH = '/api/v1/security/headers/profiles';

/**
 * Response headers that a profile created by createHeaderProfile() must produce.
 * Playwright lowercases response header names.
 */
export const PROFILE_HEADER_EXPECTATIONS = {
  'x-frame-options': 'DENY',
  'x-content-type-options': 'nosniff',
  'referrer-policy': 'no-referrer',
};

export interface HeaderProfile {
  id: number;
  uuid: string;
}

export interface ProxyHostOptions {
  domain: string;
  forwardPort: number;
  name: string;
  accessListUuid?: string;
  headerProfileUuid?: string;
}

/** Origin of the Caddy proxy hop, derived from the app URL the same way the route handling cases do. */
export function proxyOrigin(): string {
  const url = new URL(APP_BASE_URL);
  return `${url.protocol}//${url.hostname}:${process.env.PLAYWRIGHT_CADDY_PROXY_PORT || '80'}`;
}

/** Sends one request through the proxy hop with the given Host header. Refused statuses are returned, not thrown. */
export async function fetchThroughProxy(
  context: APIRequestContext,
  domain: string,
  path: string,
  method = 'GET'
): Promise<APIResponse> {
  return context.fetch(`${proxyOrigin()}${path}`, {
    method,
    headers: { Host: domain },
    data: method === 'POST' ? {} : undefined,
    failOnStatusCode: false,
    maxRedirects: 0,
  });
}

/** Creates an allow-list that admits only the given countries. Returns its uuid. */
export async function createCountryAllowList(
  request: APIRequestContext,
  name: string,
  countryCodes = 'US'
): Promise<string> {
  const response = await request.post(ACCESS_LISTS_PATH, {
    data: { name, type: 'geo_whitelist', country_codes: countryCodes, enabled: true },
  });
  expect(response.ok(), `create country allow list: ${response.status()}`).toBe(true);
  return (await response.json()).uuid;
}

export async function deleteAccessList(request: APIRequestContext, uuid: string): Promise<void> {
  await request.delete(`${ACCESS_LISTS_PATH}/${uuid}`);
}

/** Creates a security header profile whose values match PROFILE_HEADER_EXPECTATIONS. */
export async function createHeaderProfile(request: APIRequestContext, name: string): Promise<HeaderProfile> {
  const response = await request.post(SECURITY_HEADER_PROFILES_PATH, {
    data: {
      name,
      hsts_enabled: false,
      csp_enabled: false,
      x_frame_options: 'DENY',
      x_content_type_options: true,
      referrer_policy: 'no-referrer',
      xss_protection: false,
      cache_control_no_store: false,
    },
  });
  expect(response.ok(), `create header profile: ${response.status()}`).toBe(true);
  const { profile } = (await response.json()) as { profile: HeaderProfile };
  return { id: profile.id, uuid: profile.uuid };
}

export async function deleteHeaderProfile(request: APIRequestContext, profile: HeaderProfile): Promise<void> {
  await request.delete(`${SECURITY_HEADER_PROFILES_PATH}/${profile.id}`);
}

/** Creates an enabled proxy host that forwards to a local port, optionally with an access list and header profile. */
export async function createProxyHost(request: APIRequestContext, options: ProxyHostOptions): Promise<string> {
  const response = await request.post(PROXY_HOSTS_PATH, {
    data: {
      domain_names: options.domain,
      forward_host: '127.0.0.1',
      forward_port: options.forwardPort,
      forward_scheme: 'http',
      websocket_support: false,
      enabled: true,
      name: options.name,
      access_list_id: options.accessListUuid,
      security_header_profile_id: options.headerProfileUuid,
    },
  });
  expect(response.ok(), `create proxy host ${options.domain}: ${response.status()}`).toBe(true);
  const body = await response.json();
  return body.uuid ?? body.id;
}

export async function deleteProxyHost(request: APIRequestContext, uuid: string): Promise<void> {
  await request.delete(`${PROXY_HOSTS_PATH}/${uuid}`);
}
