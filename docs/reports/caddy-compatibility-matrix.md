# PR-1 Caddy Compatibility Matrix Report

- Generated at: 2026-02-23T13:52:26Z
- Candidate Caddy version: 2.11.1
- Plugin set: caddy-security,coraza-caddy,caddy-crowdsec-bouncer,caddy-geoip2,caddy-ratelimit
- Smoke set: boot_caddy,plugin_modules,config_validate,admin_api_health
- Matrix dimensions: patch scenario × platform/arch × checked plugin modules

## Deterministic Pass/Fail

A matrix cell is PASS only when every smoke check and module inventory extraction passes.

Promotion gate semantics (spec-aligned):
- Scenario A on linux/amd64 and linux/arm64 is promotion-gating.
- Scenario B/C are evidence-only; failures in B/C do not fail the PR-1 promotion gate.

## Matrix Output

| Scenario | Platform | Plugins Checked | boot_caddy | plugin_modules | config_validate | admin_api_health | module_inventory | Status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| A | linux/amd64 | http.handlers.auth_portal, http.handlers.waf, http.handlers.crowdsec, http.handlers.geoip2, http.handlers.rate_limit | PASS | PASS | PASS | PASS | PASS | PASS |
| A | linux/arm64 | http.handlers.auth_portal, http.handlers.waf, http.handlers.crowdsec, http.handlers.geoip2, http.handlers.rate_limit | PASS | PASS | PASS | PASS | PASS | PASS |
| B | linux/amd64 | http.handlers.auth_portal, http.handlers.waf, http.handlers.crowdsec, http.handlers.geoip2, http.handlers.rate_limit | PASS | PASS | PASS | PASS | PASS | PASS |
| B | linux/arm64 | http.handlers.auth_portal, http.handlers.waf, http.handlers.crowdsec, http.handlers.geoip2, http.handlers.rate_limit | PASS | PASS | PASS | PASS | PASS | PASS |
| C | linux/amd64 | http.handlers.auth_portal, http.handlers.waf, http.handlers.crowdsec, http.handlers.geoip2, http.handlers.rate_limit | PASS | PASS | PASS | PASS | PASS | PASS |
| C | linux/arm64 | http.handlers.auth_portal, http.handlers.waf, http.handlers.crowdsec, http.handlers.geoip2, http.handlers.rate_limit | PASS | PASS | PASS | PASS | PASS | PASS |

## Artifacts

- Matrix CSV: test-results/caddy-compat/matrix-summary.csv
- Per-cell module inventories: test-results/caddy-compat/module-inventory-*-go-version-m.txt
- Per-cell Caddy module listings: test-results/caddy-compat/module-inventory-*-modules.txt
