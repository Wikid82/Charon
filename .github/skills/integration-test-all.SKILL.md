---
# agentskills.io specification v1.0
name: "integration-test-all"
version: "1.0.0"
description: "Run the canonical integration tests aligned with CI workflows, covering Cerberus, Coraza WAF, CrowdSec bouncer/decisions/startup, and rate limiting. Use when you need local parity with CI integration runs."
author: "Charon Project"
license: "MIT"
tags:
  - "integration"
  - "testing"
  - "docker"
  - "end-to-end"
  - "security"
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
  - name: "docker-compose"
    version: ">=2.0"
    optional: false
  - name: "curl"
    version: ">=7.0"
    optional: false
environment_variables:
  - name: "DOCKER_BUILDKIT"
    description: "Enable Docker BuildKit for faster builds"
    default: "1"
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
    description: "Aggregated test results from all integration tests"
metadata:
  category: "integration-test"
  subcategory: "all"
  execution_time: "long"
  risk_level: "medium"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Integration Test All

## Overview

Executes the integration test suite for the Charon project aligned with CI workflows. This skill runs Cerberus full-stack, Coraza WAF, CrowdSec bouncer/decisions/startup, and rate limiting integration tests. It validates the core security stack in a containerized environment.

This is the comprehensive test suite that ensures all components work together correctly before deployment.

## Prerequisites

- Docker 24.0 or higher installed and running
- Docker Compose 2.0 or higher
- curl 7.0 or higher for API testing
- At least 4GB of available RAM for containers
- Network access for pulling container images
- Docker daemon running with sufficient disk space

## Usage

### Basic Usage

Run all integration tests:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh integration-test-all
```

### Verbose Mode

Run with detailed output:

```bash
VERBOSE=1 .github/skills/scripts/skill-runner.sh integration-test-all
```

### CI/CD Integration

For use in GitHub Actions workflows:

```yaml
- name: Run All Integration Tests
  run: .github/skills/scripts/skill-runner.sh integration-test-all
  timeout-minutes: 20
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| verbose   | boolean | No    | false   | Enable verbose output |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| DOCKER_BUILDKIT | No | 1 | Enable BuildKit for faster builds |
| SKIP_CLEANUP | No | false | Skip container cleanup after tests |
| TEST_TIMEOUT | No | 300 | Timeout in seconds for each test |

## Outputs

### Success Exit Code
- **0**: All integration tests passed

### Error Exit Codes
- **1**: One or more tests failed
- **2**: Docker environment setup failed
- **3**: Container startup timeout
- **4**: Network connectivity issues

### Console Output
Example output:
```
=== Running Integration Test Suite ===
✓ Cerberus Integration Tests
✓ Coraza WAF Integration Tests
✓ CrowdSec Bouncer Integration Tests
✓ CrowdSec Decision Tests
✓ CrowdSec Startup Tests
✓ Rate Limiting Tests

All integration tests passed!
```

## Examples

### Example 1: Basic Execution

```bash
.github/skills/scripts/skill-runner.sh integration-test-all
```

### Example 2: Verbose with Custom Timeout

```bash
VERBOSE=1 TEST_TIMEOUT=600 .github/skills/scripts/skill-runner.sh integration-test-all
```

### Example 3: Skip Cleanup for Debugging

```bash
SKIP_CLEANUP=true .github/skills/scripts/skill-runner.sh integration-test-all
```

### Example 4: CI/CD Pipeline

```bash
# Run with specific Docker configuration
DOCKER_BUILDKIT=1 .github/skills/scripts/skill-runner.sh integration-test-all
```

## Test Coverage

This skill executes the following test suites:

1. **Cerberus Tests**: WAF + rate limit + handler order checks
2. **Coraza WAF Tests**: SQL injection, XSS, path traversal detection
3. **CrowdSec Bouncer Tests**: IP blocking, decision synchronization
4. **CrowdSec Decision Tests**: Decision API lifecycle
5. **CrowdSec Startup Tests**: LAPI and bouncer startup validation
6. **Rate Limit Tests**: Request throttling, burst handling

## Error Handling

### Common Errors

#### Error: Cannot connect to Docker daemon
**Solution**: Ensure Docker is running: `sudo systemctl start docker`

#### Error: Port already in use
**Solution**: Stop conflicting services or run cleanup: `docker compose down`

#### Error: Container startup timeout
**Solution**: Check Docker logs: `docker compose logs`

#### Error: Network connectivity issues
**Solution**: Verify network configuration: `docker network ls`

### Troubleshooting

- **Slow execution**: Check available system resources
- **Random failures**: Increase TEST_TIMEOUT
- **Cleanup issues**: Manually run `docker compose down -v`

## Related Skills

- [integration-test-cerberus](./integration-test-cerberus.SKILL.md) - Cerberus full stack tests
- [integration-test-coraza](./integration-test-coraza.SKILL.md) - Coraza WAF tests only
- [integration-test-crowdsec](./integration-test-crowdsec.SKILL.md) - CrowdSec tests only
- [integration-test-crowdsec-decisions](./integration-test-crowdsec-decisions.SKILL.md) - Decision API tests
- [integration-test-crowdsec-startup](./integration-test-crowdsec-startup.SKILL.md) - Startup tests
- [integration-test-rate-limit](./integration-test-rate-limit.SKILL.md) - Rate limit tests

## Notes

- **Execution Time**: Long execution (10-15 minutes typical)
- **Resource Intensive**: Requires significant CPU and memory
- **Network Required**: Pulls Docker images and tests network functionality
- **Idempotency**: Safe to run multiple times (cleanup between runs)
- **Cleanup**: Automatically cleans up containers unless SKIP_CLEANUP=true
- **CI/CD**: Designed for automated pipelines with proper timeout configuration
- **Isolation**: Tests run in isolated Docker networks

---

**Last Updated**: 2026-02-07
**Maintained by**: Charon Project Team
**Source**: `scripts/integration-test-all.sh`
