## Active Plan: PR-0 Spec Correction Pass (Security Notifications Provider Events)

Date: 2026-02-20
Status: Active and authoritative (supersedes all prior appended sections in this file)

### 1) Scope and Intent

- This is a spec-only correction pass for compatibility, migration determinism, and rollout safety.
- Security notifications are provider-event subscriptions, not a single global destination.
- Notify-only runtime remains fail-closed; legacy fallback dispatch remains blocked.

### 2) Compatibility GET Aggregation Rules (Exact)

Compatibility endpoint in scope:

- `GET /api/v1/notifications/settings/security`

Aggregation source of truth:

- Active provider rows in `notification_providers` after filtering `enabled=true` and supported notify-only types.

Derived booleans (OR semantics):

- `security_waf_enabled = OR(provider.notify_security_waf_blocks)`
- `security_acl_enabled = OR(provider.notify_security_acl_denies)`
- `security_rate_limit_enabled = OR(provider.notify_security_rate_limit_hits)`

Deterministic conflict behavior when providers disagree:

- If any active provider has `true` for a category, compatibility GET returns `true` for that category.
- Returns `false` only when all active providers are `false` (or no active providers exist).
- Provider order never affects result.

Legacy destination reporting:

- If compatibility payload still includes `destination`, it is read-only compatibility metadata.
- `destination` is emitted only when exactly one active managed-legacy provider is in scope.
- If zero or more than one candidate exists, `destination` is returned as empty string and `destination_ambiguous=true` is set in compatibility metadata.

### 3) Compatibility PUT Translation Semantics (Exact)

Compatibility endpoint in scope:

- `PUT /api/v1/notifications/settings/security`

Deterministic target set:

- If one or more managed-legacy providers exist, target set is exactly that managed set.
- If none exist and request is valid, create one managed-legacy provider and use it as the target set.
- Non-managed providers are never mutated by compatibility PUT.

Translation mode: **replace on managed set** (not global merge)

- For each target provider, set security event booleans exactly to request values:
  - `notify_security_waf_blocks = request.security_waf_enabled`
  - `notify_security_acl_denies = request.security_acl_enabled`
  - `notify_security_rate_limit_hits = request.security_rate_limit_enabled`
- Existing non-security provider event booleans remain unchanged.
- Existing provider transport fields remain unchanged unless destination mapping applies (Section 5).

Idempotency:

- Repeating identical PUT payload produces no state drift and same compatibility GET output.
- Write timestamps may change only when effective values change.

Conflict handling:

- If target set cannot be resolved deterministically (for example data corruption with duplicate managed identity keys), return `409 Conflict` and do not partially write.
- If request includes unsupported destination mapping shape, return `422 Unprocessable Entity` and do not mutate providers.
- All compatibility PUT writes are transactional: all target providers updated or none.

### 4) Migration Marker Storage and Deterministic Re-Run

Marker storage:

- Table: `settings`
- Key: `notifications.security_provider_events.migration.v1`
- Value JSON schema:
  - `version` (string, fixed `v1`)
  - `checksum` (string, deterministic hash of legacy source fields used for migration)
  - `last_completed_at` (RFC3339 timestamp)
  - `result` (`completed` | `completed_with_warnings`)

Deterministic boot-time re-run behavior:

1. Read legacy source (`notification_configs`) and compute checksum.
2. Read migration marker.
3. If marker missing: run migration and write marker.
4. If marker exists with same checksum: skip mutation (no-op).
5. If marker exists but checksum differs: re-run migration in upsert mode, then replace marker.

Determinism constraints:

- Migration is pure over legacy source + managed provider key.
- Upsert key for managed provider is fixed identity `managed_legacy_security=true` plus name `Migrated Security Notifications (Legacy)`.
- Re-run does not create duplicate managed providers.

### 5) Legacy Destination-to-Provider Mapping Rules

Supported destination mappings for compatibility PUT:

- `webhook_url` present:
  - map to provider type `webhook`, set endpoint URL from `webhook_url`.
- `discord_webhook_url` present:
  - map to provider type `discord`, set endpoint URL from `discord_webhook_url`.
- `slack_webhook_url` present:
  - map to provider type `slack`, set endpoint URL from `slack_webhook_url`.
- `gotify_url` + `gotify_token` present:
  - map to provider type `gotify`, set URL/token fields.

Unsupported/ambiguous destination handling (fail-safe):

- If multiple destination types are simultaneously present in one request, return `422`.
- If destination type is unknown or incomplete for required fields, return `422`.
- On `422`, no provider rows are created/updated and compatibility state remains unchanged.

### 6) Feature Flag as Explicit Rollout Gate

Primary gate:

- `feature.notifications.security_provider_events.enabled`

Required defaults:

- Default in production: `false`
- Default in development/test: `true`

Behavior when flag is `false`:

- Provider-based security dispatch path is disabled.
- Compatibility GET/PUT remain available.
- Runtime dispatch uses compatibility translation path only.
- Managed migration may run in read-only dry-evaluate mode, but must not mutate providers.

Behavior when flag is `true`:

- Provider-based security dispatch is authoritative.
- Compatibility GET is derived projection from providers.
- Compatibility PUT writes managed set via translation semantics defined above.

Operational rule:

- This flag is mandatory for rollout and rollback; it is not optional/recommended.

### 7) Authoritative Plan Boundary and Cleanup

- This document is now the single authoritative plan for this scope.
- All previously appended certificate flaky-test and QA remediation sections are removed from active plan scope.
- Any future unrelated plan content must go to a separate plan file under `docs/plans/`.

### 8) PR Slicing Strategy

Decision: two PR slices.

- **PR-0 (this pass):** spec correction only in `docs/plans/current_spec.md`.
- **PR-1:** backend compatibility, migration marker, and feature-flag gate implementation.
- **PR-2:** frontend alignment and compatibility deprecation messaging.

### 9) Acceptance Criteria (Spec Pass)

1. Compatibility GET rules define exact OR aggregation and disagreement behavior.
2. Compatibility PUT defines deterministic target set, replace semantics, idempotency, and transaction/conflict behavior.
3. Migration marker key/storage and deterministic re-run logic are explicit.
4. Destination mapping rules cover supported non-webhook legacy forms and fail-safe unsupported behavior.
5. File contains one authoritative plan with stale duplicate trailing sections removed.
6. Feature flag is defined as explicit mandatory rollout gate with defaults and disable behavior.

### 10) Handoff

- Delegate PR-1 implementation to `Supervisor` using this plan as the sole baseline.
