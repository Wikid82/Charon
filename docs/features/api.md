---
title: REST API
description: Comprehensive REST API for automation and integrations
---

# REST API

Automate everything. Charon's comprehensive REST API lets you manage hosts, certificates, security rules, and settings programmatically. Perfect for CI/CD pipelines, Infrastructure as Code, or custom integrations.

## Overview

The REST API provides full control over Charon's functionality through HTTP endpoints. All responses are JSON-formatted, and the API follows RESTful conventions for resource management.

**Base URL**: `http://your-charon-instance:81/api`

### Authentication

All API requests require a Bearer token. Generate tokens in **Settings → API Tokens**.

```bash
# Include in all requests
Authorization: Bearer your-api-token-here
```

Tokens support granular permissions:
- **Read-only**: View configurations without modification
- **Full access**: Complete CRUD operations
- **Scoped**: Limit to specific resource types

## Why Use the API?

| Use Case | Benefit |
|----------|---------|
| **CI/CD Pipelines** | Automatically create proxy hosts for staging/preview deployments |
| **Infrastructure as Code** | Version control your Charon configuration |
| **Custom Dashboards** | Build monitoring integrations |
| **Bulk Operations** | Manage hundreds of hosts programmatically |
| **GitOps Workflows** | Sync configuration from Git repositories |

## Key Endpoints

### Proxy Hosts

```bash
# List all proxy hosts
curl -X GET "http://charon:81/api/nginx/proxy-hosts" \
  -H "Authorization: Bearer $TOKEN"

# Create a proxy host
curl -X POST "http://charon:81/api/nginx/proxy-hosts" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "domain_names": ["app.example.com"],
    "forward_host": "10.0.0.5",
    "forward_port": 3000,
    "ssl_forced": true,
    "certificate_id": 1
  }'

# Update a proxy host
curl -X PUT "http://charon:81/api/nginx/proxy-hosts/1" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"forward_port": 8080}'

# Delete a proxy host
curl -X DELETE "http://charon:81/api/nginx/proxy-hosts/1" \
  -H "Authorization: Bearer $TOKEN"
```

### SSL Certificates

```bash
# List certificates
curl -X GET "http://charon:81/api/nginx/certificates" \
  -H "Authorization: Bearer $TOKEN"

# Request new Let's Encrypt certificate
curl -X POST "http://charon:81/api/nginx/certificates" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "letsencrypt",
    "domain_names": ["secure.example.com"],
    "meta": {"dns_challenge": true, "dns_provider": "cloudflare"}
  }'
```

### DNS Providers

```bash
# List configured DNS providers
curl -X GET "http://charon:81/api/nginx/dns-providers" \
  -H "Authorization: Bearer $TOKEN"

# Add a DNS provider (for DNS-01 challenges)
curl -X POST "http://charon:81/api/nginx/dns-providers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Cloudflare Production",
    "acme_dns_provider": "cloudflare",
    "meta": {"CF_API_TOKEN": "your-cloudflare-token"}
  }'
```

### Security Settings

```bash
# Get WAF status
curl -X GET "http://charon:81/api/security/waf" \
  -H "Authorization: Bearer $TOKEN"

# Enable WAF for a host
curl -X PUT "http://charon:81/api/nginx/proxy-hosts/1" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"waf_enabled": true, "waf_mode": "block"}'

# List CrowdSec decisions
curl -X GET "http://charon:81/api/security/crowdsec/decisions" \
  -H "Authorization: Bearer $TOKEN"
```

## CI/CD Integration Example

### GitHub Actions

```yaml
- name: Create Preview Environment
  run: |
    curl -X POST "${{ secrets.CHARON_URL }}/api/nginx/proxy-hosts" \
      -H "Authorization: Bearer ${{ secrets.CHARON_TOKEN }}" \
      -H "Content-Type: application/json" \
      -d '{
        "domain_names": ["pr-${{ github.event.number }}.preview.example.com"],
        "forward_host": "${{ steps.deploy.outputs.ip }}",
        "forward_port": 3000
      }'
```

## Error Handling

The API returns standard HTTP status codes:

| Code | Meaning |
|------|---------|
| `200` | Success |
| `201` | Resource created |
| `400` | Invalid request body |
| `401` | Invalid or missing token |
| `403` | Insufficient permissions |
| `404` | Resource not found |
| `500` | Server error |

## Related

- [Backup & Restore](backup-restore.md) - API-managed backups
- [SSL Certificates](ssl-certificates.md) - Certificate automation
- [Back to Features](../features.md)
