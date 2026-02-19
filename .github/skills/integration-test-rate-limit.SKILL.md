---
# agentskills.io specification v1.0
name: "integration-test-rate-limit"
version: "1.0.0"
description: "Run rate limit integration tests aligned with the CI rate-limit workflow. Use to validate 200/429 behavior and reset windows."
author: "Charon Project"
license: "MIT"
tags:
  - "integration"
  - "security"
  - "rate-limit"
  - "throttling"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "docker"
    version: ">=24.0"
    optional: false
  - name: "curl"
    version: ">=7.0"
    optional: false
environment_variables:
  - name: "RATE_LIMIT_REQUESTS"
    description: "Requests allowed per window in the test"
    default: "3"
    required: false
parameters:
  - name: "verbose"
    type: "boolean"
    description: "Enable verbose output"
    default: "false"
    required: false
outputs:
  - name: "test_results"
    type: "stdout"
    description: "Rate limit integration test results"
metadata:
  category: "integration-test"
  subcategory: "rate-limit"
  execution_time: "medium"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Integration Test Rate Limit

## Overview

Runs the rate limit integration tests. This suite validates request throttling, HTTP 429 responses, Retry-After headers, and rate limit window resets.

## Prerequisites

- Docker 24.0 or higher installed and running
- curl 7.0 or higher for HTTP testing
- Network access for pulling container images

## Usage

### Basic Usage

Run rate limit integration tests:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh integration-test-rate-limit
```

### Verbose Mode

```bash
VERBOSE=1 .github/skills/scripts/skill-runner.sh integration-test-rate-limit
```

### CI/CD Integration

```yaml
- name: Run Rate Limit Integration
  run: .github/skills/scripts/skill-runner.sh integration-test-rate-limit
  timeout-minutes: 7
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| verbose   | boolean | No    | false   | Enable verbose output |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| RATE_LIMIT_REQUESTS | No | 3 | Allowed requests per window in the test |
| RATE_LIMIT_WINDOW_SEC | No | 10 | Window size in seconds |
| RATE_LIMIT_BURST | No | 1 | Burst size in tests |

## Outputs

### Success Exit Code
- **0**: All rate limit integration tests passed

### Error Exit Codes
- **1**: One or more tests failed
- **2**: Docker environment setup failed
- **3**: Container startup timeout

## Related Skills

- [integration-test-all](./integration-test-all.SKILL.md) - Full integration suite
- [integration-test-cerberus](./integration-test-cerberus.SKILL.md) - Cerberus full stack tests

## Notes

- **Execution Time**: Medium execution (3-5 minutes typical)
- **CI Parity**: Matches the rate limit integration workflow entrypoint

---

**Last Updated**: 2026-02-07
**Maintained by**: Charon Project Team
**Source**: `scripts/rate_limit_integration.sh`
