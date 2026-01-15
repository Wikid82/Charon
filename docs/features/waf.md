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

### Enabling WAF

1. Navigate to **Proxy Hosts**
2. Edit or create a proxy host
3. In the **Security** tab, toggle **Web Application Firewall**
4. Select your preferred mode

### Operating Modes

| Mode | Behavior | Use Case |
|------|----------|----------|
| **Monitor** | Logs threats but allows traffic | Testing rules, reducing false positives |
| **Block** | Actively blocks malicious requests | Production protection |

**Recommendation**: Start in Monitor mode to review detected threats, then switch to Block mode once you're confident in the rules.

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
|--------------|-----------|
| HTTP/HTTPS Proxy Hosts | ✅ Yes |
| TCP/UDP Streams | ❌ No |
| Non-HTTP protocols | ❌ No |

For TCP/UDP protection, use [CrowdSec](./crowdsec.md) or network-level firewalls.

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Legitimate requests blocked | Switch to Monitor mode and review logs |
| High latency | Check if complex rules are triggering; consider rule tuning |
| WAF not activating | Verify the proxy host has WAF enabled in Security tab |

## Related

- [CrowdSec Integration](./crowdsec.md) — Behavioral threat detection
- [Access Control](./access-control.md) — IP and geo-based restrictions
- [Proxy Hosts](./proxy-hosts.md) — Configure WAF per host
- [Back to Features](../features.md)
