# Integration Tests: Run All

Run all Charon integration test suites.

## Command

```bash
.github/skills/scripts/skill-runner.sh integration-test-all
```

## What It Runs

All integration test suites:
- Cerberus (access control)
- Coraza WAF
- CrowdSec (decisions + startup)
- Rate limiting
- WAF rules

## Prerequisites

The E2E/integration container must be running and healthy:
```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
```

## Run Individual Suites

```bash
# Cerberus only
.github/skills/scripts/skill-runner.sh integration-test-cerberus

# WAF only
.github/skills/scripts/skill-runner.sh integration-test-waf

# CrowdSec only
.github/skills/scripts/skill-runner.sh integration-test-crowdsec

# Rate limiting only
.github/skills/scripts/skill-runner.sh integration-test-rate-limit
```

## Related

- `/test-e2e-playwright` — E2E UI tests
- `/test-backend-unit` — Backend unit tests
