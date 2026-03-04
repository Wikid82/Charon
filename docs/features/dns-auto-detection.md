# DNS Provider Auto-Detection

## Overview

DNS Provider Auto-Detection is an intelligent feature that automatically identifies which DNS provider manages your domain's nameservers. This helps streamline the setup process and reduces configuration errors when creating wildcard SSL certificate proxy hosts.

### Benefits

- **Reduce Configuration Errors**: Eliminates the risk of selecting the wrong DNS provider
- **Faster Setup**: No need to manually check your DNS registrar or control panel
- **Auto-Fill Provider Selection**: Automatically suggests the correct DNS provider in proxy host forms
- **Reduced Support Burden**: Fewer configuration issues to troubleshoot

### When Detection Occurs

Auto-detection runs automatically when you:

- Enter a wildcard domain (`*.example.com`) in the proxy host creation form
- The domain requires DNS-01 challenge validation for Let's Encrypt SSL certificates

## How Auto-Detection Works

### Detection Process

1. **Nameserver Lookup**: System performs a DNS query to retrieve the authoritative nameservers for your domain
2. **Pattern Matching**: Compares nameserver hostnames against known provider patterns
3. **Confidence Assessment**: Assigns a confidence level based on match quality
4. **Provider Suggestion**: Suggests configured DNS providers that match the detected type
5. **Caching**: Results are cached for 1 hour to improve performance

### Confidence Levels

| Level | Description | Action Required |
|-------|-------------|-----------------|
| **High** | Exact match with known provider pattern | Safe to use auto-detected provider |
| **Medium** | Partial match or common pattern | Verify provider before using |
| **Low** | Weak match or ambiguous pattern | Manually verify provider selection |
| **None** | No matching pattern found | Manual provider selection required |

### Caching Behavior

- Detection results are cached for **1 hour**
- Reduces DNS query load and improves response time
- Cache is invalidated when manually changing provider
- Each domain is cached independently

## Using Auto-Detection

### Automatic Detection

When creating a new proxy host with a wildcard domain:

1. Enter your wildcard domain in the **Domain Names** field (e.g., `*.example.com`)
2. The system automatically performs nameserver lookup
3. Detection results appear in the **DNS Provider** section
4. If a match is found, the provider is automatically selected

**Visual Indicator**: A detection status badge appears next to the DNS Provider dropdown showing:

- ✓ Provider detected
- ⚠ No provider detected
- ℹ Multiple nameservers found

### Manual Detection

If auto-detection doesn't run automatically or you want to recheck:

1. Click the **Detect Provider** button next to the DNS Provider dropdown
2. System performs fresh nameserver lookup (bypasses cache)
3. Results update immediately

> **Note**: Manual detection is useful after changing nameservers at your DNS provider.

### Reviewing Detection Results

The detection results panel displays:

| Field | Description |
|-------|-------------|
| **Status** | Whether provider was detected |
| **Detected Provider Type** | DNS provider identified (e.g., "cloudflare") |
| **Confidence** | Detection confidence level |
| **Nameservers** | List of authoritative nameservers found |
| **Suggested Provider** | Configured provider that matches detected type |

### Manual Override

You can always override auto-detection:

1. Select a different provider from the **DNS Provider** dropdown
2. Your selection takes precedence over auto-detection
3. System uses your selected provider credentials

> **Warning**: Using the wrong provider will cause SSL certificate issuance to fail.

## Detection Results Explained

### Example 1: Successful Detection

```
Domain: *.example.com

Detection Results:
✓ Provider Detected

Detected Provider Type: cloudflare
Confidence: High
Nameservers:
  - ns1.cloudflare.com
  - ns2.cloudflare.com

Suggested Provider: "Production Cloudflare"
```

**Action**: Use the suggested provider with confidence.

### Example 2: No Match Found

```
Domain: *.internal.company.com

Detection Results:
⚠ No Provider Detected

Nameservers:
  - ns1.internal.company.com
  - ns2.internal.company.com

Confidence: None
```

**Action**: Manually select the appropriate DNS provider or configure a custom provider.

### Example 3: Multiple Providers (Rare)

