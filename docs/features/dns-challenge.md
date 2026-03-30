# DNS Challenge (DNS-01) for SSL Certificates

Charon supports **DNS-01 challenge validation** for issuing SSL/TLS certificates, enabling wildcard certificates and secure automation through 15+ integrated DNS providers.

## Table of Contents

- [Overview](#overview)
- [Why Use DNS Challenge?](#why-use-dns-challenge)
- [Supported DNS Providers](#supported-dns-providers)
- [Getting Started](#getting-started)
- [Manual DNS Challenge](#manual-dns-challenge)
- [Troubleshooting](#troubleshooting)
- [Related Documentation](#related-documentation)

---

## Overview

### What is DNS-01 Challenge?

The DNS-01 challenge is an ACME (Automatic Certificate Management Environment) validation method where you prove domain ownership by creating a specific DNS TXT record. When you request a certificate, the Certificate Authority (CA) provides a challenge token that must be published as a DNS record at `_acme-challenge.yourdomain.com`.

### How It Works

```
┌─────────────┐       1. Request Certificate       ┌──────────────┐
│   Charon    │ ─────────────────────────────────▶ │   Let's      │
│             │                                    │   Encrypt    │
│             │ ◀───────────────────────────────── │   (CA)       │
└─────────────┘       2. Receive Challenge Token   └──────────────┘
      │
      │ 3. Create TXT Record via DNS Provider API
      ▼
┌─────────────┐
│    DNS      │  _acme-challenge.example.com TXT "token123..."
│   Provider  │
└─────────────┘
      │
      │ 4. CA Verifies DNS Record
      ▼
┌──────────────┐       5. Certificate Issued       ┌─────────────┐
│   Let's      │ ─────────────────────────────────▶│   Charon    │
│   Encrypt    │                                   │             │
└──────────────┘                                   └─────────────┘
```

### Key Features

| Feature | Description |
|---------|-------------|
| **Wildcard Certificates** | Issue certificates for `*.example.com` |
| **15+ DNS Providers** | Native integration with major DNS services |
| **Secure Credentials** | AES-256-GCM encryption with automatic key rotation |
| **Plugin Architecture** | Extend with custom providers via webhooks or scripts |
| **Manual Option** | Support for any DNS provider via manual record creation |
| **Auto-Renewal** | Certificates renew automatically before expiration |

---

## Why Use DNS Challenge?

### DNS-01 vs HTTP-01 Comparison

| Feature | DNS-01 Challenge | HTTP-01 Challenge |
|---------|-----------------|-------------------|
| **Wildcard Certificates** | ✅ Yes | ❌ No |
| **Requires Port 80** | ❌ No | ✅ Yes |
| **Works Behind Firewall** | ✅ Yes | ⚠️ Requires port forwarding |
| **Internal Networks** | ✅ Yes | ❌ No |
| **Multiple Servers** | ✅ One validation, many servers | ❌ Each server validates |
| **Setup Complexity** | Medium (API credentials) | Low (no credentials) |

### When to Use DNS-01

Choose DNS-01 challenge when you need:

- ✅ **Wildcard certificates** (`*.example.com`) — DNS-01 is the **only** method that supports wildcards
- ✅ **Servers without public port 80** — Firewalls, NAT, or security policies blocking HTTP
- ✅ **Internal/private networks** — Servers not accessible from the internet
- ✅ **Multi-server deployments** — One certificate for load-balanced or clustered services
- ✅ **CI/CD automation** — Fully automated certificate issuance without HTTP exposure

### When HTTP-01 May Be Better

Consider HTTP-01 challenge when:

- You don't need wildcard certificates
- Port 80 is available and publicly accessible
- You want simpler setup without managing DNS credentials
- Your DNS provider isn't supported by Charon

---

## Supported DNS Providers

Charon integrates with 15+ DNS providers for automatic DNS record management.

### Tier 1: Full API Support

These providers have complete, tested integration with automatic record creation and cleanup:

| Provider | API Type | Documentation |
|----------|----------|---------------|
| **Cloudflare** | REST API | [Cloudflare Setup Guide](#cloudflare-setup) |
| **AWS Route53** | AWS SDK | [Route53 Setup Guide](#route53-setup) |
| **DigitalOcean** | REST API | [DigitalOcean Setup Guide](#digitalocean-setup) |
| **Google Cloud DNS** | GCP SDK | [Google Cloud Setup Guide](#google-cloud-setup) |
| **Azure DNS** | Azure SDK | [Azure Setup Guide](#azure-setup) |

### Tier 2: Standard API Support

Fully functional providers with standard API integration:

| Provider | API Type | Notes |
|----------|----------|-------|
| **Hetzner** | REST API | Hetzner Cloud DNS |
| **Linode** | REST API | Linode DNS Manager |
| **Vultr** | REST API | Vultr DNS |
| **OVH** | REST API | OVH API credentials required |
| **Namecheap** | XML API | API access must be enabled in account |
| **GoDaddy** | REST API | Production API key required |
| **DNSimple** | REST API | v2 API |
| **NS1** | REST API | NS1 Managed DNS |

### Tier 3: Alternative Methods

For providers without direct API support or custom DNS infrastructure:

| Method | Use Case | Documentation |
|--------|----------|---------------|
| **RFC 2136** | Self-hosted BIND9, PowerDNS, Knot DNS | [RFC 2136 Setup](./dns-providers.md#rfc-2136-dynamic-dns) |
| **Webhook** | Custom DNS APIs, automation platforms | [Webhook Provider](./dns-providers.md#webhook-provider) |
| **Script** | Legacy tools, custom integrations | [Script Provider](./dns-providers.md#script-provider) |
| **Manual** | Any DNS provider (user creates records) | [Manual DNS Challenge](#manual-dns-challenge) |

---

## Getting Started

### Prerequisites

Before configuring DNS challenge:

1. ✅ A domain name you control
2. ✅ Access to your DNS provider's control panel
3. ✅ API credentials from your DNS provider (see provider-specific guides below)
4. ✅ Charon installed and running

### Step 1: Add a DNS Provider

1. Navigate to **Settings** → **DNS Providers** in Charon
2. Click **"Add DNS Provider"**
3. Select your DNS provider from the dropdown
4. Enter a descriptive name (e.g., "Cloudflare - Production")

### Step 2: Configure API Credentials

Each provider requires specific credentials. See provider-specific sections below.

> **Security Note**: All credentials are encrypted with AES-256-GCM before storage. See [Key Rotation](./key-rotation.md) for credential security best practices.

### Step 3: Test the Connection

1. After saving, click **"Test Connection"** on the provider card
2. Charon will verify API access by attempting to list DNS zones
3. A green checkmark indicates successful authentication

### Step 4: Request a Certificate

1. Navigate to **Certificates** → **Request Certificate**
2. Enter your domain name:
   - For standard certificate: `example.com`
   - For wildcard certificate: `*.example.com`
3. Select **"DNS-01"** as the challenge type
4. Choose your configured DNS provider
5. Click **"Request Certificate"**

### Step 5: Monitor Progress

The certificate request progresses through these stages:

```
Pending → Creating DNS Record → Waiting for Propagation → Validating → Issued
```

- **Creating DNS Record**: Charon creates the `_acme-challenge` TXT record
- **Waiting for Propagation**: DNS changes propagate globally (typically 30-120 seconds)
- **Validating**: CA verifies the DNS record
- **Issued**: Certificate is ready for use

---

## Provider-Specific Setup

### Cloudflare Setup

Cloudflare is the recommended DNS provider due to fast propagation and excellent API support.

#### Creating API Credentials

**Option A: API Token (Recommended)**

1. Log in to [Cloudflare Dashboard](https://dash.cloudflare.com)
2. Go to **My Profile** → **API Tokens**
3. Click **"Create Token"**
4. Select **"Edit zone DNS"** template
5. Configure permissions:
   - **Zone**: DNS → Edit
   - **Zone Resources**: Include → Specific zone → Your domain
6. Click **"Continue to summary"** → **"Create Token"**
7. Copy the token (shown only once)

**Option B: Global API Key (Not Recommended)**

1. Go to **My Profile** → **API Tokens**
2. Scroll to **"API Keys"** section
3. Click **"View"** next to **"Global API Key"**
4. Copy the key

#### Charon Configuration

| Field | Value |
|-------|-------|
| **Provider Type** | Cloudflare |
| **API Token** | Your API token (from Option A) |
| **Email** | (Required only for Global API Key) |

### Route53 Setup

AWS Route53 requires IAM credentials with specific DNS permissions.

#### Creating IAM Policy

Create a custom IAM policy with these permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "route53:GetHostedZone",
        "route53:ListHostedZones",
        "route53:ListHostedZonesByName",
        "route53:ChangeResourceRecordSets",
        "route53:GetChange"
      ],
      "Resource": "*"
    }
  ]
}
```

> **Security Tip**: For production, restrict `Resource` to specific hosted zone ARNs.

#### Creating IAM User

1. Go to **IAM** → **Users** → **Add Users**
2. Enter username (e.g., `charon-dns`)
3. Select **"Access key - Programmatic access"**
4. Attach the custom policy created above
5. Complete user creation and save the **Access Key ID** and **Secret Access Key**

#### Charon Configuration

| Field | Value |
|-------|-------|
| **Provider Type** | Route53 |
| **Access Key ID** | Your IAM access key |
| **Secret Access Key** | Your IAM secret key |
| **Region** | (Optional) AWS region, e.g., `us-east-1` |

### DigitalOcean Setup

#### Creating API Token

1. Log in to [DigitalOcean Control Panel](https://cloud.digitalocean.com)
2. Go to **API** → **Tokens/Keys**
3. Click **"Generate New Token"**
4. Enter a name (e.g., "Charon DNS")
5. Select **"Write"** scope
6. Click **"Generate Token"**
7. Copy the token (shown only once)

#### Charon Configuration

| Field | Value |
|-------|-------|
| **Provider Type** | DigitalOcean |
| **API Token** | Your personal access token |

### Google Cloud Setup

#### Creating Service Account

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Select your project
3. Navigate to **IAM & Admin** → **Service Accounts**
4. Click **"Create Service Account"**
5. Enter name (e.g., `charon-dns`)
6. Grant role: **DNS Administrator** (`roles/dns.admin`)
7. Click **"Create Key"** → **JSON**
8. Download and secure the JSON key file

#### Charon Configuration

| Field | Value |
|-------|-------|
| **Provider Type** | Google Cloud DNS |
| **Project ID** | Your GCP project ID |
| **Service Account JSON** | Contents of the JSON key file |

### Azure Setup

#### Creating Service Principal

```bash
# Create service principal with DNS Zone Contributor role
az ad sp create-for-rbac \
  --name "charon-dns" \
  --role "DNS Zone Contributor" \
  --scopes "/subscriptions/<subscription-id>/resourceGroups/<resource-group>/providers/Microsoft.Network/dnszones/<zone-name>"
```

Save the output containing `appId`, `password`, and `tenant`.

#### Charon Configuration

| Field | Value |
|-------|-------|
| **Provider Type** | Azure DNS |
| **Subscription ID** | Your Azure subscription ID |
| **Resource Group** | Resource group containing DNS zone |
| **Tenant ID** | Azure AD tenant ID |
| **Client ID** | Service principal appId |
| **Client Secret** | Service principal password |

---

## Manual DNS Challenge

For DNS providers not directly supported by Charon, you can use the **Manual DNS Challenge** workflow.

### When to Use Manual Challenge

- Your DNS provider lacks API support
- Company policies restrict API credential storage
- You prefer manual control over DNS records
- Testing or one-time certificate requests

### Manual Challenge Workflow

#### Step 1: Initiate the Challenge

1. Navigate to **Certificates** → **Request Certificate**
2. Enter your domain name
3. Select **"DNS-01 (Manual)"** as the challenge type
4. Click **"Request Certificate"**

#### Step 2: Create DNS Record

Charon displays the required DNS record:

```
┌──────────────────────────────────────────────────────────────────────┐
│  Manual DNS Challenge Instructions                                   │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Create the following DNS TXT record at your DNS provider:          │
│                                                                      │
│  Record Name:  _acme-challenge.example.com                          │
│  Record Type:  TXT                                                   │
│  Record Value: dGVzdC12YWx1ZS1mb3ItYWNtZS1jaGFsbGVuZ2U=             │
│  TTL:          120 (or minimum allowed)                              │
│                                                                      │
│  ⏳ Waiting for confirmation...                                      │
│                                                                      │
│  [ Copy Record Value ]          [ I've Created the Record ]          │
└──────────────────────────────────────────────────────────────────────┘
```

#### Step 3: Add Record to DNS Provider

Log in to your DNS provider and create the TXT record:

**Example: Generic DNS Provider**

1. Navigate to DNS management for your domain
2. Click **"Add Record"** or **"New DNS Record"**
3. Configure:
   - **Type**: TXT
   - **Name/Host**: `_acme-challenge` (some providers auto-append domain)
   - **Value/Content**: The challenge token from Charon
   - **TTL**: 120 seconds (or minimum allowed)
4. Save the record

#### Step 4: Verify DNS Propagation

Before confirming, verify the record has propagated:

**Using dig command:**

```bash
dig TXT _acme-challenge.example.com +short
```

**Expected output:**

```
"dGVzdC12YWx1ZS1mb3ItYWNtZS1jaGFsbGVuZ2U="
```

**Using online tools:**

- [DNSChecker](https://dnschecker.org)
- [MXToolbox](https://mxtoolbox.com/TXTLookup.aspx)
- [WhatsMyDNS](https://whatsmydns.net)

#### Step 5: Confirm Record Creation

1. Return to Charon
2. Click **"I've Created the Record"**
3. Charon verifies the record and completes validation
4. Certificate is issued upon successful verification

#### Step 6: Cleanup (Automatic)

Charon displays instructions to remove the TXT record after certificate issuance. While optional, removing challenge records is recommended for cleaner DNS configuration.

### Manual Challenge Tips

- ✅ **Wait for propagation**: DNS changes can take 1-60 minutes to propagate globally
- ✅ **Check exact record name**: Some providers require `_acme-challenge`, others need `_acme-challenge.example.com`
- ✅ **Verify before confirming**: Use `dig` or online tools to confirm the record exists
- ✅ **Mind the TTL**: Lower TTL values speed up propagation but may not be supported by all providers
- ❌ **Don't include quotes**: The TXT value should be the raw token, not wrapped in quotes (unless your provider requires it)

---

## Troubleshooting

### Common Issues

#### DNS Propagation Delays

**Symptom**: Certificate request stuck at "Waiting for Propagation" or validation fails.

**Causes**:

- DNS TTL is high (cached old records)
- DNS provider has slow propagation
- Regional DNS inconsistency

**Solutions**:

1. **Verify the record exists locally:**

   ```bash
   dig TXT _acme-challenge.example.com @8.8.8.8
   ```

2. **Check multiple DNS servers:**

   ```bash
   dig TXT _acme-challenge.example.com @1.1.1.1
   dig TXT _acme-challenge.example.com @208.67.222.222
   ```

3. **Wait longer**: Some providers take up to 60 minutes for full propagation

4. **Lower TTL**: If possible, set TTL to 120 seconds or lower before requesting certificates

5. **Retry the request**: Cancel and retry after confirming DNS propagation

#### Invalid API Credentials

**Symptom**: "Authentication failed" or "Invalid credentials" error when testing connection.

**Solutions**:

| Provider | Common Issues |
|----------|---------------|
| **Cloudflare** | Token expired, wrong zone permissions, using Global API Key without email |
| **Route53** | IAM policy missing required actions, wrong region |
| **DigitalOcean** | Token has read-only scope (needs write) |
| **Google Cloud** | Wrong project ID, service account lacks DNS Admin role |

**Verification Steps**:

1. **Re-check credentials**: Copy-paste directly from provider, avoid manual typing
2. **Verify permissions**: Ensure API token/key has DNS edit permissions
3. **Test API directly**: Use provider's API documentation to test credentials independently
4. **Check for typos**: Especially in email addresses and project IDs

#### Permission Denied / Access Denied

**Symptom**: Connection test passes, but record creation fails.

**Causes**:

- API token has read-only permissions
- Zone/domain not accessible with current credentials
- Rate limiting or account restrictions

**Solutions**:

1. **Cloudflare**: Ensure token has "Zone:DNS:Edit" permission for the specific zone
2. **Route53**: Verify IAM policy includes `ChangeResourceRecordSets` action
3. **DigitalOcean**: Confirm token has "Write" scope
4. **Google Cloud**: Service account needs "DNS Administrator" role

#### DNS Record Already Exists

**Symptom**: "Record already exists" error during certificate request.

**Causes**:

- Previous challenge attempt left orphaned record
- Manual DNS record with same name exists
- Another ACME client managing the same domain

**Solutions**:

1. **Delete existing record**: Log in to DNS provider and remove the `_acme-challenge` TXT record
2. **Wait for deletion**: Allow time for deletion to propagate
3. **Retry certificate request**

#### CAA Record Issues

**Symptom**: Certificate Authority refuses to issue certificate despite successful DNS validation.

**Cause**: CAA (Certificate Authority Authorization) DNS records restrict which CAs can issue certificates.

**Solutions**:

1. **Check CAA records:**

   ```bash
   dig CAA example.com
   ```

2. **Add Let's Encrypt to CAA** (if CAA records exist):

   ```
   example.com.  CAA  0 issue "letsencrypt.org"
   example.com.  CAA  0 issuewild "letsencrypt.org"
   ```

3. **Remove restrictive CAA records** (if you don't need CAA enforcement)

#### Rate Limiting

**Symptom**: "Too many requests" or "Rate limit exceeded" errors.

**Causes**:

- Too many certificate requests in short period
- DNS provider API rate limits
- Let's Encrypt rate limits

**Solutions**:

1. **Wait and retry**: Most rate limits reset within 1 hour
2. **Use staging environment**: For testing, use Let's Encrypt staging to avoid production rate limits
3. **Consolidate domains**: Use SANs or wildcards to reduce certificate count
4. **Check provider limits**: Some DNS providers have low API rate limits

### DNS Provider-Specific Issues

#### Cloudflare

| Issue | Solution |
|-------|----------|
| "Invalid API Token" | Regenerate token with correct zone permissions |
| "Zone not found" | Ensure domain is active in Cloudflare account |
| "Rate limited" | Wait 5 minutes; Cloudflare allows 1200 requests/5 min |

#### Route53

| Issue | Solution |
|-------|----------|
| "Access Denied" | Check IAM policy includes all required actions |
| "NoSuchHostedZone" | Verify hosted zone ID is correct |
| "Throttling" | Implement exponential backoff; Route53 has strict rate limits |

#### DigitalOcean

| Issue | Solution |
|-------|----------|
| "Unable to authenticate" | Regenerate token with Write scope |
| "Domain not found" | Ensure domain is added to DigitalOcean DNS |

### Getting Help

If issues persist:

1. **Check Charon logs**: Look for detailed error messages in container logs
2. **Enable debug mode**: Set `LOG_LEVEL=debug` for verbose logging
3. **Search existing issues**: [GitHub Issues](https://github.com/Wikid82/charon/issues)
4. **Open a new issue**: Include Charon version, provider type, and sanitized error messages

---

## Related Documentation

### Feature Guides

- [DNS Provider Types](./dns-providers.md) — RFC 2136, Webhook, and Script providers
- [DNS Auto-Detection](./dns-auto-detection.md) — Automatic provider identification
- [Multi-Credential Support](./multi-credential.md) — Managing multiple credentials per provider
- [Key Rotation](./key-rotation.md) — Credential encryption and rotation

### General Documentation

- [Getting Started](../getting-started.md) — Initial Charon setup
- [Security Best Practices](../security.md) — Securing your Charon installation
- [API Reference](../api.md) — Programmatic certificate management
- [Troubleshooting Guide](../troubleshooting/) — General troubleshooting

### External Resources

- [Let's Encrypt Documentation](https://letsencrypt.org/docs/)
- [ACME Protocol RFC 8555](https://datatracker.ietf.org/doc/html/rfc8555)
- [DNS-01 Challenge Specification](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge)

---

*Last Updated: January 2026*
*Charon Version: 0.1.0-beta*
