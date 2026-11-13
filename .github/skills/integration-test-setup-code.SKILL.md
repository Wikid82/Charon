---
# agentskills.io specification v1.0
name: "integration-test-setup-code"
version: "1.0.0"
description: "Run first-run setup code integration tests (setup code required, throttling, one-shot bootstrap, log-delivered code). Use for local parity with the CI integration workflow."
author: "Charon Project"
license: "MIT"
tags:
  - "integration"
  - "security"
  - "setup"
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
outputs:
  - name: "test_results"
    type: "stdout"
    description: "First-run setup code test results (the setup code itself is never printed)"
metadata:
  category: "integration-test"
  subcategory: "setup"
  execution_time: "short"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: false
  idempotent: true
---

# Integration Test Setup Code

## Overview

Starts throwaway Charon containers from `charon:local` and checks the first-account bootstrap path: setup code required, 403 without or with a wrong code, 201 with the correct code, a one-shot setup, the per-client limiter (429), and delivery of a generated code through the startup log.

Each run generates its own token and unique container and volume names, and removes them on exit.

## Usage

```bash
.github/skills/scripts/skill-runner.sh integration-test-setup-code
```

Or directly:

```bash
bash scripts/setup_code_integration.sh
```

## Prerequisites

- Docker running, with the `charon:local` image (built with `docker build -t charon:local .` if absent)
- `curl`, `jq`, and `openssl`

## Exit Codes

- **0**: All checks passed
- **1**: One or more checks failed (debug output is redacted)

## Related Skills

- [integration-test-cerberus](./integration-test-cerberus.SKILL.md) - Cerberus full stack tests

---

**Last Updated**: 2026-11-13
**Maintained by**: Charon Project Team
**Source**: `scripts/setup_code_integration.sh`
