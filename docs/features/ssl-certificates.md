---
title: Automatic HTTPS Certificates
description: Automatic SSL certificate provisioning and renewal via Let's Encrypt or ZeroSSL
---

# Automatic HTTPS Certificates

Charon automatically obtains free SSL certificates from Let's Encrypt or ZeroSSL, installs them, and renews them before they expire—all without you lifting a finger.

## Overview

When you create a proxy host with HTTPS enabled, Charon handles the entire certificate lifecycle:

1. **Automatic Provisioning** — Requests a certificate from your chosen provider
2. **Domain Validation** — Completes the ACME challenge automatically
3. **Installation** — Configures Caddy to use the new certificate
4. **Renewal** — Renews certificates before they expire (typically 30 days before)
5. **Smart Cleanup** — Removes certificates when you delete hosts

## Why Use This

- **Zero Configuration** — Works out of the box with sensible defaults
- **Free Certificates** — Both Let's Encrypt and ZeroSSL provide certificates at no cost
- **Always Valid** — Automatic renewal prevents certificate expiration
- **No Downtime** — Certificate updates happen seamlessly

## SSL Provider Selection

Navigate to **Settings → Default Settings** to choose your SSL provider:

| Provider | Best For | Rate Limits |
|----------|----------|-------------|
| **Auto** | Most users | Caddy selects automatically |
| **Let's Encrypt (Production)** | Production sites | 50 certs/domain/week |
| **Let's Encrypt (Staging)** | Testing & development | Unlimited (untrusted certs) |
| **ZeroSSL** | Alternative to LE, or if rate-limited | 3 certs/domain/90 days (free tier) |

### When to Use Each Provider

- **Auto**: Recommended for most users. Caddy intelligently selects the best provider.
- **Let's Encrypt Production**: When you need trusted certificates and are within rate limits.
- **Let's Encrypt Staging**: When testing your setup—certificates are not trusted by browsers but have no rate limits.
- **ZeroSSL**: When you've hit Let's Encrypt rate limits or prefer an alternative CA.

## Dashboard Certificate Status

The **Certificate Status Card** on your dashboard shows:

- Total certificates managed
- Certificates expiring soon (within 30 days)
- Any failed certificate requests

Click on any certificate to view details including expiration date, domains covered, and issuer information.

## Smart Certificate Cleanup

When you delete a proxy host, Charon automatically:

1. Removes the certificate from Caddy's configuration
2. Cleans up any associated ACME data
3. Frees up rate limit quota for new certificates

This prevents certificate accumulation and keeps your system tidy.

## Manual Certificate Deletion

Over time, expired or unused certificates can pile up in the Certificates list. You can remove them manually:

| Certificate Type | When You Can Delete It |
|------------------|----------------------|
| **Expired Let's Encrypt** | When it's not attached to any proxy host |
| **Custom (uploaded)** | When it's not attached to any proxy host |
| **Staging** | When it's not attached to any proxy host |
| **Valid Let's Encrypt** | Managed automatically — no delete button shown |

If a certificate is still attached to a proxy host, the delete button is disabled and a tooltip explains which host is using it. Remove the certificate from the proxy host first, then come back to delete it.

A confirmation dialog appears before anything is removed. Charon creates a backup before deleting, so you have a safety net.

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Certificate not issued | Ensure ports 80/443 are accessible from the internet |
| Rate limit exceeded | Switch to Let's Encrypt Staging or ZeroSSL temporarily |
| Domain validation failed | Verify DNS points to your Charon server |

## Related

- [Proxy Hosts](./proxy-hosts.md) — Configure HTTPS for your services
- [DNS Providers](./dns-providers.md) — Use DNS challenge for wildcard certificates
- [Back to Features](../features.md)
