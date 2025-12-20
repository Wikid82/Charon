---
# agentskills.io specification v1.0
name: "integration-test-crowdsec-startup"
version: "1.0.0"
description: "Test CrowdSec startup sequence, initialization, and error handling"
author: "Charon Project"
license: "MIT"
tags:
  - "integration"
  - "crowdsec"
  - "startup"
  - "initialization"
  - "resilience"
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
  - name: "STARTUP_TIMEOUT"
    description: "Maximum wait time for startup in seconds"
    default: "60"
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
    description: "Startup test results"
metadata:
  category: "integration-test"
  subcategory: "startup"
  execution_time: "medium"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Integration Test CrowdSec Startup

## Overview

Tests the CrowdSec startup sequence and initialization process. This skill validates that CrowdSec components (LAPI, bouncer) start correctly, handle initialization errors gracefully, and recover from common startup failures. It ensures the system is resilient to network issues, configuration problems, and timing-related edge cases.

Proper startup behavior is critical for production deployments and automated container orchestration.

## Prerequisites

- Docker 24.0 or higher installed and running
- curl 7.0 or higher for health checks
- Docker Compose for orchestration
- Network connectivity for pulling images

## Usage

### Basic Usage

Run CrowdSec startup tests:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh integration-test-crowdsec-startup
```

### Verbose Mode

Run with detailed startup logging:

```bash
VERBOSE=1 .github/skills/scripts/skill-runner.sh integration-test-crowdsec-startup
```

### Custom Timeout

Run with extended startup timeout:

```bash
STARTUP_TIMEOUT=120 .github/skills/scripts/skill-runner.sh integration-test-crowdsec-startup
```

### CI/CD Integration

For use in GitHub Actions workflows:

```yaml
- name: Test CrowdSec Startup
  run: .github/skills/scripts/skill-runner.sh integration-test-crowdsec-startup
  timeout-minutes: 5
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| verbose   | boolean | No    | false   | Enable verbose output |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| STARTUP_TIMEOUT | No | 60 | Maximum wait for startup (seconds) |
| SKIP_CLEANUP | No | false | Skip container cleanup after tests |
| CROWDSEC_VERSION | No | latest | CrowdSec image version to test |

## Outputs

### Success Exit Code
- **0**: All startup tests passed

### Error Exit Codes
- **1**: One or more tests failed
- **2**: Startup timeout exceeded
- **3**: Configuration errors detected
- **4**: Health check failed

### Console Output
Example output:
```
=== Testing CrowdSec Startup Sequence ===
✓ LAPI Initialization: Ready in 8s
✓ Database Migration: Successful
✓ Bouncer Registration: Successful
✓ Configuration Validation: No errors
✓ Health Check: All services healthy
✓ Graceful Shutdown: Clean exit
✓ Restart Resilience: Fast recovery

All CrowdSec startup tests passed!
```

## Test Coverage

This skill validates:

1. **Clean Startup**:
   - LAPI starts and becomes ready
   - Database schema migration
   - Configuration loading
   - API endpoint availability

2. **Bouncer Initialization**:
   - Bouncer registers with LAPI
   - API key generation/validation
   - Decision cache initialization
   - First sync successful

3. **Error Handling**:
   - Invalid configuration detection
   - Missing database handling
   - Network timeout recovery
   - Retry mechanisms

4. **Edge Cases**:
   - LAPI not ready on first attempt
   - Race conditions in initialization
   - Concurrent bouncer registrations
   - Configuration hot-reload

5. **Resilience**:
   - Graceful shutdown
   - Fast restart (warm start)
   - State persistence
   - No resource leaks

## Examples

### Example 1: Basic Execution

```bash
.github/skills/scripts/skill-runner.sh integration-test-crowdsec-startup
```

### Example 2: Extended Timeout

```bash
STARTUP_TIMEOUT=180 VERBOSE=1 \
  .github/skills/scripts/skill-runner.sh integration-test-crowdsec-startup
```

### Example 3: Test Specific Version

```bash
CROWDSEC_VERSION=v1.5.0 \
  .github/skills/scripts/skill-runner.sh integration-test-crowdsec-startup
```

### Example 4: Keep Containers for Debugging

```bash
SKIP_CLEANUP=true \
  .github/skills/scripts/skill-runner.sh integration-test-crowdsec-startup
```

## Startup Sequence Verified

1. **Phase 1: Container Start** (0-5s)
   - Container created and started
   - Entrypoint script execution
   - Environment variable processing

2. **Phase 2: LAPI Initialization** (5-15s)
   - Database connection established
   - Schema migration/validation
   - Configuration parsing
   - API server binding

3. **Phase 3: Bouncer Registration** (15-25s)
   - Bouncer discovers LAPI
   - API key generated/validated
   - Initial decision sync
   - Cache population

4. **Phase 4: Ready State** (25-30s)
   - Health check endpoint responds
   - All components initialized
   - Ready to process requests

## Error Handling

### Common Errors

#### Error: Startup timeout exceeded
**Solution**: Increase STARTUP_TIMEOUT or check container logs for hangs

#### Error: Database connection failed
**Solution**: Verify database container is running and accessible

#### Error: Configuration validation failed
**Solution**: Check CrowdSec config files for syntax errors

#### Error: Port already in use
**Solution**: Stop conflicting services or change port configuration

### Debugging

- **LAPI Logs**: `docker logs $(docker ps -q -f name=crowdsec) -f`
- **Bouncer Logs**: `docker logs $(docker ps -q -f name=charon-app) | grep crowdsec`
- **Health Check**: `curl http://localhost:8080/health`
- **Database**: `docker exec crowdsec cscli machines list`

## Related Skills

- [integration-test-crowdsec](./integration-test-crowdsec.SKILL.md) - Main bouncer tests
- [integration-test-crowdsec-decisions](./integration-test-crowdsec-decisions.SKILL.md) - Decision tests
- [docker-verify-crowdsec-config](./docker-verify-crowdsec-config.SKILL.md) - Config validation

## Notes

- **Execution Time**: Medium execution (3-5 minutes)
- **Typical Startup**: 20-30 seconds for clean start
- **Warm Start**: 5-10 seconds after restart
- **Timeout Buffer**: Default timeout includes safety margin
- **Container Orchestration**: Tests applicable to Kubernetes/Docker Swarm
- **Production Ready**: Validates production deployment scenarios
- **Cleanup**: Automatically removes test containers unless SKIP_CLEANUP=true
- **Idempotency**: Safe to run multiple times consecutively

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project Team
**Source**: `scripts/crowdsec_startup_test.sh`
