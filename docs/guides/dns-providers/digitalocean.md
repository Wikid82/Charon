# DigitalOcean DNS Provider Setup

## Overview

DigitalOcean provides DNS hosting for free with any DigitalOcean account. This guide covers setting up DigitalOcean DNS as a provider in Charon for wildcard certificate management.

## Prerequisites

- DigitalOcean account (free tier is sufficient)
- Domain added to DigitalOcean DNS
- Domain nameservers pointing to DigitalOcean:
  - `ns1.digitalocean.com`
  - `ns2.digitalocean.com`
  - `ns3.digitalocean.com`

## Step 1: Generate Personal Access Token

1. Log in to [DigitalOcean Control Panel](https://cloud.digitalocean.com/)
2. Click on **API** in the left sidebar (under Account)
3. Navigate to the **Tokens/Keys** tab
4. Click **Generate New Token** (in the Personal access tokens section)
5. Configure the token:
   - **Token Name:** `charon-dns-challenge` (or any descriptive name)
   - **Expiration:** Choose expiration period (90 days, 1 year, or no expiry)
   - **Scopes:** Select **Write** (this includes Read access)
6. Click **Generate Token**
7. **Copy the token immediately** (shown only once)

> **Warning:** DigitalOcean shows the token only once. Store it securely in a password manager.

## Step 2: Verify DNS Configuration

Ensure your domain is properly configured in DigitalOcean DNS:

1. Navigate to **Networking** → **Domains** in the DigitalOcean control panel
2. Verify your domain is listed
3. Click on the domain to view DNS records
4. Ensure at least one A or CNAME record exists (for the domain itself)

> **Note:** Charon will create and remove TXT records automatically; no manual DNS configuration is needed.

## Step 3: Configure in Charon

1. Navigate to **DNS Providers** in Charon
2. Click **Add Provider**
3. Fill in the form:
   - **Provider Type:** Select `DigitalOcean`
   - **Name:** Enter a descriptive name (e.g., "DigitalOcean DNS")
   - **API Token:** Paste the Personal Access Token from Step 1

### Advanced Settings (Optional)

Expand **Advanced Settings** to customize:

- **Propagation Timeout:** `90` seconds (DigitalOcean propagates quickly)
- **Polling Interval:** `10` seconds (default)
- **Set as Default:** Enable if this is your primary DNS provider

## Step 4: Test Connection

1. Click **Test Connection** button
2. Wait for validation (usually 3-5 seconds)
3. Verify you see: ✅ **Connection successful**

The test verifies:

- Token is valid and active
- Account has DNS write permissions
- DigitalOcean API is accessible

If the test fails, see [Troubleshooting](#troubleshooting) below.

## Step 5: Save Configuration

Click **Save** to store the DNS provider configuration. The token is encrypted at rest using AES-256-GCM.

## Step 6: Use with Wildcard Certificates

When creating a proxy host with a wildcard domain:

1. Navigate to **Proxy Hosts** → **Add Proxy Host**
2. Enter a wildcard domain: `*.example.com`
3. Select **DigitalOcean** from the DNS Provider dropdown
4. Configure remaining settings
5. Save

Charon will automatically obtain a wildcard certificate using DNS-01 challenge.

## Example Configuration

```yaml
Provider Type: digitalocean
Name: DigitalOcean - example.com
API Token: dop_v1_********************************
Propagation Timeout: 90 seconds
Polling Interval: 10 seconds
Default: Yes
```

## Required Permissions

The Personal Access Token needs **Write** scope, which includes:

- Read access to domains and DNS records
- Write access to create/update/delete DNS records

> **Note:** Token scope is account-wide. You cannot restrict to specific domains in DigitalOcean.

## Troubleshooting

### Connection Test Fails

**Error:** `Invalid token` or `Unauthorized`

- Verify the token was copied correctly (should start with `dop_v1_`)
- Ensure token has **Write** scope (not just Read)
- Check token hasn't expired (if expiration was set)
- Regenerate the token if necessary

**Error:** `Domain not found`

- Verify the domain is added to DigitalOcean DNS
- Ensure domain nameservers point to DigitalOcean
- Check domain status in the Networking section
- Wait 24-48 hours if nameservers were recently changed

### Certificate Issuance Fails

**Error:** `DNS propagation timeout`

- DigitalOcean DNS typically propagates in <60 seconds
- Verify nameservers are correctly configured:

  ```bash
  dig NS example.com +short
  ```

- Check DigitalOcean Status page for service issues
- Increase Propagation Timeout to 120 seconds as a workaround

**Error:** `Record creation failed`

- Check token permissions (must be Write scope)
- Verify domain exists in DigitalOcean DNS
- Review Charon logs for detailed API errors
- Ensure no conflicting TXT records exist with name `_acme-challenge`

### Nameserver Propagation

**Issue:** DNS changes not visible globally

- Nameserver changes can take 24-48 hours to propagate
- Use [DNS Checker](https://dnschecker.org/) to verify global propagation
- Ensure your domain registrar shows DigitalOcean nameservers
- Wait for full propagation before attempting certificate issuance

### Rate Limiting

DigitalOcean API rate limits:

- 5,000 requests per hour (per account)
- Certificate challenges typically use <20 requests

If you hit limits:

- Reduce frequency of certificate renewals
- Avoid unnecessary test connection attempts
- Contact DigitalOcean support if consistently hitting limits

## Security Recommendations

1. **Token Expiration:** Set 90-day expiration and rotate regularly
2. **Dedicated Token:** Create a separate token for Charon (easier to revoke)
3. **Monitor Usage:** Review API logs in DigitalOcean control panel
4. **Least Privilege:** Use Write scope (don't grant Full Access)
5. **Backup Access:** Keep a backup token in secure storage (offline)
6. **Revoke Unused:** Delete tokens that are no longer needed

## DigitalOcean DNS Limitations

- **No per-domain token scoping:** Tokens grant access to all domains in the account
- **No rate limit customization:** Fixed at 5,000 requests/hour
- **Public zones only:** Private DNS not supported
- **No DNSSEC:** DigitalOcean does not support DNSSEC at this time

## Additional Resources

- [DigitalOcean DNS Documentation](https://docs.digitalocean.com/products/networking/dns/)
- [DigitalOcean API Documentation](https://docs.digitalocean.com/reference/api/)
- [Personal Access Tokens Guide](https://docs.digitalocean.com/reference/api/create-personal-access-token/)
- [Caddy DigitalOcean Module](https://caddyserver.com/docs/modules/dns.providers.digitalocean)
- [DigitalOcean Status Page](https://status.digitalocean.com/)

## Related Documentation

- [DNS Providers Overview](../dns-providers.md)
- [Wildcard Certificates Guide](../certificates.md#wildcard-certificates)
- [DNS Challenges Troubleshooting](../../troubleshooting/dns-challenges.md)
