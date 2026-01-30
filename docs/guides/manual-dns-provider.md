# Manual DNS Provider Guide

## Overview

The Manual DNS Provider allows you to obtain SSL/TLS certificates using the ACME DNS-01 challenge when your DNS provider is not directly supported by Charon. Instead of automating DNS record creation, Charon displays the required TXT record details for you to create manually at your DNS provider.

### When to Use Manual DNS

Use the Manual DNS Provider when:

- Your DNS provider is not in the [supported providers list](dns-providers.md)
- You need a one-time certificate for testing or development
- You want to verify your DNS setup before configuring automated providers
- Your organization requires manual approval for DNS changes

### How It Works

1. You request a certificate for your domain (e.g., `*.example.com`)
2. Charon generates the DNS challenge and displays the TXT record details
3. You create the TXT record at your DNS provider
4. You click "Verify" to confirm the record exists
5. Charon completes the ACME challenge and issues the certificate

## Prerequisites

Before using the Manual DNS Provider, ensure you have:

- **DNS Management Access:** Login credentials for your DNS provider's control panel
- **Domain Ownership:** Administrative access to the domain you want to secure
- **Time Availability:** The challenge must be completed within **10 minutes**
- **Charon Setup:** A running Charon instance with the encryption key configured

## Setup Guide

### Step 1: Add the Manual DNS Provider

1. Log in to your Charon dashboard
2. Navigate to **Settings** → **DNS Providers**
3. Click **Add Provider**
4. Select **Manual (No Automation)** from the provider list

### Step 2: Configure Provider Settings

Fill in the configuration form:

| Field | Description | Recommended Value |
|-------|-------------|-------------------|
| **Provider Name** | A descriptive name for this provider | "Manual DNS" |
| **Challenge Timeout** | Time (in minutes) to complete the challenge | 10 |

Click **Save** to create the provider.

### Step 3: Create a Proxy Host with Manual DNS

1. Navigate to **Proxy Hosts**
2. Click **Add Proxy Host**
3. Enter your domain (e.g., `*.example.com` for wildcard)
4. Select your **Manual DNS** provider
5. Configure other proxy settings as needed
6. Click **Save**

Charon will begin the certificate request and display the Manual DNS Challenge interface.

## Using Manual DNS Challenges

### Understanding the Challenge Interface

When you request a certificate using the Manual DNS provider, Charon displays a challenge screen:

