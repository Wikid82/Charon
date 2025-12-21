---
# agentskills.io specification v1.0
name: "integration-test-coraza"
version: "1.0.0"
description: "Test Coraza WAF integration with OWASP Core Rule Set protection"
author: "Charon Project"
license: "MIT"
tags:
  - "integration"
  - "waf"
  - "security"
  - "coraza"
  - "owasp"
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
  - name: "WAF_ENABLED"
    description: "Enable WAF protection"
    default: "true"
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
    description: "WAF test results including blocked attacks"
metadata:
  category: "integration-test"
  subcategory: "waf"
  execution_time: "medium"
  risk_level: "medium"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Integration Test Coraza

## Overview

Tests the Coraza Web Application Firewall (WAF) integration with OWASP Core Rule Set (CRS). This skill validates that the WAF correctly detects and blocks common web attacks including SQL injection, cross-site scripting (XSS), remote code execution (RCE), and path traversal attempts.

Coraza provides ModSecurity-compatible rule processing with improved performance and modern Go implementation.

## Prerequisites

- Docker 24.0 or higher installed and running
- curl 7.0 or higher for HTTP testing
- Running Charon Docker environment (or automatic startup)
- Network access to test endpoints

## Usage

### Basic Usage

Run Coraza WAF integration tests:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh integration-test-coraza
```

### Verbose Mode

Run with detailed attack payloads and responses:

```bash
VERBOSE=1 .github/skills/scripts/skill-runner.sh integration-test-coraza
```

### CI/CD Integration

For use in GitHub Actions workflows:

```yaml
- name: Test Coraza WAF Integration
  run: .github/skills/scripts/skill-runner.sh integration-test-coraza
  timeout-minutes: 5
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| verbose   | boolean | No    | false   | Enable verbose output |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| WAF_ENABLED | No | true | Enable WAF protection for tests |
| TEST_HOST | No | localhost:8080 | Target host for WAF tests |

## Outputs

### Success Exit Code
- **0**: All WAF tests passed (attacks blocked correctly)

### Error Exit Codes
- **1**: One or more attacks were not blocked
- **2**: Docker environment setup failed
- **3**: WAF not responding or misconfigured

### Console Output
Example output:
```
=== Testing Coraza WAF Integration ===
✓ SQL Injection: Blocked (403 Forbidden)
✓ XSS Attack: Blocked (403 Forbidden)
✓ Path Traversal: Blocked (403 Forbidden)
✓ RCE Attempt: Blocked (403 Forbidden)
✓ Legitimate Request: Allowed (200 OK)

All Coraza WAF tests passed!
```

## Test Coverage

This skill validates protection against:

1. **SQL Injection**: `' OR '1'='1`, `UNION SELECT`, `'; DROP TABLE`
2. **Cross-Site Scripting (XSS)**: `<script>alert('XSS')</script>`, `javascript:alert(1)`
3. **Path Traversal**: `../../etc/passwd`, `....//....//etc/passwd`
4. **Remote Code Execution**: `<?php system($_GET['cmd']); ?>`, `eval()`
5. **Legitimate Traffic**: Ensures normal requests are not blocked

## Examples

### Example 1: Basic Execution

```bash
.github/skills/scripts/skill-runner.sh integration-test-coraza
```

### Example 2: Verbose with Custom Host

```bash
TEST_HOST=production.example.com VERBOSE=1 \
  .github/skills/scripts/skill-runner.sh integration-test-coraza
```

### Example 3: Disable WAF for Comparison

```bash
WAF_ENABLED=false .github/skills/scripts/skill-runner.sh integration-test-coraza
```

## Error Handling

### Common Errors

#### Error: WAF not responding
**Solution**: Verify Docker containers are running: `docker ps | grep coraza`

#### Error: Attacks not blocked (false negatives)
**Solution**: Check WAF configuration in `configs/coraza/` and rule sets

#### Error: Legitimate requests blocked (false positives)
**Solution**: Review WAF logs and adjust rule sensitivity

#### Error: Connection refused
**Solution**: Ensure application is accessible: `curl http://localhost:8080/health`

### Debugging

- **WAF Logs**: `docker logs $(docker ps -q -f name=coraza)`
- **Rule Debugging**: Set `SecRuleEngine DetectionOnly` in config
- **Test Individual Payloads**: Use curl with specific attack strings

## Related Skills

- [integration-test-all](./integration-test-all.SKILL.md) - Complete integration suite
- [integration-test-waf](./integration-test-waf.SKILL.md) - General WAF tests
- [security-scan-trivy](./security-scan-trivy.SKILL.md) - Vulnerability scanning

## Notes

- **OWASP CRS**: Uses Core Rule Set v4.0+ for comprehensive protection
- **Execution Time**: Medium execution (3-5 minutes)
- **False Positives**: Tuning required for production workloads
- **Performance**: Minimal latency impact (<5ms per request)
- **Compliance**: Helps meet OWASP Top 10 and PCI DSS requirements
- **Logging**: All blocked requests are logged for analysis
- **Rule Updates**: Regularly update CRS for latest threat intelligence

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project Team
**Source**: `scripts/coraza_integration.sh`
