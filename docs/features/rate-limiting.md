---
title: Rate Limiting
description: Prevent abuse by limiting requests per user or IP address
---

# Rate Limiting

Prevent abuse by limiting how many requests a user or IP address can make. Stop brute-force attacks, API abuse, and resource exhaustion with simple, configurable limits.

## Overview

Rate limiting controls how frequently clients can make requests to your proxied services. When a client exceeds the configured limit, additional requests receive a `429 Too Many Requests` response until the limit resets.

Key concepts:

- **Requests** — How many requests a visitor may send in one time window
- **Time Window** — The length of that window, in seconds
- **Burst** — A short spike allowance, used by Charon's management interface limit (see Parameters)
- **Per-Address Tracking** — Each visitor's network address has its own count

## Why Use This

- **Brute-Force Prevention** — Stop password guessing attacks
- **API Protection** — Prevent excessive API consumption
- **Resource Management** — Protect backend services from overload
- **Fair Usage** — Ensure equitable access across all users
- **Cost Control** — Limit expensive operations

## Configuration

### Turning On Rate Limiting

1. Open the **Security** page
2. Turn on the **Rate Limiting** switch
3. On the Rate Limiting card, check the numbers

When you turn it on, any number that is not set is filled with the standard values. Numbers you type in the same step are kept.

### Parameters

| Parameter | Description | Standard Value |
| ----------- | ------------- | --------- |
| **Requests** | How many requests a visitor may send in one window | `100` |
| **Time Window** | Length of the window in seconds | `60` (one minute) |
| **Burst** | Spike allowance for Charon's own management interface. Your sites use only Requests and Time Window | `20` |

### Numbers Not Set

If rate limiting was already on but its request count or time window is empty or zero, Charon does not guess. The Security page shows the rate limit as not applied, with the reason "thresholds unset", until you enter the numbers.

### Visitors Behind a Tunnel or Shared Network

Charon counts visitors by the network address that connects to it. If your site is reached through a tunnel, a proxy, or a shared network, many visitors can share one address. They then share one limit, and a busy page can reach it quickly. Raise the **Requests** number to give such visitors more room.

## Best Practices

- **Start Generous** — Begin with higher limits and tighten based on observed traffic
- **Monitor Logs** — Watch for legitimate users hitting limits
- **One Shared Limit** — The same numbers apply to every proxy host
- **Combine with WAF** — Rate limiting + WAF provides layered protection

## Related

- [Access Control](./access-control.md) — IP-based access restrictions
- [CrowdSec Integration](./crowdsec.md) — Automatic attacker blocking
- [Back to Features](../features.md)
