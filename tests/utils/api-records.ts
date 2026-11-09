/**
 * Minimal shapes of API records that E2E specs read back via `response.json()`.
 *
 * Playwright types JSON bodies as `any`. These interfaces declare only the fields the specs
 * inspect, so callbacks such as `users.find((u) => u.email === ...)` are type-checked.
 * Add fields here when a spec starts reading a new property.
 */

/** A proxy host as returned by `GET /api/v1/proxy-hosts`. */
export interface ProxyHostRecord {
  uuid?: string;
  domain_names?: string;
  domainNames?: string;
}

/** A user as returned by `GET /api/v1/users`. */
export interface UserRecord {
  id?: number | string;
  email?: string;
  role?: string;
}

/** A host in a Caddy import preview. Extra fields are allowed. */
export interface PreviewHostRecord {
  domain_names?: string;
  warnings?: unknown[];
  [key: string]: unknown;
}

/** An audit log entry as returned by `GET /api/v1/audit-logs`. */
export interface AuditLogRecord {
  action?: string;
  event_category?: string;
  details?: string;
  [key: string]: unknown;
}