```
┌─────────────────────────────────────────────────────────────────────┐
│                   Manual DNS Challenge                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Certificate Request: *.example.com                                  │
│                                                                      │
│  CREATE THIS TXT RECORD AT YOUR DNS PROVIDER:                        │
│                                                                      │
│  Record Name:  _acme-challenge.example.com              [Copy]       │
│  Record Type:  TXT                                                   │
│  Record Value: gZrH7wL9t3kM2nP4qX5yR8sT0uV1wZ2a...      [Copy]       │
│  TTL:          300 (5 minutes)                                       │
│                                                                      │
│  Time Remaining: 7:23                                                │
│  [━━━━━━━━━━━━━━━━░░░░░░░░░░░░░░░░] 73%                              │
│                                                                      │
│  [Check DNS Now]        [I've Created the Record - Verify]           │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

**Key Elements:**

- **Record Name:** The full DNS name where you create the TXT record
- **Record Value:** The token value that proves domain ownership
- **Time Remaining:** Countdown until the challenge expires
- **Copy Buttons:** Click to copy values to your clipboard

### Step-by-Step: Creating the TXT Record

Follow these steps to complete the challenge:

1. **Copy the Record Name**
   - Click the **Copy** button next to the Record Name
   - This copies `_acme-challenge.example.com` to your clipboard

2. **Copy the Record Value**
   - Click the **Copy** button next to the Record Value
   - This copies the challenge token to your clipboard

3. **Log in to Your DNS Provider**
   - Open your DNS provider's control panel in a new browser tab
   - Navigate to the DNS management section for your domain

4. **Create a New TXT Record**
   - Click "Add Record" or similar button
   - Select **TXT** as the record type
   - Paste the Record Name (or just `_acme-challenge` depending on your provider)
   - Paste the Record Value
   - Set TTL to **300** seconds (5 minutes) or the lowest available option

5. **Save the DNS Record**
   - Confirm and save the new TXT record
   - Wait a few seconds for the change to process

### Provider-Specific Instructions

Different DNS providers have different interfaces. Here are common patterns:

#### Cloudflare (Manual)

1. Go to **DNS** → **Records**
2. Click **Add record**
3. Type: `TXT`
4. Name: `_acme-challenge` (Cloudflare adds the domain automatically)
5. Content: Paste the challenge token
6. TTL: `Auto` or `5 min`

#### GoDaddy

1. Go to **DNS Management**
2. Click **Add** in the Records section
3. Type: `TXT`
4. Host: `_acme-challenge`
5. TXT Value: Paste the challenge token
6. TTL: `1/2 Hour` (minimum)

#### Namecheap

1. Go to **Advanced DNS**
2. Click **Add New Record**
3. Type: `TXT Record`
4. Host: `_acme-challenge`
5. Value: Paste the challenge token
6. TTL: `Automatic`

#### Generic Providers

Most providers follow this pattern:

| Field | What to Enter |
|-------|---------------|
| Type | TXT |
| Host/Name | `_acme-challenge` or full `_acme-challenge.yourdomain.com` |
| Value/Content | The challenge token from Charon |
| TTL | 300 or lowest available |

### Verifying the Challenge

After creating the TXT record:

1. **Wait for Propagation**
   - DNS changes can take 30 seconds to several minutes to propagate
   - The "Check DNS Now" button lets you verify without triggering the full challenge

2. **Click "Check DNS Now" (Optional)**
   - Charon queries DNS to see if your record exists
   - Status updates to show if the record was found

3. **Click "I've Created the Record - Verify"**
   - Charon sends the verification to the ACME server
   - If successful, the certificate is issued
   - If the record is not found, you can try again (within the time limit)

### Challenge Status Messages

| Status | Meaning | Action |
|--------|---------|--------|
| **Pending** | Waiting for you to create the DNS record | Create the TXT record |
| **Checking DNS** | Charon is verifying the record exists | Wait for result |
| **DNS Found** | Record detected, verifying with ACME | Wait for completion |
| **Verified** | Challenge completed successfully | Certificate issued! |
| **Expired** | Time limit exceeded | Start a new challenge |
| **Failed** | Verification failed | Check record and retry |

## Troubleshooting Common Issues

### "DNS record not found"

**Possible Causes:**

1. **Typo in record name or value**
   - Double-check you copied the exact values from Charon
   - Some providers require just `_acme-challenge`, others need the full domain

2. **DNS propagation delay**
   - Wait 1-2 minutes and try "Check DNS Now" again
   - Use [DNS Checker](https://dnschecker.org/) to verify propagation

3. **Wrong DNS zone**
   - Ensure you're editing the correct domain's DNS
   - For subdomains, the record goes in the parent zone

**Solution:**

```bash
# Verify your record from command line
dig TXT _acme-challenge.example.com +short

