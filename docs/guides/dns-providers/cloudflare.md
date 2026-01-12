# Cloudflare DNS Provider Setup

## Overview

Cloudflare is one of the most popular DNS providers and offers a free tier with API access. This guide walks you through setting up Cloudflare as a DNS provider in Charon for wildcard certificate support.

## Prerequisites

- Active Cloudflare account (free tier is sufficient)
- Domain added to Cloudflare with nameservers configured
- Domain status: **Active** (not pending nameserver update)

## Step 1: Generate API Token

Cloudflare API Tokens provide scoped access and are more secure than Global API Keys.

1. Log in to [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. Click on your profile icon (top right) → **My Profile**
3. Select **API Tokens** from the left sidebar
4. Click **Create Token**
5. Use the **Edit zone DNS** template or create a custom token
6. Configure token permissions:
   - **Permissions:**
     - Zone → DNS → Edit
   - **Zone Resources:**
     - Include → Specific zone → Select your domain
     - OR Include → All zones (if managing multiple domains)
7. (Optional) Set **Client IP Address Filtering** for additional security
8. (Optional) Set **TTL** for token expiration
9. Click **Continue to summary**
10. Review permissions and click **Create Token**
11. **Copy the token immediately** (shown only once)

> **Tip:** Store the API token in a password manager. Cloudflare won't display it again.

## Step 2: Configure in Charon

1. Navigate to **DNS Providers** in Charon
2. Click **Add Provider**
3. Fill in the form:
   - **Provider Type:** Select `Cloudflare`
   - **Name:** Enter a descriptive name (e.g., "Cloudflare Production")
   - **API Token:** Paste the token from Step 1

### Advanced Settings (Optional)

Expand **Advanced Settings** to customize:

- **Propagation Timeout:** `60` seconds (Cloudflare has fast global propagation)
- **Polling Interval:** `10` seconds (default)
- **Set as Default:** Enable if this is your primary DNS provider

## Step 3: Test Connection

1. Click **Test Connection** button
2. Wait for validation (usually 2-5 seconds)
3. Verify you see: ✅ **Connection successful**

If the test fails, see [Troubleshooting](#troubleshooting) below.

## Step 4: Save Configuration

Click **Save** to store the DNS provider configuration. Credentials are encrypted at rest using AES-256-GCM.

## Step 5: Use with Wildcard Certificates

When creating a proxy host with a wildcard domain:

1. Navigate to **Proxy Hosts** → **Add Proxy Host**
2. Enter a wildcard domain: `*.example.com`
3. Select **Cloudflare** from the DNS Provider dropdown
4. Configure remaining settings
5. Save

Charon will automatically obtain a wildcard certificate using DNS-01 challenge.

## Example Configuration

```yaml
Provider Type: cloudflare
Name: Cloudflare - example.com
API Token: ********************************
Propagation Timeout: 60 seconds
Polling Interval: 10 seconds
Default: Yes
```

## Required Permissions

The API token needs the following Cloudflare permissions:

- **Zone → DNS → Edit:** Create and delete TXT records for ACME challenges

> **Note:** The token does NOT need Zone → Edit or Account-level permissions.

## Troubleshooting

### Connection Test Fails

**Error:** `Invalid API token`

- Verify the token was copied correctly (no extra spaces)
- Ensure the token has Zone → DNS → Edit permission
- Check token hasn't expired (if TTL was set)
- Regenerate the token if necessary

**Error:** `Zone not found`

- Verify the domain is added to your Cloudflare account
- Ensure domain status is **Active** (nameservers updated)
- Check API token includes the correct zone in Zone Resources

### Certificate Issuance Fails

**Error:** `DNS propagation timeout`

- Cloudflare typically propagates in <30 seconds
- Check Cloudflare Status page for service issues
- Verify DNSSEC is configured correctly (if enabled)
- Try increasing Propagation Timeout to 120 seconds

**Error:** `Unauthorized to edit DNS`

- API token may have been revoked
- Regenerate a new token with correct permissions
- Update configuration in Charon

### Rate Limiting

Cloudflare has generous API rate limits:

- Free plan: 1,200 requests per 5 minutes
- Certificate challenges typically use <10 requests

If you hit limits:

- Reduce polling frequency
- Avoid unnecessary test connection attempts
- Consider upgrading Cloudflare plan

## Security Recommendations

1. **Scope Tokens:** Limit to specific zones rather than "All zones"
2. **IP Filtering:** Add your server's IP to Client IP Address Filtering
3. **Set Expiration:** Use token TTL for automatic expiration (renew before expiry)
4. **Rotate Regularly:** Generate new tokens every 90-180 days
5. **Monitor Usage:** Review API token activity in Cloudflare dashboard

## Additional Resources

- [Cloudflare API Documentation](https://developers.cloudflare.com/api/)
- [API Token Permissions](https://developers.cloudflare.com/api/tokens/create/)
- [Caddy Cloudflare Module](https://caddyserver.com/docs/modules/dns.providers.cloudflare)
- [Cloudflare Status Page](https://www.cloudflarestatus.com/)

## Related Documentation

- [DNS Providers Overview](../dns-providers.md)
- [Wildcard Certificates Guide](../certificates.md#wildcard-certificates)
- [DNS Challenges Troubleshooting](../../troubleshooting/dns-challenges.md)
