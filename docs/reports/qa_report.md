# QA Report: #1644 CrowdSec process tracking fix

Branch: `fix/crowdsec-process-tracking` (2 commits over `development`). Date: 2026-10-18.

## Gate results

| Gate | Result | Detail |
|---|---|---|
| `go build ./...` | PASS | clean |
| `go test -race -timeout 60m ./...` | PASS | 42 packages ok, no FAIL, no data races |
| golangci-lint (`backend/.golangci.yml`) | PASS | 0 issues |
| golangci-lint (`.golangci-fast.yml`) | PASS | 0 issues |
| Backend coverage (`scripts/go-test-coverage.sh`) | PASS | 92.1% statements, 91.0% lines (gate 87%) |
| Local patch report | PASS | backend patch 93.2% (82/88). Uncovered: `crowdsec_exec.go` 66-67, 126-127, 179, 183 (error branches). The script needs `frontend/coverage/lcov.info`; an empty temporary file was used and removed (backend-only change). |
| Real-process gate (container built from branch, fresh volume) | PASS | see below |

## Real-process gate (throwaway container, all artifacts deleted)

| Check | Result |
|---|---|
| Clean start | PASS: `started`, pid 468, pid file 468, status running |
| Duplicate Start | PASS: HTTP 200 `already_running`, pid 468, pid file still 468, engine untouched |
| Status | PASS: `running:true` with live pid |
| Stop | PASS: process gone, pid file removed, status `running:false` |
| Engine started via `docker exec` (no pid file) | PASS: Status adopts it (pid 588, pid file rewritten), Stop kills it and removes the file |
| Container restart then Start | PASS: boot re-launched the engine, Start returned `already_running`; Stop then Start gave a fresh engine with correct pid file |
| Stale pid file pointing at unrelated live `sleep` (no engine) | PASS: Status `running:false`, Stop did not signal `sleep` (still alive), stale file removed |
| Same stale file while an untracked engine runs | PASS: engine adopted and stopped, `sleep` untouched |

Note: JWTs are invalidated by a container restart, so a re-login was needed. This is existing behaviour and unrelated to the fix.

## /proc scan security review

`findProcess` walks `/proc`, requires `cmdline` to contain `crowdsec` and one argument to equal exactly `<configDir>/config/config.yaml`. It does not follow symlinks or read beyond `cmdline`, and `procPath` is a trusted constant. A process using a different config dir is not matched (verified with `other.yaml`).

## Findings

### Blocker
None.

### Major
None.

### Minor
1. **Process match is too loose (verified).** `isCrowdSecProcess` only checks that the cmdline contains the substring `crowdsec` anywhere, and the config path itself (`.../crowdsec/config/config.yaml`) contains it. Any process that has that config file as an argument is therefore adopted by Status and SIGTERM/SIGKILLed by Stop. Reproduced with `tail -f /app/data/crowdsec/config/config.yaml`: Status reported `running:true` with its pid, and Stop killed it. Exploitability is low, since it needs the ability to exec in the container as the same user, but it can kill an unrelated helper (editor, `cat`, a backup tool). Suggested fix in `crowdsec_exec.go`: require `filepath.Base(args[0]) == "crowdsec"` (or `args[0]` ending in `/crowdsec`) in `findProcess`, and ideally in `isCrowdSecProcess` too.
2. **Start has a check-then-act race.** Two concurrent Start requests can both see `running:false` and both spawn, which recreates the original symptom, though only for simultaneous calls. Suggested fix: serialise Start/Stop with a mutex on the handler or executor.
3. **Uncovered error branches** (patch report lines above) are low risk. Optional extra tests.

No GORM, model or migration changes, so the GORM scan does not apply. This is a `fix:`, so local CodeQL/Trivy were skipped and deferred to CI per policy.

## Cleanup
Container `qa1644-c`, volume `qa1644-vol` and image `qa1644-img:test` were removed, and temp files deleted. Container `charon` was not touched (still up/healthy). The git working tree is clean apart from this report, which is uncommitted.

## Verdict
Mergeable. Minor 1 is worth a quick follow-up in this PR since it is a one-line tightening.
