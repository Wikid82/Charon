---
title: HTTP Security Headers
description: Automatic security headers including CSP, HSTS, and more
category: security
---

# HTTP Security Headers

Modern browsers expect specific security headers to protect your users. Charon automatically adds industry-standard headers including Content-Security-Policy, Strict-Transport-Security, X-Frame-Options, and X-Content-Type-Options.

## Overview

HTTP security headers instruct browsers how to handle your content securely. Without them, your site remains vulnerable to clickjacking, XSS attacks, protocol downgrades, and MIME-type confusion. Charon provides a visual interface for configuring these headers without memorizing complex syntax.

### Supported Headers

| Header | Purpose |
|--------|---------|
| **HSTS** | Forces HTTPS connections, prevents downgrade attacks |
| **Content-Security-Policy** | Controls resource loading, mitigates XSS |
| **X-Frame-Options** | Prevents clickjacking via iframe embedding |
| **X-Content-Type-Options** | Stops MIME-type sniffing attacks |
| **Referrer-Policy** | Controls referrer information leakage |
| **Permissions-Policy** | Restricts browser feature access (camera, mic, geolocation) |
| **Cross-Origin-Opener-Policy** | Isolates browsing context |
| **Cross-Origin-Resource-Policy** | Controls cross-origin resource sharing |

## Why Use This

- **Browser Protection**: Modern browsers actively check for security headers
- **Compliance**: Many security audits and standards require specific headers
- **Defense in Depth**: Headers add protection even if application code has vulnerabilities
- **No Code Changes**: Protect legacy applications without modifying source code

## Security Presets

Charon offers three ready-to-use presets based on your security requirements:

### Basic (Production Safe)

Balanced security suitable for most production sites. Enables essential protections without breaking typical web functionality.

- HSTS enabled (1 year, includeSubdomains)
- X-Frame-Options: SAMEORIGIN
- X-Content-Type-Options: nosniff
- Referrer-Policy: strict-origin-when-cross-origin

### Strict (High Security)

Enhanced security for applications handling sensitive data. May require CSP tuning for inline scripts.

- All Basic headers plus:
- Content-Security-Policy with restrictive defaults
- Permissions-Policy denying sensitive features
- X-Frame-Options: DENY

### Paranoid (Maximum)

Maximum security for high-value targets. Expect to customize CSP directives for your specific application.

- All Strict headers plus:
- CSP with nonce-based script execution
- Cross-Origin policies fully restricted
- All permissions denied by default

## Configuration

### Using Presets

1. Navigate to **Hosts** → Select your host → **Security Headers**
2. Choose a preset from the dropdown
3. Review the applied headers in the preview
4. Click **Save** to apply

### Custom Header Profiles

Create reusable header configurations:

1. Go to **Settings** → **Security Profiles**
2. Click **Create Profile**
3. Name your profile (e.g., "API Servers", "Public Sites")
4. Configure individual headers
5. Save and apply to multiple hosts

### Interactive CSP Builder

The CSP Builder provides a visual interface for constructing Content-Security-Policy:

1. Select directive (script-src, style-src, img-src, etc.)
2. Add allowed sources (self, specific domains, unsafe-inline)
3. Preview the generated policy
4. Test against your site before applying

## Security Score Calculator

Each host displays a security score from 0-100 based on enabled headers:

| Score Range | Rating | Description |
|-------------|--------|-------------|
| 90-100 | Excellent | All recommended headers configured |
| 70-89 | Good | Core protections in place |
| 50-69 | Fair | Basic headers only |
| 0-49 | Poor | Missing critical headers |

## When to Use Each Preset

| Scenario | Recommended Preset |
|----------|-------------------|
| Marketing sites, blogs | Basic |
| E-commerce, user accounts | Strict |
| Banking, healthcare, government | Paranoid |
| Internal tools | Basic or Strict |
| APIs (no browser UI) | Minimal or disabled |

## Related

- [Proxy Headers](proxy-headers.md) - Backend communication headers
- [Access Lists](access-lists.md) - IP-based access control
- [Back to Features](../features.md)
