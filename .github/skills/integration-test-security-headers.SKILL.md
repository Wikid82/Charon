---
# agentskills.io specification v1.0
name: "integration-test-security-headers"
version: "1.0.0"
description: "Run security header profile integration tests aligned with the CI security-headers workflow. Use to validate that a profile emits each header exactly once and that upstream headers pass through correctly."
author: "Charon Project"
license: "MIT"
tags:
  - "integration"
  - "security"
  - "security-headers"
  - "caddy"
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
  - name: "CHARON_IMAGE"
    description: "Charon image to test"
    default: "charon:local"
    required: false
  - name: "SH_PREFIX"
    description: "Name prefix for the containers, network and volumes the test creates"
    default: "secheaders"
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
    description: "Security header integration test results"
metadata:
  category: "integration-test"
  subcategory: "security-headers"
  execution_time: "short"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Integration Test Security Headers

## Overview

Runs the security header profile integration tests against a real Caddy. An upstream that already sends `X-Content-Type-Options` and `Cross-Origin-Resource-Policy` is proxied through a host with a security header profile. The suite validates that:

- each profile-controlled header reaches the client exactly once, with the profile's value (HEAD and GET)
- headers the profile does not control (a custom upstream header) pass through
- a host with no profile and headers disabled leaves upstream headers untouched
- the generated Caddy config emits each response headers handler as a non-deferred + deferred pair with identical values
- streaming responses stay incremental and carry the profile headers once
- a Caddy-generated 502 (upstream down) carries the profile headers

## Prerequisites

- Docker 24.0 or higher installed and running
- curl 7.0 or higher and jq 1.6 or higher
- Network access for pulling the `busybox` upstream image
- A Charon image (`charon:local` by default, built automatically if missing)

## Usage

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh integration-test-security-headers
```

Test a specific image without touching `charon:local`:

```bash
CHARON_IMAGE=charon:my-build SH_PREFIX=sh-test .github/skills/scripts/skill-runner.sh integration-test-security-headers
```

### CI/CD Integration

```yaml
- name: Run Security Headers Integration
  run: .github/skills/scripts/skill-runner.sh integration-test-security-headers
  timeout-minutes: 7
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| CHARON_IMAGE | No | charon:local | Image under test |
| SH_PREFIX | No | secheaders | Prefix for created containers, network and volumes |
| SH_HTTP_PORT | No | 8182 | Host port for Caddy HTTP |
| SH_HTTPS_PORT | No | 8144 | Host port for Caddy HTTPS |
| SH_API_PORT | No | 8282 | Host port for the Charon API |
| SH_ADMIN_PORT | No | 2121 | Host port for the Caddy admin API |

## Outputs

### Success Exit Code
- **0**: All security header integration tests passed

### Error Exit Codes
- **1**: One or more assertions failed or the environment failed to start

## Related Skills

- [integration-test-all](./integration-test-all.SKILL.md) - Full integration suite
- [integration-test-rate-limit](./integration-test-rate-limit.SKILL.md) - Rate limit integration tests

## Notes

- **Execution Time**: Short (about 1 minute plus image pull)
- **Cleanup**: The script removes only the resources it created, on success or failure
- **CI Parity**: Matches the `security-headers` job in `.github/workflows/integration-tests.yml`

---

**Last Updated**: 2026-09-30
**Maintained by**: Charon Project Team
**Source**: `scripts/security_headers_integration.sh`
