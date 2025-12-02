# Cerberus Security Suite

Cerberus is Charon's optional, modular security layer bundling a lightweight WAF pipeline, CrowdSec integration, Access Control Lists (ACLs), and future rate limiting. It focuses on *ease of enablement*, *observability first*, and *gradual enforcement* so home and small business users avoid accidental lockouts.

---
## Architecture Overview

Cerberus sits as a Gin middleware applied to all `/api/v1` routes (and indirectly protects reverse proxy management workflows). Components:

| Component | Purpose | Current Status |
| :--- | :--- | :--- |
| WAF | Inspect requests, detect payload signatures, optionally block | Prototype (placeholder `<script>` detection) |
| CrowdSec | Behavior & reputation-based IP decisions | Local agent planned; mode wiring present |
| ACL | Static allow/deny (IP, CIDR, geo) per host | Implemented (evaluates active lists) |
| Rate Limiting | Volume-based abuse prevention | Placeholder (API + config stub) |
| Decisions & Audit | Persist actions for UI visibility | Implemented models + listing |
| Rulesets | Persist rule content/metadata for dynamic WAF config | CRUD implemented |
| Break-Glass | Emergency disable token generation & verification | Implemented |

### Request Flow (Simplified)
1. Cerberus `IsEnabled()` checks global flags and dynamic DB setting.
2. WAF (if `waf_mode != disabled`) increments `charon_waf_requests_total` and evaluates payload.
3. If suspicious and in `block` mode (design intent), reject with JSON error; otherwise log & continue in `monitor`.
4. ACL evaluation (if enabled) tests client IP against active lists; may 403.
5. CrowdSec & Rate Limit placeholders reserved for future enforcement phases.
6. Downstream handler runs if not aborted.

> Note: Current prototype blocks suspicious payloads even in `monitor` mode; future refinement will ensure true log-only behavior. Monitor first for safe rollout.

---
## Configuration Model

Global config persisted via `/api/v1/security/config` matches `SecurityConfig`:
```json
{
  "name": "default",
  "enabled": true,
  "admin_whitelist": "198.51.100.10,203.0.113.0/24",
  "crowdsec_mode": "local",
  "waf_mode": "monitor",
  "waf_rules_source": "owasp-crs-local",
  "waf_learning": true,
  "rate_limit_enable": false,
  "rate_limit_burst": 0,
  "rate_limit_requests": 0,
  "rate_limit_window_sec": 0
}
```

Environment variables (fallback defaults) mirror these settings (`CERBERUS_SECURITY_WAF_MODE`, etc.). Runtime enable/disable uses `/security/enable` & `/security/disable` with whitelist or break-glass validation.

---
## WAF Details

| Field | Meaning |
| :--- | :--- |
| `waf_mode` | `disabled`, `monitor`, `block` |
| `waf_rules_source` | Identifier or URL for ruleset content |
| `waf_learning` | Flag for future adaptive tuning |

Metrics (Prometheus):
```
charon_waf_requests_total
charon_waf_blocked_total
charon_waf_monitored_total
```
Structured log fields:
```
source: "waf"
decision: "block" | "monitor"
mode: "block" | "monitor" | "disabled"
path: request path
query: raw query string
```

Rulesets (`SecurityRuleSet`) are managed via `/security/rulesets` and store raw rule `content` plus metadata (`name`, `source_url`, `mode`). The Caddy manager applies changes after upsert/delete.

---
## Access Control Lists

Each ACL defines IP/Geo whitelist/blacklist semantics. Cerberus iterates enabled lists and calls `AccessListService.TestIP()`; the first denial aborts with 403. Use ACLs for *static* restrictions (internal-only, geofencing) and rely on CrowdSec / rate limiting for dynamic attacker behavior.

---
## Decisions & Auditing

`SecurityDecision` captures source (`waf`, `crowdsec`, `ratelimit`, `manual`), action (`allow`, `block`, `challenge`), and context. Manual overrides are created via `POST /security/decisions`. Audit entries (`SecurityAudit`) record actor + action for UI timelines (future visualization).

---
## Break-Glass & Lockout Prevention

- Include at least one trusted IP/CIDR in `admin_whitelist` before enabling.
- Generate a token with `POST /security/breakglass/generate`; store securely.
- Disable from localhost without token for emergency local access.

Rollout path:
1. Set `waf_mode=monitor`.
2. Observe metrics & logs; tune rulesets.
3. Add `admin_whitelist` entries.
4. Switch to `block`.

---
## Observability Patterns

Suggested PromQL ideas:
- Block Rate: `rate(charon_waf_blocked_total[5m]) / rate(charon_waf_requests_total[5m])`
- Monitor Volume: `rate(charon_waf_monitored_total[5m])`
- Drift After Enforcement: Compare block vs monitor trend pre/post switch.

Alerting:
- High block rate spike (>30% sustained 10m)
- Zero evaluations (requests counter flat) indicating middleware misconfiguration

---
## Roadmap Phases

| Phase | Focus | Status |
| :--- | :--- | :--- |
| 1 | WAF prototype + observability | Complete |
| 2 | CrowdSec local agent integration | Pending |
| 3 | True WAF rule evaluation (Coraza CRS load) | Pending |
| 4 | Rate limiting enforcement | Pending |
| 5 | Advanced dashboards + adaptive learning | Planned |

---
## FAQ

**Why monitor before block?** Prevent accidental service impact; gather baseline.

**Can I scrape `/metrics` securely?** Place behind network-level controls or reverse proxy requiring auth; endpoint itself is unauthenticated for simplicity.

**Does monitor mode block today?** Prototype still blocks suspicious `<script>` payloads; this will change to pure logging in a future refinement.

---
## See Also
- [Security Overview](security.md)
- [Features](features.md)
- [API Reference](api.md)
