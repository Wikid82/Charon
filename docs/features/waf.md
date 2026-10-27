---
title: Web Application Firewall (WAF)
description: Protect against OWASP Top 10 vulnerabilities with Coraza WAF
---

# Web Application Firewall (WAF)

Stop common attacks like SQL injection, cross-site scripting (XSS), and path traversal before they reach your applications. Powered by Coraza, the WAF protects your apps from the OWASP Top 10 vulnerabilities.

## Overview

The Web Application Firewall inspects every HTTP/HTTPS request and blocks malicious payloads before they reach your backend services. Charon uses [Coraza](https://coraza.io/), a high-performance, open-source WAF engine compatible with the OWASP Core Rule Set (CRS).

Protected attack types include:

- **SQL Injection** — Blocks database manipulation attempts
- **Cross-Site Scripting (XSS)** — Prevents script injection attacks
- **Path Traversal** — Stops directory traversal exploits
- **Remote Code Execution** — Blocks command injection
- **Zero-Day Exploits** — CRS updates provide protection against newly discovered vulnerabilities

## Why Use This

- **Defense in Depth** — Add a security layer in front of your applications
- **OWASP CRS** — Industry-standard ruleset trusted by enterprises
- **Low Latency** — Coraza processes rules efficiently with minimal overhead
- **Flexible Modes** — Choose between monitoring and active blocking

## Configuration

### Turning On the WAF

1. Open the **Security** page
2. Turn on the **Web Application Firewall** switch
3. Open the **WAF** page and pick a rule set from the list, or choose automatic selection

The WAF page shows the rule sets you have and their modes. If the WAF is on but no rule set can be used yet, the Security page says so and tells you the reason.

### Operating Modes

Each rule set has its own mode:

| Mode | Behavior | Use Case |
| --- | --- | --- |
| **Detection** | Logs threats but allows traffic | Testing rules, reducing false positives |
| **Blocking** | Actively blocks malicious requests | Production protection |

**Recommendation**: Start with Detection to review detected threats, then switch the rule set to Blocking once you're confident in the rules.

### After Upgrading

After upgrading, check the WAF page. With the WAF switched on and a rule set selected, each rule set's own mode applies: a rule set set to blocking blocks matching requests, and one set to detection only logs them. A rule set with no mode of its own follows the saved WAF mode, which is detection when no mode had been saved.

### Per-Host Configuration

WAF can be enabled independently for each proxy host:

- Enable for public-facing applications
- Disable for internal services or APIs with custom security
- Mix modes across different hosts as needed

## Zero-Day Protection

The OWASP Core Rule Set is regularly updated to address:

- Newly discovered CVEs
- Emerging attack patterns
- Bypass techniques

Charon includes the latest CRS version and receives updates through container image releases.

## Limitations

The WAF protects **HTTP and HTTPS traffic only**:

| Traffic Type | Protected |
| -------------- | ----------- |
| HTTP/HTTPS Proxy Hosts | ✅ Yes |
| TCP/UDP Streams | ❌ No |
| Non-HTTP protocols | ❌ No |

For TCP/UDP protection, use [CrowdSec](./crowdsec.md) or network-level firewalls.

## Troubleshooting

| Issue | Solution |
| ------- | ---------- |
| Legitimate requests blocked | Switch the rule set to Detection mode and review logs |
| High latency | Check if complex rules are triggering; consider rule tuning |
| WAF shows as not applied | Check the WAF switch and rule set choice; see [Checking that a security module is applied](../troubleshooting/security-module-applied.md) |

## Related

- [CrowdSec Integration](./crowdsec.md) — Behavioral threat detection
- [Access Control](./access-control.md) — IP and geo-based restrictions
- Proxy Hosts — Configure WAF per host (guide not yet published)
- [Back to Features](../features.md)