# Expected output: Your challenge token in quotes
"gZrH7wL9t3kM2nP4qX5yR8sT0uV1wZ2aB3cD4eF5gH6i"
```

### "Challenge expired"

**Cause:** The 10-minute time limit was exceeded.

**Solution:**

1. Click **Cancel Challenge** or wait for it to clear
2. Start a new certificate request
3. Have your DNS provider's control panel ready before starting
4. Create the record immediately after copying the values

### "Challenge already in progress"

**Cause:** Another challenge is active for the same domain.

**Solution:**

1. Wait for the existing challenge to complete or expire
2. If you started the challenge, navigate to the pending challenge screen
3. Complete or cancel the existing challenge before starting a new one

### "Verification failed"

**Possible Causes:**

1. **Record value mismatch**
   - Ensure no extra spaces or characters in the TXT value
   - Some providers add quotes automatically; don't add your own

2. **Wrong record type**
   - Must be a TXT record, not CNAME or other type

3. **Cached old record**
   - If you had a previous challenge, the old record might be cached
   - Delete any existing `_acme-challenge` records before creating new ones

**Solution:**

1. Delete the existing TXT record
2. Wait 2 minutes for cache to clear
3. Create a new record with the exact values from Charon
4. Click "Verify" again

### DNS Provider Rate Limits

Some providers limit how frequently you can modify DNS records.

**Symptoms:**

- "Too many requests" error
- Changes not appearing immediately
- API errors in provider dashboard

**Solution:**

1. Wait 5-10 minutes before retrying
2. Contact your DNS provider if issues persist
3. Consider using a provider with better API limits for frequent certificate operations

## Limitations

### Auto-Renewal Not Supported

> **Important:** The Manual DNS Provider **does not support automatic certificate renewal**.

When your certificate approaches expiration:

1. You will receive a notification (if notifications are configured)
2. You must manually initiate a new certificate request
3. You must complete the DNS challenge again

**Recommendation:** Use the Manual DNS Provider only for:

- Initial testing and verification
- One-time certificates
- Domains where you plan to migrate to an automated provider

For production use with automatic renewal, consider:

- [Supported DNS Providers](dns-providers.md)
- [Webhook DNS Provider](../features/webhook-dns.md) for custom integrations
- [RFC 2136 Provider](../features/rfc2136-dns.md) for self-hosted DNS

### Challenge Timeout

The DNS challenge must be completed within **10 minutes** (default). This includes:

- Creating the TXT record
- Waiting for DNS propagation
- Clicking "Verify"

If you frequently run out of time:

1. Have your DNS provider control panel open before starting
2. Use a provider with faster propagation
3. Consider a different approach for complex setups

### Single Challenge at a Time

Only one manual challenge can be active per domain (FQDN) at a time. If you need certificates for multiple domains, complete each challenge sequentially.

## Frequently Asked Questions

### Can I use Manual DNS for production certificates?

Yes, but with caveats. The certificate itself is the same as those obtained through automated providers. However, you must remember to manually renew before expiration. For production systems, automated renewal is strongly recommended.

### How long does DNS propagation take?

DNS propagation typically takes:

- **Cloudflare:** Near-instant (seconds)
- **Most providers:** 30 seconds to 2 minutes
- **Some providers:** Up to 5-10 minutes

The Manual DNS Provider's 10-minute timeout accommodates most scenarios.

### Can I use a shorter TTL?

Yes. Lower TTL values (60-300 seconds) help because:

- Changes propagate faster
- Cached records expire sooner if you need to retry

Set the TTL to the lowest value your provider allows.

### What happens if I enter the wrong value?

The verification will fail with "DNS record not found" or "Verification failed." Simply:

1. Delete the incorrect TXT record
2. Create a new record with the correct value
3. Click "Verify" again (if time permits)

### Can I use Manual DNS for multi-domain certificates?

Yes, but each domain requires its own TXT record. For a certificate covering `example.com` and `www.example.com`:

1. Charon displays challenges for each domain
2. Create TXT records for each `_acme-challenge` subdomain
3. Verify each challenge in sequence

### Is the Manual DNS Provider secure?

Yes. The Manual DNS Provider:

- Uses the same ACME protocol as automated providers
- Encrypts all data at rest
- Requires authentication for all operations
- Logs all challenge activity for auditing

The security of your certificate depends on:

- Protecting your DNS provider credentials
- Not sharing challenge tokens publicly
- Completing challenges promptly

### How do I delete a Manual DNS Provider?

1. Navigate to **Settings** → **DNS Providers**
2. Find your Manual DNS provider in the list
3. Ensure no proxy hosts are using it (migrate them first)
4. Click the **Delete** button
5. Confirm deletion

## Related Documentation

- [DNS Providers Overview](dns-providers.md)
- [Certificates Guide](certificates.md)
- [DNS Challenges Troubleshooting](../troubleshooting/dns-challenges.md)
- [Custom DNS Plugins](../features/custom-plugins.md)

## Getting Help

If you encounter issues not covered in this guide:

1. Check the [Troubleshooting Guide](../troubleshooting/dns-challenges.md)
2. Search [GitHub Discussions](https://github.com/Wikid82/charon/discussions)
3. Open an issue with:
   - Your Charon version
   - DNS provider name
   - Error messages
   - Steps you've tried
