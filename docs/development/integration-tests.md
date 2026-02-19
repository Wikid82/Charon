# Integration Tests Runbook

## Overview

This runbook describes how to run integration tests locally with the same entrypoints used in CI. It also documents the scope of each integration script, known port bindings, and the local-only Go integration tests.

## Prerequisites

- Docker 24+
- Docker Compose 2+
- curl (required by all scripts)
- jq (required by CrowdSec decisions script)

## CI-Aligned Entry Points

Local runs should follow the same entrypoints used in CI workflows.

- Cerberus full stack: `scripts/cerberus_integration.sh` (skill: `integration-test-cerberus`, wrapper: `.github/skills/integration-test-cerberus-scripts/run.sh`)
- Coraza WAF: `scripts/coraza_integration.sh` (skill: `integration-test-coraza`, wrapper: `.github/skills/integration-test-coraza-scripts/run.sh`)
- Rate limiting: `scripts/rate_limit_integration.sh` (skill: `integration-test-rate-limit`, wrapper: `.github/skills/integration-test-rate-limit-scripts/run.sh`)
- CrowdSec bouncer: `scripts/crowdsec_integration.sh` (skill: `integration-test-crowdsec`, wrapper: `.github/skills/integration-test-crowdsec-scripts/run.sh`)
- CrowdSec startup: `scripts/crowdsec_startup_test.sh` (skill: `integration-test-crowdsec-startup`, wrapper: `.github/skills/integration-test-crowdsec-startup-scripts/run.sh`)
- Run all (CI-aligned): `scripts/integration-test-all.sh` (skill: `integration-test-all`, wrapper: `.github/skills/integration-test-all-scripts/run.sh`)

## Local Execution (Preferred)

Use the skill runner to mirror CI behavior:

- `.github/skills/scripts/skill-runner.sh integration-test-all` (wrapper: `.github/skills/integration-test-all-scripts/run.sh`)
- `.github/skills/scripts/skill-runner.sh integration-test-cerberus` (wrapper: `.github/skills/integration-test-cerberus-scripts/run.sh`)
- `.github/skills/scripts/skill-runner.sh integration-test-coraza` (wrapper: `.github/skills/integration-test-coraza-scripts/run.sh`)
- `.github/skills/scripts/skill-runner.sh integration-test-rate-limit` (wrapper: `.github/skills/integration-test-rate-limit-scripts/run.sh`)
- `.github/skills/scripts/skill-runner.sh integration-test-crowdsec` (wrapper: `.github/skills/integration-test-crowdsec-scripts/run.sh`)
- `.github/skills/scripts/skill-runner.sh integration-test-crowdsec-startup` (wrapper: `.github/skills/integration-test-crowdsec-startup-scripts/run.sh`)
- `.github/skills/scripts/skill-runner.sh integration-test-crowdsec-decisions` (wrapper: `.github/skills/integration-test-crowdsec-decisions-scripts/run.sh`)
- `.github/skills/scripts/skill-runner.sh integration-test-waf` (legacy WAF path, wrapper: `.github/skills/integration-test-waf-scripts/run.sh`)

## Go Integration Tests (Local-Only)

Go integration tests under `backend/integration/` are build-tagged and are not executed by CI. To run them locally, use `go test -tags=integration ./backend/integration/...`.

## WAF Scope

- Canonical CI entrypoint: `scripts/coraza_integration.sh`
- Local-only legacy path: `scripts/waf_integration.sh` (skill: `integration-test-waf`)

## Known Port Bindings

- `scripts/cerberus_integration.sh`: API 8480, HTTP 8481, HTTPS 8444, admin 2319
- `scripts/waf_integration.sh`: API 8380, HTTP 8180, HTTPS 8143, admin 2119
- `scripts/coraza_integration.sh`: API 8080, HTTP 80, HTTPS 443, admin 2019
- `scripts/rate_limit_integration.sh`: API 8280, HTTP 8180, HTTPS 8143, admin 2119
- `scripts/crowdsec_*`: API 8280/8580, HTTP 8180/8480, HTTPS 8143/8443, admin 2119 (varies by script)
