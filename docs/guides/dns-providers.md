# DNS Providers Guide

## Overview

DNS providers enable Charon to obtain SSL/TLS certificates for wildcard domains (e.g., `*.example.com`) using the ACME DNS-01 challenge. This challenge proves domain ownership by creating a temporary TXT record in your DNS zone, which is required for wildcard certificates since HTTP-01 challenges cannot validate wildcards.

## Why DNS Providers Are Required

- **Wildcard Certificates:** ACME providers (like Let's Encrypt) require DNS-01 challenges for wildcard domains
- **Automated Validation:** Charon automatically creates and removes DNS records during certificate issuance
- **Secure Storage:** All credentials are encrypted at rest using AES-256-GCM encryption

## Supported DNS Providers

Charon dynamically discovers available DNS provider types from an internal registry. This registry includes:

- **Built-in providers** — Compiled into Charon (Cloudflare, Route 53, etc.)
- **Custom providers** — Special-purpose providers like `manual` for unsupported DNS services
- **External plugins** — Third-party `.so` plugin files loaded at runtime

### Built-in Providers

| Provider | Type | Setup Guide |
|----------|------|-------------|
| Cloudflare | `cloudflare` | [Cloudflare Setup](dns-providers/cloudflare.md) |
| AWS Route 53 | `route53` | [Route 53 Setup](dns-providers/route53.md) |
| DigitalOcean | `digitalocean` | [DigitalOcean Setup](dns-providers/digitalocean.md) |
| Google Cloud DNS | `googleclouddns` | [Documentation](https://caddyserver.com/docs/modules/dns.providers.googleclouddns) |
| Azure DNS | `azure` | [Documentation](https://caddyserver.com/docs/modules/dns.providers.azure) |
| Namecheap | `namecheap` | [Documentation](https://caddyserver.com/docs/modules/dns.providers.namecheap) |
| GoDaddy | `godaddy` | [Documentation](https://caddyserver.com/docs/modules/dns.providers.godaddy) |
| Hetzner | `hetzner` | [Documentation](https://caddyserver.com/docs/modules/dns.providers.hetzner) |
| Vultr | `vultr` | [Documentation](https://caddyserver.com/docs/modules/dns.providers.vultr) |
| DNSimple | `dnsimple` | [Documentation](https://caddyserver.com/docs/modules/dns.providers.dnsimple) |

### Custom Providers

| Provider | Type | Description |
|----------|------|-------------|
| Manual DNS | `manual` | For DNS providers without API support. Displays TXT record for manual creation. |

### Discovering Available Provider Types

Query available provider types programmatically via the API:

```bash
curl https://your-charon-instance/api/v1/dns-providers/types \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Example Response:**

```json
{
  "types": [
    {
      "type": "cloudflare",
      "name": "Cloudflare",
      "description": "Cloudflare DNS provider",
      "documentation_url": "https://developers.cloudflare.com/api/",
      "is_built_in": true,
      "fields": [...]
    },
    {
      "type": "manual",
      "name": "Manual DNS",
      "description": "Manually create DNS TXT records",
      "documentation_url": "",
      "is_built_in": false,
      "fields": []
    }
  ]
}
```

**Response fields:**

| Field | Description |
|-------|-------------|
| `type` | Unique identifier used in API requests |
| `name` | Human-readable display name |
| `description` | Brief description of the provider |
| `documentation_url` | Link to provider's API documentation |
| `is_built_in` | `true` for compiled providers, `false` for plugins/custom |
| `fields` | Required credential fields and their specifications |

> **Tip:** Use `is_built_in` to distinguish official providers from external plugins in your automation workflows.

## Adding External Plugins

Extend Charon with third-party DNS provider plugins by placing `.so` files in the plugin directory.

### Installation

1. Set the plugin directory environment variable:

   ```bash
   export CHARON_PLUGINS_DIR=/etc/charon/plugins
   ```

2. Copy plugin files:

   ```bash
   cp powerdns.so /etc/charon/plugins/
   chmod 755 /etc/charon/plugins/powerdns.so
   ```

3. Restart Charon — plugins load automatically at startup.

4. Verify the plugin appears in `GET /api/v1/dns-providers/types` with `is_built_in: false`.

For detailed plugin installation and security guidance, see [Custom Plugins](../features/custom-plugins.md).

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

For other providers, consult the official Caddy libdns module documentation linked in the table above.

## Related Documentation

- [Certificates Guide](certificates.md)
- [Proxy Hosts Guide](proxy-hosts.md)
- [DNS Challenges Troubleshooting](../troubleshooting/dns-challenges.md)
- [Security Best Practices](../security/best-practices.md)

## Additional Resources

- [Let's Encrypt DNS-01 Challenge Documentation](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge)
- [Caddy DNS Providers](https://caddyserver.com/docs/modules/)
- [ACME Protocol Specification](https://datatracker.ietf.org/doc/html/rfc8555)
