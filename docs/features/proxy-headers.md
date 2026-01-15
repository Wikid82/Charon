---
title: Smart Proxy Headers
description: Automatic X-Real-IP, X-Forwarded-For, and X-Forwarded-Proto headers
category: networking
---

# Smart Proxy Headers

Your backend applications need to know the real client IP address, not Charon's. Standard headers like X-Real-IP, X-Forwarded-For, and X-Forwarded-Proto are added automatically.

## Overview

When traffic passes through a reverse proxy, your backend loses visibility into the original client connection. Without proxy headers, every request appears to come from Charon's IP address, breaking logging, rate limiting, geolocation, and security features.

### Standard Proxy Headers

| Header | Purpose | Example Value |
|--------|---------|---------------|
| **X-Real-IP** | Original client IP address | `203.0.113.42` |
| **X-Forwarded-For** | Chain of proxy IPs | `203.0.113.42, 10.0.0.1` |
| **X-Forwarded-Proto** | Original protocol (HTTP/HTTPS) | `https` |
| **X-Forwarded-Host** | Original host header | `example.com` |
| **X-Forwarded-Port** | Original port number | `443` |

## Why These Headers Matter

### Client IP Detection

Without X-Real-IP, your application sees Charon's internal IP for every request:

- **Logging**: All logs show the same IP, making debugging impossible
- **Rate Limiting**: Cannot throttle abusive clients
- **Geolocation**: Location services return proxy location, not user location
- **Analytics**: Traffic analytics become meaningless

### HTTPS Enforcement

X-Forwarded-Proto tells your backend the original protocol:

- **Redirect Loops**: Backend sees HTTP, redirects to HTTPS, Charon proxies as HTTP, infinite loop
- **Secure Cookies**: Applications need to know when to set `Secure` flag
- **Mixed Content**: Helps applications generate correct absolute URLs

### Virtual Host Routing

X-Forwarded-Host preserves the original domain:

- **Multi-tenant Apps**: Route requests to correct tenant
- **URL Generation**: Generate correct links in emails, redirects

## Configuration

### Default Behavior

| Host Type | Proxy Headers |
|-----------|---------------|
| New hosts | **Enabled** by default |
| Existing hosts (pre-upgrade) | **Disabled** (preserves existing behavior) |

### Enabling/Disabling

1. Navigate to **Hosts** → Select your host
2. Go to **Advanced** tab
3. Toggle **Proxy Headers** on or off
4. Click **Save**

### Backend Configuration Requirements

Your backend must trust proxy headers from Charon. Common configurations:

**Node.js/Express:**
```javascript
app.set('trust proxy', true);
```

**Django:**
```python
SECURE_PROXY_SSL_HEADER = ('HTTP_X_FORWARDED_PROTO', 'https')
USE_X_FORWARDED_HOST = True
```

**Rails:**
```ruby
config.action_dispatch.trusted_proxies = [IPAddr.new('10.0.0.0/8')]
```

**PHP/Laravel:**
```php
// In TrustProxies middleware
protected $proxies = '*';
```

## When to Enable vs Disable

### Enable When

- Backend needs real client IP for logging or security
- Application generates absolute URLs
- Using secure cookies with HTTPS termination at proxy
- Rate limiting or geolocation features are needed

### Disable When

- Backend is an external service you don't control
- Proxying to another reverse proxy that handles headers
- Legacy application that misinterprets forwarded headers
- Security policy requires hiding internal topology

## Security Considerations

### Trusted Proxies

Only trust proxy headers from known sources. If your backend blindly trusts X-Forwarded-For, attackers can spoof their IP by injecting fake headers.

### Header Injection Prevention

Charon sanitizes incoming proxy headers before adding its own, preventing header injection attacks where malicious clients send fake forwarded headers.

### IP Chain Verification

When multiple proxies exist, verify the entire X-Forwarded-For chain rather than trusting only the first or last IP.

## Troubleshooting

| Issue | Likely Cause | Solution |
|-------|--------------|----------|
| Backend shows wrong IP | Headers not enabled | Enable proxy headers for host |
| Redirect loop | Backend doesn't trust X-Forwarded-Proto | Configure backend trust settings |
| Wrong URLs in emails | Missing X-Forwarded-Host trust | Enable host header forwarding |

## Related

- [Security Headers](security-headers.md) - Browser security headers
- [SSL Certificates](ssl-certificates.md) - HTTPS configuration
- [Back to Features](../features.md)
