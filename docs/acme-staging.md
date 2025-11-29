# ACME Staging Environment

## Overview

Charon supports using Let's Encrypt's staging environment for development and testing. This prevents rate limiting issues when frequently rebuilding/testing SSL certificates.

## Configuration

Set the `CHARON_ACME_STAGING` environment variable to `true` to enable staging mode. `CPM_ACME_STAGING` is still supported as a legacy fallback:
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
      - CHARON_ENV=development
- Perfect for development, testing, and CI/CD

## Production Use

For production deployments:
- **Remove** or set `CPM_ACME_STAGING=false`
- Caddy will use the production Let's Encrypt server by default
- Certificates will be valid and trusted by browsers
      - CHARON_ENV=production

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
## Verifying Configuration
Check container logs to confirm staging is active:
```bash
docker logs charon 2>&1 | grep acme-staging
export CHARON_ACME_STAGING=true
Set the `CHARON_ACME_STAGING` environment variable to `true` to enable staging mode. `CHARON_` is preferred; `CPM_` variables are still supported as a legacy fallback.
Set the `CHARON_ACME_STAGING` environment variable to `true` to enable staging mode:
You should see:
```
export CHARON_ACME_STAGING=true
```

## Rate Limits Reference

  - CHARON_ACME_STAGING=true  # Use staging for dev (CHARON_ preferred; CPM_ still supported)
- 50 certificates per registered domain per week
- 5 duplicate certificates per week
- 300 new orders per account per 3 hours
- 10 accounts per IP address per 3 hours
      - CHARON_ENV=development
      - CHARON_ACME_STAGING=true  # Use staging for dev (CHARON_ preferred; CPM_ still supported)
- **No practical rate limits**
 - **Remove** or set `CHARON_ACME_STAGING=false` (CPM_ still supported)
- Perfect for development and testing
  - CHARON_ACME_STAGING=true  # Use staging for dev (CHARON_ preferred; CPM_ still supported)
### Staging (CHARON_ACME_STAGING=true)

1. Set `CHARON_ACME_STAGING=false` (or remove the variable)
### "Certificate not trusted" in browser
1. Set `CHARON_ACME_STAGING=false` (or remove the variable)
1. Set `CHARON_ACME_STAGING=true`
This is **expected** when using staging. Staging certificates are signed by a fake CA that browsers don't recognize.

1. Set `CHARON_ACME_STAGING=false` (or remove the variable)
1. Set `CHARON_ACME_STAGING=true`
### Switching from staging to production
1. Set `CPM_ACME_STAGING=false` (or remove the variable)
2. Restart the container
3. **Clean up staging certificates** (choose one method):

   **Option A - Via UI (Recommended):**
   - Go to **Certificates** page in the web interface
   - Delete any certificates with "acme-staging" in the issuer name

   **Option B - Via Terminal:**
   ```bash
  docker exec charon rm -rf /app/data/caddy/data/acme/acme-staging*
  docker exec charon rm -rf /data/acme/acme-staging*
   ```

4. Certificates will be automatically reissued from production on next request

### Switching from production to staging
1. Set `CPM_ACME_STAGING=true`
2. Restart the container
3. **Optional:** Delete production certificates to force immediate reissue
   ```bash
  docker exec charon rm -rf /app/data/caddy/data/acme/acme-v02.api.letsencrypt.org-directory
  docker exec charon rm -rf /data/acme/acme-v02.api.letsencrypt.org-directory
   ```

### Cleaning up old certificates
Caddy automatically manages certificate renewal and cleanup. However, if you need to manually clear certificates:

**Remove all ACME certificates (both staging and production):**
```bash
docker exec charon rm -rf /app/data/caddy/data/acme/*
docker exec charon rm -rf /data/acme/*
```

**Remove only staging certificates:**
```bash
docker exec charon rm -rf /app/data/caddy/data/acme/acme-staging*
 docker exec charon rm -rf /data/acme/acme-staging*
```

After deletion, restart your proxy hosts or container to trigger fresh certificate requests.

## Best Practices

1. **Always use staging for local development** to avoid hitting rate limits
2. Use production in CI/CD pipelines that test actual certificate validation
3. Document your environment variable settings in your deployment docs
4. Monitor Let's Encrypt rate limit emails in production