```
Domain: *.example.com

Detection Results:
⚠ Multiple Providers Detected

Detected Types:
  - cloudflare (2 nameservers)
  - route53 (1 nameserver)

Confidence: Medium
```

**Action**: Verify your domain's nameserver configuration at your DNS registrar. Mixed providers are uncommon and may indicate a configuration issue.

## Supported DNS Providers

The system recognizes the following DNS providers by their nameserver patterns:

| Provider | Nameserver Pattern | Example Nameserver |
|----------|-------------------|-------------------|
| **Cloudflare** | `*.ns.cloudflare.com` | `ns1.cloudflare.com` |
| **AWS Route 53** | `*.awsdns*` | `ns-123.awsdns-12.com` |
| **DigitalOcean** | `*.digitalocean.com` | `ns1.digitalocean.com` |
| **Google Cloud DNS** | `*.googledomains.com`, `ns-cloud*` | `ns-cloud-a1.googledomains.com` |
| **Azure DNS** | `*.azure-dns*` | `ns1-01.azure-dns.com` |
| **Namecheap** | `*.registrar-servers.com` | `dns1.registrar-servers.com` |
| **GoDaddy** | `*.domaincontrol.com` | `ns01.domaincontrol.com` |
| **Hetzner** | `*.hetzner.com`, `*.hetzner.de` | `helium.ns.hetzner.com` |
| **Vultr** | `*.vultr.com` | `ns1.vultr.com` |
| **DNSimple** | `*.dnsimple.com` | `ns1.dnsimple.com` |

### Provider-Specific Examples

#### Cloudflare

```
Nameservers:
  ns1.cloudflare.com
  ns2.cloudflare.com

Detected: cloudflare (High confidence)
```

#### AWS Route 53

```
Nameservers:
  ns-1234.awsdns-12.com
  ns-5678.awsdns-34.net

Detected: route53 (High confidence)
```

#### Google Cloud DNS

```
Nameservers:
  ns-cloud-a1.googledomains.com
  ns-cloud-a2.googledomains.com

Detected: googleclouddns (High confidence)
```

#### DigitalOcean

```
Nameservers:
  ns1.digitalocean.com
  ns2.digitalocean.com
  ns3.digitalocean.com

Detected: digitalocean (High confidence)
```

### Unsupported Providers

If your DNS provider isn't listed above:

1. **Custom/Internal DNS**: You'll need to manually select a provider that uses the same API (e.g., many providers use Cloudflare's API)
2. **New Provider**: Request support by opening a GitHub issue with your provider's nameserver pattern
3. **Workaround**: Configure a supported provider that's API-compatible, or use a different DNS provider for wildcard domains

## Manual Override Scenarios

### When to Override Auto-Detection

Override auto-detection when:

1. **Multiple Credentials**: You have multiple configured providers of the same type (e.g., "Dev Cloudflare" and "Prod Cloudflare")
2. **API-Compatible Providers**: Using a provider that shares an API with a detected provider
3. **Custom DNS Servers**: Running custom DNS infrastructure that mimics provider nameservers
4. **Testing**: Deliberately testing with different credentials

### How to Override

1. Ignore the auto-detected provider suggestion
2. Select your preferred provider from the **DNS Provider** dropdown
3. Save the proxy host with your selection
4. System will use your selected credentials

> **Important**: Ensure your selected provider has valid API credentials and permissions to modify DNS records for the domain.

### Custom Nameservers

For custom or internal nameservers:

1. Detection will likely return "No Provider Detected"
2. You must manually select a provider
3. Ensure the selected provider type matches your DNS server's API
4. Configure appropriate API credentials in the DNS Provider settings

Example:

```
Domain: *.corp.internal
Nameservers: ns1.corp.internal, ns2.corp.internal

Auto-detection: None
Manual selection required: Select compatible provider or configure custom
```

## Troubleshooting

### Detection Failed: Domain Not Found

**Symptom**: Error message "Failed to detect DNS provider" or "Domain not found"

**Causes**:

- Domain doesn't exist yet
- Domain not propagated to public DNS
- DNS resolution blocked by firewall

**Solutions**:

