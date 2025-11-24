# ACME Staging Environment

## Overview

CaddyProxyManager+ supports using Let's Encrypt's staging environment for development and testing. This prevents rate limiting issues when frequently rebuilding/testing SSL certificates.

## Configuration

Set the `CPM_ACME_STAGING` environment variable to `true` to enable staging mode:

```bash
export CPM_ACME_STAGING=true
```

Or in Docker Compose:

```yaml
environment:
  - CPM_ACME_STAGING=true
```

## What It Does

When enabled:
- Caddy will use `https://acme-staging-v02.api.letsencrypt.org/directory` instead of production
- Certificates issued will be **fake/invalid** for browsers (untrusted)
- **No rate limits** apply to staging certificates
- Perfect for development, testing, and CI/CD

## Production Use

For production deployments:
- **Remove** or set `CPM_ACME_STAGING=false`
- Caddy will use the production Let's Encrypt server by default
- Certificates will be valid and trusted by browsers
- Subject to [Let's Encrypt rate limits](https://letsencrypt.org/docs/rate-limits/)

## Docker Compose Examples

### Development (docker-compose.local.yml)
```yaml
services:
  app:
    environment:
      - CPM_ENV=development
      - CPM_ACME_STAGING=true  # Use staging for dev
```

### Production (docker-compose.yml)
```yaml
services:
  app:
    environment:
      - CPM_ENV=production
      # CPM_ACME_STAGING not set (defaults to false)
```

## Verifying Configuration

Check container logs to confirm staging is active:

```bash
docker logs cpmp 2>&1 | grep acme-staging
```

You should see:
```
"ca":"https://acme-staging-v02.api.letsencrypt.org/directory"
```

## Rate Limits Reference

### Production (CPM_ACME_STAGING=false or unset)
- 50 certificates per registered domain per week
- 5 duplicate certificates per week
- 300 new orders per account per 3 hours
- 10 accounts per IP address per 3 hours

### Staging (CPM_ACME_STAGING=true)
- **No practical rate limits**
- Certificates are not trusted by browsers
- Perfect for development and testing

## Troubleshooting

### "Certificate not trusted" in browser
This is **expected** when using staging. Staging certificates are signed by a fake CA that browsers don't recognize.

### Switching from staging to production
1. Set `CPM_ACME_STAGING=false` (or remove the variable)
2. Restart the container
3. Delete the old staging certificates: `docker exec cpmp rm -rf /app/data/caddy/data/acme/acme-staging*`
4. Certificates will be automatically reissued from production

### Switching from production to staging
1. Set `CPM_ACME_STAGING=true`
2. Restart the container
3. Optionally delete old production certificates to force immediate reissue

## Best Practices

1. **Always use staging for local development** to avoid hitting rate limits
2. Use production in CI/CD pipelines that test actual certificate validation
3. Document your environment variable settings in your deployment docs
4. Monitor Let's Encrypt rate limit emails in production
