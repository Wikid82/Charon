---
# agentskills.io specification v1.0
name: "integration-test-crowdsec"
version: "1.0.0"
description: "Test CrowdSec bouncer integration and IP blocking functionality"
author: "Charon Project"
license: "MIT"
tags:
  - "integration"
  - "security"
  - "crowdsec"
  - "ip-blocking"
  - "bouncer"
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
  - name: "CROWDSEC_API_KEY"
    description: "CrowdSec API key for bouncer authentication"
    default: "auto-generated"
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
    description: "CrowdSec integration test results"
metadata:
  category: "integration-test"
  subcategory: "security"
  execution_time: "medium"
  risk_level: "medium"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Integration Test CrowdSec

## Overview

Tests the CrowdSec bouncer integration for IP-based threat detection and blocking. This skill validates that the CrowdSec bouncer correctly synchronizes with the CrowdSec Local API (LAPI), retrieves and applies IP block decisions, and enforces security policies.

CrowdSec provides collaborative security with real-time threat intelligence sharing across the community.

## Prerequisites

- Docker 24.0 or higher installed and running
- curl 7.0 or higher for API testing
- Running CrowdSec LAPI container
- Running Charon application with CrowdSec bouncer enabled
- Network access between bouncer and LAPI

## Usage

### Basic Usage

Run CrowdSec bouncer integration tests:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh integration-test-crowdsec
```

### Verbose Mode

Run with detailed API interactions:

```bash
VERBOSE=1 .github/skills/scripts/skill-runner.sh integration-test-crowdsec
```

### CI/CD Integration

For use in GitHub Actions workflows:

```yaml
- name: Test CrowdSec Integration
  run: .github/skills/scripts/skill-runner.sh integration-test-crowdsec
  timeout-minutes: 7
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| verbose   | boolean | No    | false   | Enable verbose output |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| CROWDSEC_API_KEY | No | auto | Bouncer API key (auto-generated if not set) |
| CROWDSEC_LAPI_URL | No | http://crowdsec:8080 | CrowdSec LAPI endpoint |
| BOUNCER_SYNC_INTERVAL | No | 60 | Decision sync interval in seconds |

## Outputs

### Success Exit Code
- **0**: All CrowdSec integration tests passed

### Error Exit Codes
- **1**: One or more tests failed
- **2**: CrowdSec LAPI not accessible
- **3**: Bouncer authentication failed
- **4**: Decision synchronization failed

### Console Output
Example output:
```
=== Testing CrowdSec Bouncer Integration ===
✓ LAPI Connection: Successful
✓ Bouncer Authentication: Valid API Key
✓ Decision Retrieval: 5 active decisions
✓ IP Blocking: Blocked malicious IP (403 Forbidden)
✓ Legitimate IP: Allowed (200 OK)
✓ Decision Synchronization: Every 60s

All CrowdSec integration tests passed!
```

## Test Coverage

This skill validates:

1. **LAPI Connectivity**: Bouncer can reach CrowdSec Local API
2. **Authentication**: Valid API key and successful bouncer registration
3. **Decision Retrieval**: Fetching active IP block decisions
4. **IP Blocking**: Correctly blocking malicious IPs
5. **Legitimate Traffic**: Allowing non-blocked IPs
6. **Decision Synchronization**: Regular updates from LAPI
7. **Graceful Degradation**: Handling LAPI downtime

## Examples

### Example 1: Basic Execution

```bash
.github/skills/scripts/skill-runner.sh integration-test-crowdsec
```

### Example 2: Custom API Key

```bash
CROWDSEC_API_KEY=my-bouncer-key \
  .github/skills/scripts/skill-runner.sh integration-test-crowdsec
```

### Example 3: Custom LAPI URL

```bash
CROWDSEC_LAPI_URL=http://crowdsec-lapi:8080 \
  .github/skills/scripts/skill-runner.sh integration-test-crowdsec
```

### Example 4: Fast Sync Interval

```bash
BOUNCER_SYNC_INTERVAL=30 VERBOSE=1 \
  .github/skills/scripts/skill-runner.sh integration-test-crowdsec
```

## Error Handling

### Common Errors

#### Error: Cannot connect to LAPI
**Solution**: Verify LAPI container is running: `docker ps | grep crowdsec`

#### Error: Authentication failed
**Solution**: Check API key is valid: `docker exec crowdsec cscli bouncers list`

#### Error: No decisions retrieved
**Solution**: Create test decisions: `docker exec crowdsec cscli decisions add --ip 1.2.3.4`

#### Error: Blocking not working
**Solution**: Check bouncer logs: `docker logs charon-app | grep crowdsec`

### Debugging

- **LAPI Logs**: `docker logs $(docker ps -q -f name=crowdsec)`
- **Bouncer Status**: Check application logs for sync errors
- **Decision List**: `docker exec crowdsec cscli decisions list`
- **Test Block**: `curl -H "X-Forwarded-For: 1.2.3.4" http://localhost:8080/`

## Related Skills

- [integration-test-crowdsec-decisions](./integration-test-crowdsec-decisions.SKILL.md) - Decision API tests
- [integration-test-crowdsec-startup](./integration-test-crowdsec-startup.SKILL.md) - Startup tests
- [integration-test-all](./integration-test-all.SKILL.md) - Complete test suite

## Notes

- **Execution Time**: Medium execution (4-6 minutes)
- **Community Intelligence**: Benefits from CrowdSec's global threat network
- **Performance**: Minimal latency with in-memory decision caching
- **Scalability**: Tested with thousands of concurrent decisions
- **Resilience**: Continues working if LAPI is temporarily unavailable
- **Observability**: Full metrics exposed for Prometheus/Grafana
- **Compliance**: Supports GDPR-compliant threat intelligence

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project Team
**Source**: `scripts/crowdsec_integration.sh`
