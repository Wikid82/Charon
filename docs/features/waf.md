---
title: Web Application Firewall (WAF)
description: Block attacks with the rule sets you add with Coraza WAF
---

# Web Application Firewall (WAF)

Stop common attacks like SQL injection, cross-site scripting (XSS), and path traversal before they reach your applications. Powered by Coraza, the WAF blocks these attacks using the rule sets you add.

## Overview

The Web Application Firewall inspects every HTTP/HTTPS request and blocks malicious payloads before they reach your backend services. Charon uses [Coraza](https://coraza.io/), a high-performance, open-source WAF engine compatible with the OWASP Core Rule Set (CRS).

Protected attack types include:

- **SQL Injection** — Blocks database manipulation attempts
- **Cross-Site Scripting (XSS)** — Prevents script injection attacks
- **Path Traversal** — Stops directory traversal exploits
- **Remote Code Execution** — Blocks command injection
- **New Attack Patterns** — Rule sets you add can be updated to cover newly discovered attacks

## Why Use This

- **Defense in Depth** — Add a security layer in front of your applications
- **Compatible With CRS** — Works with the OWASP Core Rule Set (CRS), an industry-standard ruleset. You paste its rules in yourself
- **Low Latency** — Coraza processes rules efficiently with minimal overhead
- **Flexible Modes** — Choose between monitoring and active blocking

## Configuration

### Turning On the WAF

1. Open the **Security** page
2. Turn on the **Web Application Firewall** switch
3. Open the **WAF** page and pick a rule set from the list, or choose automatic selection

The WAF page shows the rule sets you have and their modes. If the WAF is on but no rule set can be used yet, the Security page says so and tells you the reason.

### Rule Sets Need Rules

A rule set must contain at least one rule line. A rule line starts with `SecRule`, `SecAction` or `Include`. Charon refuses to save a rule set that is empty or that has only comments or settings in it, and tells you to add a rule.

If the rule set the WAF is using has no rules, the Security page shows a short note saying so. Open the WAF page and add rules to clear it.

Rule sets you created before this change are kept, but they are not used until they have at least one rule. You can edit them to add rules. Charon does not delete them for you.

### Operating Modes

Each rule set has its own mode:

| Mode | Behavior | Use Case |
| --- | --- | --- |
| **Detection** | Logs threats but allows traffic | Testing rules, reducing false positives |
| **Blocking** | Actively blocks malicious requests | Production protection |

**Recommendation**: Start with Detection to review detected threats, then switch the rule set to Blocking once you're confident in the rules.

### After Upgrading

After upgrading, check the WAF page. With the WAF switched on and a rule set selected, each rule set's own mode applies: a rule set set to blocking blocks matching requests, and one set to detection only logs them. A rule set with no mode of its own follows the saved WAF mode (detection when it is set to monitor, blocking otherwise).

The WAF page no longer offers a built-in "OWASP Core Rule Set" choice, because that choice did not contain any rules. To use OWASP rules, paste the rule text into a rule set yourself.

### Per-Host Configuration

WAF can be enabled independently for each proxy host:

- Enable for public-facing applications
- Disable for internal services or APIs with custom security

The Detection or Blocking mode belongs to each rule set, not to a host. Hosts that use the same rule set get the same mode.

## Zero-Day Protection

Charon does not include, download or update the OWASP Core Rule Set (CRS) or any other rules. Your rule sets are the text you paste in, and Charon keeps them as you saved them.

To get newer rules, copy the updated rule text from its source, open the rule set on the WAF page, and paste in the new text. Updating Charon keeps the rule sets you saved.

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
