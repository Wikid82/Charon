---
# agentskills.io specification v1.0
name: "integration-test-cerberus"
version: "1.0.0"
description: "Run Cerberus full-stack integration tests (WAF + rate limit + handler order). Use for local parity with CI Cerberus workflow."
author: "Charon Project"
license: "MIT"
tags:
  - "integration"
  - "security"
  - "cerberus"
  - "waf"
  - "rate-limit"
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
  - name: "CHARON_EMERGENCY_TOKEN"
    description: "Emergency token required for some Cerberus teardown flows"
    default: ""
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
    description: "Cerberus integration test results"
metadata:
  category: "integration-test"
  subcategory: "cerberus"
  execution_time: "medium"
  risk_level: "medium"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Integration Test Cerberus

## Overview

Runs the Cerberus full-stack integration tests. This suite validates handler order, WAF enforcement, rate limiting behavior, and end-to-end request flow in a containerized environment.

## Prerequisites

- Docker 24.0 or higher installed and running
- curl 7.0 or higher for HTTP testing
- Network access for pulling container images

## Usage

### Basic Usage

Run Cerberus integration tests:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh integration-test-cerberus
```

### Verbose Mode

```bash
VERBOSE=1 .github/skills/scripts/skill-runner.sh integration-test-cerberus
```

### CI/CD Integration

```yaml
- name: Run Cerberus Integration
  run: .github/skills/scripts/skill-runner.sh integration-test-cerberus
  timeout-minutes: 10
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| verbose   | boolean | No    | false   | Enable verbose output |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| CHARON_EMERGENCY_TOKEN | No | (empty) | Emergency token for Cerberus teardown flows |
| SKIP_CLEANUP | No | false | Skip container cleanup after tests |
| TEST_TIMEOUT | No | 600 | Timeout in seconds for the test |

## Outputs

### Success Exit Code
- **0**: All Cerberus integration tests passed

### Error Exit Codes
- **1**: One or more tests failed
- **2**: Docker environment setup failed
- **3**: Container startup timeout

## Related Skills

- [integration-test-all](./integration-test-all.SKILL.md) - Full integration suite
- [integration-test-coraza](./integration-test-coraza.SKILL.md) - Coraza WAF tests
- [integration-test-rate-limit](./integration-test-rate-limit.SKILL.md) - Rate limit tests

## Notes

- **Execution Time**: Medium execution (5-10 minutes typical)
- **CI Parity**: Matches the Cerberus integration workflow entrypoint

---

**Last Updated**: 2026-02-07
**Maintained by**: Charon Project Team
**Source**: `scripts/cerberus_integration.sh`