- Verify domain exists and is registered
- Wait for DNS propagation (up to 48 hours)
- Check network connectivity and DNS resolution
- Manually select provider and proceed

### Wrong Provider Detected

**Symptom**: System detects incorrect provider type

**Causes**:

- Domain using DNS proxy/forwarding service
- Recent nameserver change not yet propagated
- Multiple providers in nameserver list

**Solutions**:

- Wait for DNS propagation (up to 24 hours)
- Manually override provider selection
- Verify nameservers at your domain registrar
- Use manual detection to refresh results

### Multiple Providers Detected

**Symptom**: Detection shows multiple provider types

**Causes**:

- Nameservers from different providers (unusual)
- DNS migration in progress
- Misconfigured nameservers

**Solutions**:

- Check nameserver configuration at your registrar
- Complete DNS migration to single provider
- Manually select the primary/correct provider
- Contact DNS provider support if configuration is correct

### No DNS Provider Configured for Detected Type

**Symptom**: Provider detected but no matching provider configured in system

**Example**:

```
Detected Provider Type: cloudflare
Error: No DNS provider of type 'cloudflare' is configured
```

**Solutions**:

1. Navigate to **Settings** → **DNS Providers**
2. Click **Add DNS Provider**
3. Select the detected provider type (e.g., Cloudflare)
4. Enter API credentials:
   - Cloudflare: API Token or Global API Key + Email
   - Route 53: Access Key ID + Secret Access Key
   - DigitalOcean: API Token
   - (See provider-specific documentation)
5. Save provider configuration
6. Return to proxy host creation and retry

> **Tip**: You can configure multiple providers of the same type with different names (e.g., "Dev Cloudflare" and "Prod Cloudflare").

### Custom/Internal DNS Servers Not Detected

**Symptom**: Using private/internal DNS, no provider detected

**This is expected behavior**. Custom DNS servers don't match public provider patterns.

**Solutions**:

1. Manually select a provider that uses a compatible API
2. If using BIND, PowerDNS, or other custom DNS:
   - Configure acme.sh or certbot direct integration
   - Use supported provider API if available
   - Consider using supported DNS provider for wildcard domains only
3. If no compatible API:
   - Use HTTP-01 challenge instead (no wildcard support)
   - Configure manual DNS challenge workflow

### Detection Caching Issues

**Symptom**: Detection results don't reflect recent nameserver changes

**Cause**: Results cached for 1 hour

**Solutions**:

- Wait up to 1 hour for cache to expire
- Use **Detect Provider** button for manual detection (bypasses cache)
- DNS propagation may also take additional time (separate from caching)

## API Reference

### Detection Endpoint

Auto-detection is exposed via REST API for automation and integrations.

#### Endpoint

```
POST /api/dns-providers/detect
```

#### Authentication

Requires API token with `dns_providers:read` permission.

```http
Authorization: Bearer YOUR_API_TOKEN
```

#### Request Body

```json
{
  "domain": "*.example.com"
}
```

**Parameters**:

- `domain` (required): Full domain name including wildcard (e.g., `*.example.com`)

#### Response: Success

```json
{
  "status": "detected",
  "provider_type": "cloudflare",
  "confidence": "high",
  "nameservers": [
    "ns1.cloudflare.com",
    "ns2.cloudflare.com"
  ],
  "suggested_provider_id": 42,
  "suggested_provider_name": "Production Cloudflare",
  "cached": false
}
```

**Response Fields**:

- `status`: `"detected"` or `"not_detected"`
- `provider_type`: Detected provider type (string) or `null`
- `confidence`: `"high"`, `"medium"`, `"low"`, or `"none"`
- `nameservers`: Array of authoritative nameservers (strings)
- `suggested_provider_id`: Database ID of matching configured provider (integer or `null`)
- `suggested_provider_name`: Display name of matching provider (string or `null`)
- `cached`: Whether result is from cache (boolean)

#### Response: Not Detected

```json
{
  "status": "not_detected",
  "provider_type": null,
  "confidence": "none",
  "nameservers": [
    "ns1.custom-dns.com",
    "ns2.custom-dns.com"
  ],
  "suggested_provider_id": null,
  "suggested_provider_name": null,
  "cached": false
}
```

