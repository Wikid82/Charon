# QA / Security Report: PR 5 (`fix/crowdsec-robustness-batch`, 15 commits)

Date: 2026-10-17. Scope: CrowdSec exports, imports, presets, alerts, bans (spec rev 8, batch 5).

## Gate results

| Gate | Result |
|---|---|
| Backend `go build ./...` + `go test -race -timeout 60m ./...` | PASS (all packages ok) |
| golangci-lint (`backend/.golangci.yml`, `.golangci-fast.yml`) | PASS (0 issues each) |
| GORM scan | N/A (no models/migrations changed) |
| Backend coverage script | PASS 92.0% statements / 91.0% lines (min 87%) |
| Frontend `type-check` | PASS |
| Frontend `build` | PASS (first attempt hit an `npm ci` race: "vite: not found"; rerun clean) |
| Frontend coverage script | PASS lines 91.66% (min 87%) |
| `scripts/local-patch-report.sh` | PASS overall 92.2% (backend 91.8%, frontend 98.5%). Lowest: `hub_snapshot.go` 75.7% (error branches), `backup.go` 76.9% |
| Playwright `crowdsec-decisions.spec.ts --project=security-tests` | PASS 27/27 (changelog.json restored; tree clean) |
| CodeQL Go + JS (CLI 2.27.2, no reinstall needed) | PASS 0 error/warning results. Go has 4 pre-existing notes (`go/cookie-secure-not-set`, `go/log-injection` x3) in files untouched by this PR. Generated db/sarif removed |
| Real-cscli gate (v1.8.2-rc1, throwaway container) | PARTIAL, see below |

## Real-cscli gate

| Check | Result |
|---|---|
| `alerts --since` with an old (3 d) and a new alert | PASS: 1h/24h return only the new one, 7d/30d both; `limit=1&offset=1` returns the older; `total` = fetched count |
| Export with postgres and mysql `db_config` | PASS: user/password replaced by placeholders, no fixture value anywhere in the archive; special-char password (`$ : #`) handled |
| Same-host import restores values | PASS: host's current values kept, `cscli config show` parses them; import onto a host without them returns `needs_values:["config.yaml"]` |
| Offline apply | PASS: 503 `hub_unreachable`, translated text, tree restored (hub file list identical, no leftover cscli) |
| Offline pull | FAIL (see M1) |
| Pull of name shared by several sections (`crowdsecurity/crs`) | PASS: 409 `preset_type_ambiguous` with `types`; pull with type works; apply with the cache key (and with explicit type) installs `appsec-rules` |
| Light snapshot with ~93 MB hub (97.5 MB) | PASS: snapshot dir ~0.5 MB (full import snapshot of same tree 98 MB, 2.9 s). Apply 0.55 s without filler vs 2.1 s with it; the increase is cscli's own hub scan (`cscli hub list` 169 ms -> 796 ms with the filler), not Charon's snapshot. Direct snapshot-only timing not measured (would need a Go test inside the package; app code not modified) |
| Rollback leaves no orphans | PASS: forced via `HUB_APPLY_TIMEOUT_SECONDS=3` on `sigmahq/windows_proc_creation`: 504 `preset_apply_timeout`; hub file and dir lists, non-hub config checksums and `cscli hub list` output identical before/after; no cscli processes left |
| List errors show codes | PASS: LAPI down gives 200 with `error_code:"cscli_timeout"` for decisions and alerts (30 s) |

## Findings

### M1 (major): offline pull returns a generic 502 instead of `hub_unreachable` in the normal state

- Where: `backend/internal/crowdsec/hub_sync.go:572-586` (`Pull`) with `FetchIndex` (:265-283).
- Evidence: container with no network but with the hub previously fetched (the usual state): pull of an uncached slug returns `502 {"error":"hub index has no download location for this preset (index unreachable)","hub_endpoints":[...]}`, no `error_code`. `FetchIndex` falls back to the local cscli index, so it does not return `ErrHubUnreachable`; `Pull` then sees `idx.offline` with an empty `DownloadURL` and returns a plain error. `ErrHubUnreachable` is only produced when the cscli fallback also fails (what `hub_unreachable_test.go` covers).
- Impact: the spec acceptance line "offline pull/apply answer `hub_unreachable` with a translated message" is not met for pull; users see untranslated English text. Apply is fine.
- Fix: in `Pull`, when `idx.offline && entry.DownloadURL == ""` return an error wrapping `ErrHubUnreachable` (and add a test with a working cscli fallback).
- (Cached presets still pull fine offline, e.g. `crowdsecurity/crs` was served from the cache.)

### Minor / notes

- m1: `hub_snapshot.go` patch coverage 75.7% (error branches of the sidecar writer/manifest walker); acceptable, add error-injection tests if cheap.
- m2 (not in this PR's scope, CrowdSec): in my throwaway container the API `stop` reported `stopped` while the entrypoint-started crowdsec process kept running, and `start` reported a different pid. Likely an artifact of the container restart with persisted config (process not tracked by the handler); worth a look when touching lifecycle code. No issue filed (CrowdSec items go to the spec).
- m3: backup directories (`crowdsec.backup.*`) accumulate per apply; light ones are ~0.5 MB, full import ones ~98 MB at this hub size. Existing retention behavior, unchanged.
- Test harness notes: login is rate limited (retry after ~60 s) and a request whose client half-closes the connection cancels the request context (made my first offline-pull probe misleading; redone with the connection held open).

## Verdict

No blockers and no security findings. One major functional gap (M1) against the stated acceptance criteria; everything else passes. Cleanup done: throwaway container, volume, network and image tag removed; working tree clean (`git status` empty); only this file is new (uncommitted).
