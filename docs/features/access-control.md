---
title: Access Control Lists (ACLs)
description: Define exactly who can access what with fine-grained rules
---

# Access Control Lists (ACLs)

Define exactly who can access what. Block specific countries, allow only certain IP ranges, or require authentication for sensitive applications. Fine-grained rules give you complete control.

## Overview

Access Control Lists let you create granular rules that determine who can reach your proxied services. Rules are evaluated in order, and the first matching rule determines whether access is allowed or denied.

ACL capabilities:

- **IP Allowlists** — Only permit specific IPs or ranges
- **IP Blocklists** — Deny access from known bad actors
- **Country/Geo Blocking** — Restrict access by geographic location
- **CIDR Support** — Define rules using network ranges (e.g., `192.168.1.0/24`)

## Why Use This

- **Compliance** — Restrict access to specific regions for data sovereignty
- **Security** — Block high-risk countries or known malicious networks
- **Internal Services** — Limit access to corporate IP ranges
- **Layered Defense** — Combine with WAF and CrowdSec for comprehensive protection

## Configuration

### Creating an Access List

1. Navigate to **Access Lists** in the sidebar
2. Click **Add Access List**
3. Provide a descriptive name (e.g., "Office IPs Only")
4. Configure your rules

### Rule Types

#### IP Range Filtering

Add specific IPs or CIDR ranges:

```text
Allow: 192.168.1.0/24      # Allow entire subnet
Allow: 10.0.0.5            # Allow single IP
Deny:  0.0.0.0/0           # Deny everything else
```

Rules are processed top-to-bottom. Place more specific rules before broader ones.

#### Country/Geo Blocking

Block or allow traffic by country:

1. In the Access List editor, go to **Country Rules**
2. Select countries to **Allow** or **Deny**
3. Choose default action for unlisted countries

Common configurations:

- **Allow only your country** — Whitelist your country, deny all others
- **Block high-risk regions** — Deny specific countries, allow rest
- **Compliance zones** — Allow only EU countries for GDPR compliance

### Applying to Proxy Hosts

1. Edit your proxy host
2. Go to the **Access** tab
3. Select your Access List from the dropdown
4. Save changes

Each proxy host can have one Access List assigned. Create multiple lists for different access patterns.

## Rule Evaluation Order

```text
1. Check IP allowlist → Allow if matched
2. Check IP blocklist → Deny if matched
3. Check country rules → Allow/Deny based on geo
4. Apply default action
```

## Best Practices

| Scenario | Recommendation |
|----------|----------------|
| Internal admin panels | Allowlist office/VPN IPs only |
| Public websites | Use geo-blocking for high-risk regions |
| API endpoints | Combine IP rules with rate limiting |
| Development servers | Restrict to developer IPs |

## Related

- [Proxy Hosts](./proxy-hosts.md) — Apply access lists to services
- [CrowdSec Integration](./crowdsec.md) — Automatic threat-based blocking
- [Rate Limiting](./rate-limiting.md) — Limit request frequency
- [Back to Features](../features.md)
