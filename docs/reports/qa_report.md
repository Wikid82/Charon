# QA Report: fix/crowdsec-cleanup-batch (PR 6a)

Date: 2026-10-18. Scope: `git log development..HEAD` (9 commits, 30 files). Build under test: repo `Dockerfile`, CrowdSec cscli v1.8.2-rc1.

## Verdict: PASS. No Blocking or Major findings.

## Static and unit gates

| Gate | Result |
|---|---|
| `go vet` (internal/crowdsec/..., internal/api/handlers/...) | PASS, clean |
| `go test -race` internal/crowdsec/... and internal/api/handlers/... | PASS (115 s, 382 s) |
| `scripts/go-test-coverage.sh` | PASS: 92.2% statements, 91.1% lines (gate 87%) |
| `scripts/local-patch-report.sh` | PASS: overall 97.6% (80/82), backend 97.5% (77/79), frontend 100% (3/3); artifacts present |
| Frontend vitest (src/components/crowdsec) and full coverage run | PASS (11/11 touched; full run exit 0) |
| Frontend `npm run type-check` | PASS |
| ESLint src/components/crowdsec | PASS, 0 errors, 9 warnings (unicorn style, in untouched ListErrorNote.tsx and HubItemsRestorePanel test) |
| markdownlint (cscli-contract.md, crowdsec.md) | PASS, 0 issues |
| GORM scan | Not run (no model change) |

## Real-cscli gate (throwaway container qa6a-charon, own volume, no access to live `charon`)

1. Hub item verify with `-c`: PASS. Restore of the not-yet-installed `baudneo/gotify-logs` ran install then `VerifyHubItem` against the bundled `/app/data/crowdsec/config/config.yaml`. It returned installed with no failures, and `cscli -c ... parsers inspect` reports installed true, tainted false.
2. Index-window: PASS. A typed pull then an untyped pull of the same slug caused 1 "hub index fetched" in total. A new slug's first untyped pull fetched once (count 1 to 2), and the 2nd and 3rd untyped pulls made no fetch. `barnoux/caddy-coraza` (collections and parsers) still returned 409 `preset_type_ambiguous` with `types:["collections","parsers"]` on each untyped call, even with the parsers entry cached. Those calls refetch the index by design.
3. Top IPs: PASS. After seeding four `security_decisions` rows, `/dashboard/top-ips` returned RFC3339 `last_seen` (for example `2026-10-18T06:20:11Z`) for both 24h and 7d, with correct counts and country.
4. Config redaction: PASS. The bundled default `config.yaml` exports byte-identical (nothing in it needs redaction). Import of the export returned 200, and a re-export was identical (round-trip OK). A synthetic `config.yaml.local` was redacted for password, user, registration token, dsn, api_key, signing_key, passphrase, encryption_key, connection_string, pwd, psk, bearer and authorization. Benign keys stayed intact: *_path, key_file, cert_file, token_ttl, max_tokens, tokens_per_minute, max_open_conns, flush.*, enable_key_rotation, key_rotation_days, cache_size, log_level.
5. Acquisition backups: PASS. 7 PUTs to `/admin/crowdsec/acquisition`, spaced over 1 s apart, left exactly 5 `acquis.yaml.backup.*` files.

## Findings

- Minor 1: Backup names use one-second timestamps. Two acquisition updates inside the same second reuse the same backup name, so the earlier backup is overwritten. This is pre-existing and does not affect the prune count. I spaced the test updates over 1 s apart to avoid it.
- Minor 2: `access_key_id` is not redacted. It ends in `_id`, not `access_key`. It is an identifier rather than a secret, so this is probably intended, but worth a conscious decision.
- Minor 3 (informational): `cscli ... inspect` without `-c` also reports installed in this image. `/etc/crowdsec` seems to share the same hub, so gate 1 proves `-c` runs and succeeds but cannot show the old behaviour failing here. Unit tests (`cscli_config_flag_test.go`) cover the flag being passed.
- Minor 4: The sub-second part of `last_seen` is truncated (second precision). This is acceptable.

## Cleanup

Container, image and volume `qa6a-*` were removed and temp files in /var/tmp deleted. The live `charon` container was untouched. `changelog.json` is unchanged and `git status` is clean. Nothing was staged or committed, and only this file was written.