#### Response: Error

```json
{
  "error": "Failed to resolve nameservers for domain",
  "details": "NXDOMAIN: domain does not exist"
}
```

**HTTP Status Codes**:

- `200 OK`: Detection completed successfully
- `400 Bad Request`: Invalid domain format
- `401 Unauthorized`: Missing or invalid API token
- `500 Internal Server Error`: DNS resolution or server error

#### Example: cURL

```bash
curl -X POST https://charon.example.com/api/dns-providers/detect \
  -H "Authorization: Bearer YOUR_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "*.example.com"
  }'
```

#### Example: JavaScript

```javascript
async function detectDNSProvider(domain) {
  const response = await fetch('/api/dns-providers/detect', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${apiToken}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ domain })
  });

  const result = await response.json();

  if (result.status === 'detected') {
    console.log(`Detected: ${result.provider_type} (${result.confidence})`);
    console.log(`Nameservers: ${result.nameservers.join(', ')}`);
  } else {
    console.log('No provider detected');
  }

  return result;
}

// Usage
detectDNSProvider('*.example.com');
```

#### Example: Python

```python
import requests

def detect_dns_provider(domain: str, api_token: str) -> dict:
    response = requests.post(
        'https://charon.example.com/api/dns-providers/detect',
        headers={
            'Authorization': f'Bearer {api_token}',
            'Content-Type': 'application/json'
        },
        json={'domain': domain}
    )
    response.raise_for_status()
    return response.json()

# Usage
result = detect_dns_provider('*.example.com', 'YOUR_API_TOKEN')
if result['status'] == 'detected':
    print(f"Detected: {result['provider_type']} ({result['confidence']})")
    print(f"Nameservers: {', '.join(result['nameservers'])}")
else:
    print('No provider detected')
```

## Best Practices

### General Recommendations

1. **Trust High Confidence**: High-confidence detections are highly reliable
2. **Verify Medium/Low**: Always verify medium or low confidence detections before using
3. **Manual Override When Needed**: Don't hesitate to override if detection seems incorrect
4. **Keep Providers Updated**: Ensure DNS provider API credentials are current
5. **Monitor Detection**: Track detection success rates in your environment

### For Multiple Environments

When managing multiple environments (dev, staging, production):

1. Use descriptive provider names: "Dev Cloudflare", "Prod Cloudflare"
2. Auto-detection will suggest the first matching provider by default
3. Always verify the suggested provider matches your intended environment
4. Consider using different DNS providers per environment to avoid confusion

### For Enterprise/Internal DNS

If using custom enterprise DNS infrastructure:

1. Document which Charon DNS provider type is compatible with your system
2. Create named providers for each environment/purpose
3. Train users to ignore auto-detection for internal domains
4. Consider maintaining a mapping document of internal domains to correct providers

### For Multi-Credential Setups

When using multiple credentials for the same provider:

1. Name providers clearly: "Cloudflare - Account A", "Cloudflare - Account B"
2. Document which domains belong to which account
3. Always review auto-detected suggestions carefully
4. Use manual override to select the correct credential set

## Related Documentation

- [DNS Provider Configuration](../guides/dns-providers.md) - Setting up DNS provider credentials
- [Multi-Credential DNS Support](./multi-credential-dns.md) - Managing multiple providers of same type
- [Proxy Host Creation](../guides/proxy-hosts.md) - Creating wildcard SSL proxy hosts
- [SSL Certificate Management](../guides/ssl-certificates.md) - Let's Encrypt and certificate issuance
- [Troubleshooting DNS Issues](../troubleshooting/dns-problems.md) - Common DNS configuration problems

## Support

If you encounter issues with DNS Provider Auto-Detection:

1. Check the [Troubleshooting](#troubleshooting) section above
2. Review [GitHub Issues](https://github.com/yourusername/charon/issues) for similar problems
3. Open a new issue with:
   - Domain name (sanitized if sensitive)
   - Detected provider (if any)
   - Expected provider
   - Nameservers returned
   - Error messages or logs

---

**Last Updated**: January 2026
**Feature Version**: 0.1.6-beta.0+
**Status**: Production Ready
