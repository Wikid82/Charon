---
# agentskills.io specification v1.0
name: "integration-test-waf"
version: "1.0.0"
description: "Test generic WAF integration behavior"
author: "Charon Project"
license: "MIT"
tags:
  - "integration"
  - "waf"
  - "security"
  - "testing"
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
  - name: "WAF_MODE"
    description: "Override WAF mode (monitor or block)"
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
    description: "WAF integration test results"
metadata:
  category: "integration-test"
  subcategory: "waf"
  execution_time: "medium"
  risk_level: "medium"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Integration Test WAF

## Overview

Tests the generic WAF integration behavior using the legacy WAF script. This test is kept for local verification and is not the CI WAF entrypoint (Coraza is the CI path).

## Prerequisites

- Docker 24.0 or higher installed and running
- curl 7.0 or higher for API testing

## Usage

Run the WAF integration tests:

.github/skills/scripts/skill-runner.sh integration-test-waf

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| verbose | boolean | No | false | Enable verbose output |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| WAF_MODE | No | (script default) | Override WAF mode |

## Outputs

### Success Exit Code
- 0: All WAF integration tests passed

### Error Exit Codes
- 1: One or more tests failed
- 2: Docker environment setup failed
- 3: Container startup timeout

## Test Coverage

This skill validates:

1. WAF blocking behavior for common payloads
2. Allowed requests succeed

---

**Last Updated**: 2026-02-07
**Maintained by**: Charon Project Team
**Source**: `scripts/waf_integration.sh`
