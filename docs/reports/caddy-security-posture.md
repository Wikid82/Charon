## PR-2 Security Patch Posture and Advisory Disposition

- Date: 2026-02-23
- Scope: PR-2 only (security patch posture + xcaddy patch retirement decision)
- Upstream target: Caddy 2.11.x line (`2.11.1` candidate in this repository)
- Inputs:
  - PR-1 compatibility matrix: `docs/reports/caddy-compatibility-matrix.md`
  - Plan authority: `docs/plans/current_spec.md`
  - Runtime and bootstrap assumptions: `.docker/docker-entrypoint.sh`, `.docker/compose/docker-compose.yml`

### 1) Final patch disposition

| Patch target | Decision | Rationale (evidence-backed) | Rollback path |
| --- | --- | --- | --- |
| `github.com/expr-lang/expr@v1.17.7` | Retain | Enforced by current builder patching and CI dependency checks. | Keep current pin. |
| `github.com/hslatman/ipstore@v0.4.0` | Retain | No PR-2 evidence supports safe retirement. | Keep current pin. |
| `github.com/slackhq/nebula@v1.9.7` | Retire by default | Matrix evidence supports scenario `B`/`C`; default moved to `B` with rollback preserved. | Set `CADDY_PATCH_SCENARIO=A`. |

### 2) Caddy 2.11.x advisory disposition

| Advisory | Component summary | Exploitability | Evidence source | Owner | Recheck cadence |
| --- | --- | --- | --- | --- | --- |
| `GHSA-5r3v-vc8m-m96g` (`CVE-2026-27590`) | FastCGI `split_path` confusion | Not affected | Upstream advisory + Charon runtime path review (no FastCGI transport in default generated config path) | QA_Security | weekly |
| `GHSA-879p-475x-rqh2` (`CVE-2026-27589`) | Admin API cross-origin no-cors | Mitigated | Upstream advisory + local controls: `CHARON_CADDY_ADMIN_API` now validated against internal allowlist and expected port 2019; production compose does not publish 2019 by default | QA_Security | weekly |
| `GHSA-x76f-jf84-rqj8` (`CVE-2026-27588`) | Host matcher case bypass | Mitigated | Upstream advisory + PR-1 Caddy 2.11.x matrix compatibility evidence and Charon route/security test reliance on upgraded line | QA_Security | release-candidate |
| `GHSA-g7pc-pc7g-h8jh` (`CVE-2026-27587`) | Path matcher escaped-case bypass | Mitigated | Upstream advisory + PR-1 matrix evidence and maintained security enforcement suite coverage | QA_Security | release-candidate |
| `GHSA-hffm-g8v7-wrv7` (`CVE-2026-27586`) | mTLS client-auth fail-open | Not affected | Upstream advisory + Charon default deployment model does not enable mTLS client-auth CA pool configuration by default | QA_Security | on-upstream-change |
| `GHSA-4xrr-hq4w-6vf4` (`CVE-2026-27585`) | File matcher glob sanitization bypass | Not affected | Upstream advisory + no default Charon generated config dependency on vulnerable matcher pattern | QA_Security | on-upstream-change |

### 3) Admin API exposure assumptions and hardening status

- Assumption: only internal Caddy admin endpoints are valid management targets.
- PR-2 enforcement:
  - validate and normalize `CHARON_CADDY_ADMIN_API`/`CPM_CADDY_ADMIN_API`
  - host allowlist + expected port `2019`
  - fail-fast startup on invalid/non-allowlisted endpoint
- Exposure check: production compose defaults do not publish port `2019`.

### 4) Runtime safety and rollback preservation

- Runtime defaults keep `expr` and `ipstore` pinned.
- `nebula` pin retirement is controlled by scenario switch, not hard deletion.
- Emergency rollback remains one-step: `CADDY_PATCH_SCENARIO=A`.

### Validation executed for PR-2

| Command / Task | Outcome |
| --- | --- |
| `cd /projects/Charon/backend && go test ./internal/config` | PASS |
| VS Code task `Security: Caddy PR-1 Compatibility Matrix` | PASS (A/B/C scenarios pass on `linux/amd64` and `linux/arm64`; promotion gate PASS) |

Relevant generated artifacts:
- `docs/reports/caddy-compatibility-matrix.md`
- `test-results/caddy-compat/matrix-summary.csv`
- `test-results/caddy-compat/module-inventory-*-go-version-m.txt`
- `test-results/caddy-compat/module-inventory-*-modules.txt`

### Residual risks / follow-up watch

1. Caddy advisories with reserved or evolving CVE enrichment may change exploitability interpretation; recheck cadence remains active.
2. Caddy bootstrap still binds admin listener to container interface (`0.0.0.0:2019`) for compatibility, so operator misconfiguration that publishes port `2019` can expand attack surface; production compose defaults avoid publishing this port.

### PR-2 closure statement

PR-2 posture decisions are review-ready: patch disposition is explicit, admin API assumptions are enforced, and rollback remains deterministic. No PR-3 scope is included.
