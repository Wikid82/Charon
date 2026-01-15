---
title: Rate Limiting
description: Prevent abuse by limiting requests per user or IP address
---

# Rate Limiting

Prevent abuse by limiting how many requests a user or IP address can make. Stop brute-force attacks, API abuse, and resource exhaustion with simple, configurable limits.

## Overview

Rate limiting controls how frequently clients can make requests to your proxied services. When a client exceeds the configured limit, additional requests receive a `429 Too Many Requests` response until the limit resets.

Key concepts:

- **Requests per Second (RPS)** — Sustained request rate allowed
- **Burst Limit** — Short-term spike allowance above RPS
- **Time Window** — Period over which limits are calculated
- **Per-IP Tracking** — Each client IP has independent limits

## Why Use This

- **Brute-Force Prevention** — Stop password guessing attacks
- **API Protection** — Prevent excessive API consumption
- **Resource Management** — Protect backend services from overload
- **Fair Usage** — Ensure equitable access across all users
- **Cost Control** — Limit expensive operations

## Configuration

### Enabling Rate Limiting

1. Navigate to **Proxy Hosts**
2. Edit or create a proxy host
3. Go to the **Advanced** tab
4. Toggle **Rate Limiting** to enabled
5. Configure your limits

### Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| **Requests/Second** | Sustained rate limit | `10` = 10 requests per second |
| **Burst Limit** | Temporary spike allowance | `50` = allow 50 rapid requests |
| **Time Window** | Reset period in seconds | `60` = limits reset every minute |

### Understanding Burst vs Sustained Rate

```text
Sustained Rate: 10 req/sec
Burst Limit: 50

Behavior:
- Client can send 50 requests instantly (burst)
- Then limited to 10 req/sec until burst refills
- Burst tokens refill at the sustained rate
```

This allows legitimate traffic spikes (page loads with many assets) while preventing sustained abuse.

### Recommended Configurations

| Use Case | RPS | Burst | Window |
|----------|-----|-------|--------|
| Public website | 20 | 100 | 60s |
| Login endpoint | 5 | 10 | 60s |
| API endpoint | 30 | 60 | 60s |
| Static assets | 100 | 500 | 60s |

## Dashboard Integration

### Status Badge

When rate limiting is enabled, the proxy host displays a **Rate Limited** badge on:

- Proxy host list view
- Host detail page

### Active Summary Card

The dashboard shows an **Active Rate Limiting** summary card displaying:

- Number of hosts with rate limiting enabled
- Current configuration summary
- Link to manage settings

## Response Headers

Rate-limited responses include helpful headers:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 5
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1642000000
```

Clients can use these headers to implement backoff strategies.

## Best Practices

- **Start Generous** — Begin with higher limits and tighten based on observed traffic
- **Monitor Logs** — Watch for legitimate users hitting limits
- **Separate Endpoints** — Use different limits for different proxy hosts
- **Combine with WAF** — Rate limiting + WAF provides layered protection

## Related

- [Access Control](./access-control.md) — IP-based access restrictions
- [CrowdSec Integration](./crowdsec.md) — Automatic attacker blocking
- [Proxy Hosts](./proxy-hosts.md) — Configure rate limits per host
- [Back to Features](../features.md)
