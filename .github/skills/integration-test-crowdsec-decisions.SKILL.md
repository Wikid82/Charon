---
# agentskills.io specification v1.0
name: "integration-test-crowdsec-decisions"
version: "1.0.0"
description: "Test CrowdSec decision API for creating, retrieving, and removing IP blocks"
author: "Charon Project"
license: "MIT"
tags:
  - "integration"
  - "crowdsec"
  - "decisions"
  - "api"
  - "blocking"
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
  - name: "jq"
    version: ">=1.6"
    optional: false
environment_variables:
  - name: "CROWDSEC_API_KEY"
    description: "CrowdSec API key for decision management"
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
    description: "Decision API test results"
metadata:
  category: "integration-test"
  subcategory: "api"
  execution_time: "medium"
  risk_level: "medium"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Integration Test CrowdSec Decisions

## Overview

Tests the CrowdSec decision API functionality for managing IP block decisions. This skill validates decision creation, retrieval, persistence, expiration, and removal through the CrowdSec Local API (LAPI). It ensures the decision lifecycle works correctly and that bouncers receive updates in real-time.

Decisions are the core mechanism CrowdSec uses to communicate threats between detectors and enforcers.

## Prerequisites

- Docker 24.0 or higher installed and running
- curl 7.0 or higher for API testing
- jq 1.6 or higher for JSON parsing
- Running CrowdSec LAPI container
- Valid CrowdSec API credentials

## Usage

### Basic Usage

Run CrowdSec decision API tests:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh integration-test-crowdsec-decisions
```

### Verbose Mode

Run with detailed API request/response logging:

```bash
VERBOSE=1 .github/skills/scripts/skill-runner.sh integration-test-crowdsec-decisions
```

### CI/CD Integration

For use in GitHub Actions workflows:

```yaml
- name: Test CrowdSec Decision API
  run: .github/skills/scripts/skill-runner.sh integration-test-crowdsec-decisions
  timeout-minutes: 5
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| verbose   | boolean | No    | false   | Enable verbose output |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| CROWDSEC_API_KEY | No | auto | API key for LAPI access |
| CROWDSEC_LAPI_URL | No | http://crowdsec:8080 | CrowdSec LAPI endpoint |
| TEST_IP | No | 192.0.2.1 | Test IP address for decisions |

## Outputs

### Success Exit Code
- **0**: All decision API tests passed

### Error Exit Codes
- **1**: One or more tests failed
- **2**: LAPI not accessible
- **3**: Authentication failed
- **4**: Decision creation/deletion failed

### Console Output
Example output:
```
=== Testing CrowdSec Decision API ===
✓ Create Decision: IP 192.0.2.1 blocked for 4h
✓ Retrieve Decisions: 1 active decision found
✓ Decision Details: Correct scope, value, duration
✓ Decision Persistence: Survives bouncer restart
✓ Decision Expiration: Expires after duration
✓ Remove Decision: Successfully deleted
✓ Decision Cleanup: No orphaned decisions

All CrowdSec decision API tests passed!
```

## Test Coverage

This skill validates:

1. **Decision Creation**:
   - Create IP ban decision via API
   - Create range ban decision
   - Create captcha decision
   - Set custom duration and reason

2. **Decision Retrieval**:
   - List all active decisions
   - Filter by scope (ip, range, country)
   - Filter by value (specific IP)
   - Pagination support

3. **Decision Persistence**:
   - Decisions survive LAPI restart
   - Decisions sync to bouncers
   - Database integrity

4. **Decision Lifecycle**:
   - Expiration after duration
   - Manual removal via API
   - Automatic cleanup of expired decisions

5. **Decision Synchronization**:
   - Bouncer receives new decisions
   - Bouncer updates on decision changes
   - Real-time propagation

## Examples

### Example 1: Basic Execution

```bash
.github/skills/scripts/skill-runner.sh integration-test-crowdsec-decisions
```

### Example 2: Test Specific IP

```bash
TEST_IP=10.0.0.1 \
  .github/skills/scripts/skill-runner.sh integration-test-crowdsec-decisions
```

### Example 3: Custom LAPI URL

```bash
CROWDSEC_LAPI_URL=https://crowdsec-lapi.example.com \
  .github/skills/scripts/skill-runner.sh integration-test-crowdsec-decisions
```

### Example 4: Verbose with API Key

```bash
CROWDSEC_API_KEY=my-api-key VERBOSE=1 \
  .github/skills/scripts/skill-runner.sh integration-test-crowdsec-decisions
```

## API Endpoints Tested

- `POST /v1/decisions` - Create new decision
- `GET /v1/decisions` - List decisions
- `GET /v1/decisions/:id` - Get decision details
- `DELETE /v1/decisions/:id` - Remove decision
- `GET /v1/decisions/stream` - Bouncer decision stream

## Error Handling

### Common Errors

#### Error: LAPI not responding
**Solution**: Check LAPI container: `docker ps | grep crowdsec`

#### Error: Authentication failed
**Solution**: Verify API key: `docker exec crowdsec cscli machines list`

#### Error: Decision not created
**Solution**: Check LAPI logs for validation errors

#### Error: Decision not found after creation
**Solution**: Verify database connectivity and permissions

### Debugging

- **LAPI Logs**: `docker logs $(docker ps -q -f name=crowdsec)`
- **Database**: `docker exec crowdsec cscli decisions list`
- **API Testing**: `curl -H "X-Api-Key: $KEY" http://localhost:8080/v1/decisions`
- **Decision Details**: `docker exec crowdsec cscli decisions list -o json | jq`

## Related Skills

- [integration-test-crowdsec](./integration-test-crowdsec.SKILL.md) - Main bouncer tests
- [integration-test-crowdsec-startup](./integration-test-crowdsec-startup.SKILL.md) - Startup tests
- [integration-test-all](./integration-test-all.SKILL.md) - Complete suite

## Notes

- **Execution Time**: Medium execution (3-5 minutes)
- **Decision Types**: Supports ban, captcha, and throttle decisions
- **Scopes**: IP, range, country, AS, user
- **Duration**: From seconds to permanent bans
- **API Version**: Tests LAPI v1 endpoints
- **Cleanup**: All test decisions are removed after execution
- **Idempotency**: Safe to run multiple times
- **Isolation**: Uses test IP ranges (RFC 5737)

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project Team
**Source**: `scripts/crowdsec_decision_integration.sh`
