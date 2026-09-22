# DNS Providers Guide

## Overview

DNS providers let Charon get you a **wildcard certificate** — a single certificate that covers a whole domain and all its subdomains at once, like `*.example.com`. Normal certificates only cover one address at a time; a wildcard certificate means you don't need a new one every time you add a subdomain.

To prove you actually own the domain before issuing a wildcard certificate, Charon has to create a short-lived verification record in your domain's DNS settings (this is called a "DNS-01 challenge"). Doing that automatically requires Charon to be able to talk to your DNS provider's account — that's what the DNS Provider connections below are for.

## Why You'd Want This

- **One certificate, every subdomain:** Cover `*.example.com` instead of managing a separate certificate for each subdomain.
- **Fully automatic:** Once connected, Charon creates and removes the verification record for you — nothing to do by hand.
- **Your credentials stay safe:** Any API key or token you give Charon is encrypted before it's stored.

If you don't need wildcard domains, you can skip DNS providers entirely — Charon issues regular certificates automatically without any of this setup.

## Providers You Can Connect Today

These 10 providers are fully supported — connect one from the **DNS Providers** page in the Charon UI and you're ready to issue wildcard certificates.

| Provider | Setup Guide |
|----------|-------------|
| Cloudflare | [Cloudflare Setup](dns-providers/cloudflare.md) |
| AWS Route 53 | [Route 53 Setup](dns-providers/route53.md) |
| DigitalOcean | [DigitalOcean Setup](dns-providers/digitalocean.md) |
| Google Cloud DNS | [Documentation](https://caddyserver.com/docs/modules/dns.providers.googleclouddns) |
| Azure DNS | [Documentation](https://caddyserver.com/docs/modules/dns.providers.azure) |
| Namecheap | [Documentation](https://caddyserver.com/docs/modules/dns.providers.namecheap) |
| GoDaddy | [Documentation](https://caddyserver.com/docs/modules/dns.providers.godaddy) |
| Hetzner | [Documentation](https://caddyserver.com/docs/modules/dns.providers.hetzner) |
| Vultr | [Documentation](https://caddyserver.com/docs/modules/dns.providers.vultr) |
| DNSimple | [Documentation](https://caddyserver.com/docs/modules/dns.providers.dnsimple) |

There's also a **Manual** option for any provider not on this list: Charon shows you the verification record to create yourself, so you're never fully blocked even without direct integration.

## Other Providers (Coming Soon)

Don't see your DNS provider above? It's not supported yet, but more providers are on the roadmap. Track progress or let us know you're interested on [GitHub issue #1374](https://github.com/Wikid82/Charon/issues/1374).

In the meantime, the **Manual** provider option lets you complete a wildcard certificate for any domain by copying a verification record into your DNS provider's dashboard yourself.

## General Setup Workflow

### 1. Prerequisites

- Active account with a supported DNS provider
- Domain's DNS hosted with the provider
- API access enabled on your account
- Generated API credentials (tokens, keys, etc.)

### 2. Configure Encryption Key

DNS provider credentials are encrypted at rest. Before adding providers, ensure the encryption key is configured:

```bash
# Generate a 32-byte (256-bit) random key and encode as base64
openssl rand -base64 32

# Set as environment variable
export CHARON_ENCRYPTION_KEY="your-base64-encoded-key-here"
```

> **Warning:** The encryption key must be 32 bytes (44 characters in base64). Store it securely and back it up. If lost, you'll need to reconfigure all DNS providers.

Add to your Docker Compose or systemd configuration:

```yaml
# docker-compose.yml
services:
  charon:
    environment:
      - CHARON_ENCRYPTION_KEY=${CHARON_ENCRYPTION_KEY}
```

### 3. Add DNS Provider

1. Navigate to **DNS Providers** in the Charon UI
2. Click **Add Provider**
3. Select your DNS provider type
4. Enter a descriptive name (e.g., "Cloudflare Production")
5. Fill in the required credentials
6. (Optional) Adjust propagation timeout and polling interval
7. Click **Test Connection** to verify credentials
8. Click **Save**

### 4. Set Default Provider (Optional)

If you manage multiple domains across different DNS providers, you can designate one as the default. This will be pre-selected when creating new wildcard proxy hosts.

### 5. Create Wildcard Proxy Host

1. Navigate to **Proxy Hosts**
2. Click **Add Proxy Host**
3. Enter a wildcard domain (e.g., `*.example.com`)
4. Select your DNS provider from the dropdown
5. Configure other settings as needed
6. Save the proxy host

Charon will automatically use DNS-01 challenge for certificate issuance.

## Security Best Practices

### Credential Management

- **Least Privilege:** Create API tokens with minimum required permissions (DNS zone edit only)
- **Scope Tokens:** Limit tokens to specific DNS zones when supported by the provider
- **Rotate Regularly:** Periodically regenerate API tokens
- **Secure Storage:** Never commit credentials to version control

### Encryption Key

- **Backup:** Store the `CHARON_ENCRYPTION_KEY` in a secure password manager
- **Environment Variable:** Never hardcode the key in configuration files
- **Rotate Carefully:** Changing the key requires reconfiguring all DNS providers

### Network Security

- **Firewall Rules:** Ensure Charon can reach DNS provider APIs (typically HTTPS outbound)
- **Monitor Access:** Review API access logs in your DNS provider dashboard

## Configuration Options

### Propagation Timeout

Time (in seconds) to wait for DNS changes to propagate before ACME validation. Default: **120 seconds**.

- **Increase** if you experience validation failures due to slow DNS propagation
- **Decrease** if your DNS provider has fast global propagation (e.g., Cloudflare)

### Polling Interval

Time (in seconds) between checks for DNS record propagation. Default: **10 seconds**.

- Most users should keep the default value
- Adjust if hitting DNS provider API rate limits

## Troubleshooting

For detailed troubleshooting, see [DNS Challenges Troubleshooting](../troubleshooting/dns-challenges.md).

### Common Issues

**"Encryption key not configured"**

- Ensure `CHARON_ENCRYPTION_KEY` environment variable is set
- Restart Charon after setting the variable

**"Connection test failed"**

- Verify credentials are correct
- Check API token permissions
- Ensure firewall allows outbound HTTPS to provider
- Review provider-specific troubleshooting guides

**"DNS propagation timeout"**

- Increase propagation timeout in provider settings
- Verify DNS provider is authoritative for the domain
- Check provider status page for service issues

**"Certificate issuance failed"**

- Test DNS provider connection in UI
- Check Charon logs for detailed error messages
- Verify domain DNS is properly configured
- Ensure DNS provider has edit permissions for the zone

## Provider-Specific Guides

- [Cloudflare Setup Guide](dns-providers/cloudflare.md)
- [AWS Route 53 Setup Guide](dns-providers/route53.md)
- [DigitalOcean Setup Guide](dns-providers/digitalocean.md)

## Related Documentation

- Certificates Guide (guide not yet published)
- Proxy Hosts Guide (guide not yet published)
- [DNS Challenges Troubleshooting](../troubleshooting/dns-challenges.md)
- [Security Overview](../security.md)

## Additional Resources

- [Let's Encrypt DNS-01 Challenge Documentation](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge)
- [ACME Protocol Specification](https://datatracker.ietf.org/doc/html/rfc8555)
