---
title: CrowdSec Integration
description: Behavior-based threat detection powered by a global community
---

# CrowdSec Integration

Protect your applications using behavior-based threat detection powered by a global community of security data. Bad actors get blocked automatically before they can cause harm.

## Overview

CrowdSec analyzes your traffic patterns and blocks malicious behavior in real-time. Unlike traditional firewalls that rely on static rules, CrowdSec uses behavioral analysis and crowdsourced threat intelligence to identify and stop attacks.

Key capabilities:

- **Behavior Detection** — Identifies attack patterns like brute-force, scanning, and exploitation
- **Community Blocklists** — Benefit from threats detected by the global CrowdSec community
- **Real-time Blocking** — Malicious IPs are blocked immediately via Caddy integration
- **Automatic Updates** — Threat intelligence updates continuously

## Why Use This

- **Proactive Defense** — Block attackers before they succeed
- **Zero False Positives** — Behavioral analysis reduces incorrect blocks
- **Community Intelligence** — Leverage data from thousands of CrowdSec users
- **GUI-Controlled** — Enable/disable directly from the UI, no environment variables needed

## Configuration

### Enabling CrowdSec

1. Navigate to **Settings → Security**
2. Toggle **CrowdSec Protection** to enabled
3. CrowdSec starts automatically and persists across container restarts

No environment variables or manual configuration required.

### Hub Presets

Access pre-built security configurations from the CrowdSec Hub:

1. Go to **Settings → Security → Hub Presets**
2. Browse available collections (e.g., `crowdsecurity/nginx`, `crowdsecurity/http-cve`)
3. Search for specific parsers, scenarios, or collections
4. Click **Install** to add to your configuration

Popular presets include:

- **HTTP Probing** — Detect reconnaissance and scanning
- **Bad User-Agents** — Block known malicious bots
- **CVE Exploits** — Protection against known vulnerabilities

### Console Enrollment

Connect to the CrowdSec Console for centralized management:

1. Go to **Settings → Security → Console Enrollment**
2. Enter your enrollment key from [console.crowdsec.net](https://console.crowdsec.net)
3. Click **Enroll**

The Console provides:

- Multi-instance management
- Historical attack data
- Alert notifications
- Blocklist subscriptions

### Live Decisions

View active blocks in real-time:

1. Navigate to **Security → Live Decisions**
2. See all currently blocked IPs with:
   - IP address and origin country
   - Reason for block (scenario triggered)
   - Duration remaining
   - Option to manually unban

## Automatic Startup & Persistence

CrowdSec settings are stored in Charon's database and synchronized with the Security Config:

- **On Container Start** — CrowdSec launches automatically if previously enabled
- **Configuration Sync** — Changes in the UI immediately apply to CrowdSec
- **State Persistence** — Decisions and configurations survive restarts

## Related

- [Web Application Firewall](./waf.md) — Complement CrowdSec with WAF protection
- [Access Control](./access-control.md) — Manual IP blocking and geo-restrictions
- [Back to Features](../features.md)
