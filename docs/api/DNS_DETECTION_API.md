# DNS Provider Auto-Detection API Reference

## Quick Start

The DNS Provider Auto-Detection API automatically identifies DNS providers by analyzing nameserver records.

## Authentication

All endpoints require authentication via Bearer token:

```http
Authorization: Bearer <your-jwt-token>
```

---

## Endpoints

### 1. Detect DNS Provider

Analyzes a domain's nameservers and identifies the DNS provider.

**Endpoint:** `POST /api/v1/dns-providers/detect`

**Request Body:**

```json
{
  "domain": "example.com"
}
```

**Response (Success - Provider Detected):**

```json
{
  "domain": "example.com",
  "detected": true,
  "provider_type": "cloudflare",
  "nameservers": [
    "ns1.cloudflare.com",
    "ns2.cloudflare.com"
  ],
  "confidence": "high",
  "suggested_provider": {
    "id": 1,
    "uuid": "abc-123-def-456",
    "name": "Production Cloudflare",
    "provider_type": "cloudflare",
    "enabled": true,
    "is_default": true,
    "propagation_timeout": 120,
    "polling_interval": 5,
    "success_count": 42,
    "failure_count": 0,
    "created_at": "2026-01-01T00:00:00Z",
    "updated_at": "2026-01-01T00:00:00Z"
  }
}
```

**Response (Provider Not Detected):**

```json
{
  "domain": "custom-provider.com",
  "detected": false,
  "nameservers": [
    "ns1.custom-provider.com",
    "ns2.custom-provider.com"
  ],
  "confidence": "none"
}
```

**Response (DNS Lookup Error):**

```json
{
  "domain": "nonexistent.tld",
  "detected": false,
  "nameservers": [],
  "confidence": "none",
  "error": "DNS lookup failed: no such host"
}
```

**Confidence Levels:**

- `high`: ≥80% of nameservers matched known patterns
- `medium`: 50-79% matched
- `low`: 1-49% matched
- `none`: No matches found

---

### 2. Get Detection Patterns

Returns the list of all built-in nameserver patterns used for detection.

**Endpoint:** `GET /api/v1/dns-providers/detection-patterns`

**Response:**

```json
{
  "patterns": [
    {
      "pattern": "cloudflare.com",
      "provider_type": "cloudflare"
    },
    {
      "pattern": "awsdns",
      "provider_type": "route53"
    },
    {
      "pattern": "digitalocean.com",
      "provider_type": "digitalocean"
    },
    {
      "pattern": "googledomains.com",
      "provider_type": "googleclouddns"
    },
    {
      "pattern": "ns-cloud",
      "provider_type": "googleclouddns"
    },
    {
      "pattern": "azure-dns",
      "provider_type": "azure"
    },
    {
      "pattern": "registrar-servers.com",
      "provider_type": "namecheap"
    },
    {
      "pattern": "domaincontrol.com",
      "provider_type": "godaddy"
    },
    {
      "pattern": "hetzner.com",
      "provider_type": "hetzner"
    },
    {
      "pattern": "hetzner.de",
      "provider_type": "hetzner"
    },
    {
      "pattern": "vultr.com",
      "provider_type": "vultr"
    },
    {
      "pattern": "dnsimple.com",
      "provider_type": "dnsimple"
    }
  ],
  "total": 12
}
```

---

## Supported Providers

The detection system recognizes these DNS providers:

| Provider | Pattern Examples |
|----------|------------------|
| **Cloudflare** | `ns1.cloudflare.com`, `ns2.cloudflare.com` |
| **AWS Route 53** | `ns-123.awsdns-45.com`, `ns-456.awsdns-78.net` |
| **DigitalOcean** | `ns1.digitalocean.com`, `ns2.digitalocean.com` |
| **Google Cloud DNS** | `ns-cloud-a1.googledomains.com` |
| **Azure DNS** | `ns1-01.azure-dns.com` |
| **Namecheap** | `dns1.registrar-servers.com` |
| **GoDaddy** | `ns01.domaincontrol.com` |
| **Hetzner** | `hydrogen.ns.hetzner.com` |
| **Vultr** | `ns1.vultr.com` |
| **DNSimple** | `ns1.dnsimple.com` |

---

## Usage Examples

### cURL

```bash
# Detect provider
curl -X POST \
  https://your-charon-instance.com/api/v1/dns-providers/detect \
  -H 'Authorization: Bearer your-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "domain": "example.com"
  }'

# Get detection patterns
curl -X GET \
  https://your-charon-instance.com/api/v1/dns-providers/detection-patterns \
  -H 'Authorization: Bearer your-token'
```

### JavaScript/TypeScript

```typescript
// Detection API client
async function detectDNSProvider(domain: string): Promise<DetectionResult> {
  const response = await fetch('/api/v1/dns-providers/detect', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify({ domain })
  });

  if (!response.ok) {
    throw new Error('Detection failed');
  }

  return response.json();
}

// Usage
try {
  const result = await detectDNSProvider('example.com');

  if (result.detected && result.suggested_provider) {
    console.log(`Provider: ${result.suggested_provider.name}`);
    console.log(`Confidence: ${result.confidence}`);
  } else {
    console.log('Provider not recognized');
  }
} catch (error) {
  console.error('Detection error:', error);
}
```

### Python

```python
import requests

def detect_dns_provider(domain: str, token: str) -> dict:
    """Detect DNS provider for a domain."""
    response = requests.post(
        'https://your-charon-instance.com/api/v1/dns-providers/detect',
        headers={
            'Authorization': f'Bearer {token}',
            'Content-Type': 'application/json'
        },
        json={'domain': domain}
    )
    response.raise_for_status()
    return response.json()

# Usage
try:
    result = detect_dns_provider('example.com', 'your-token')

    if result['detected']:
        provider = result.get('suggested_provider')
        if provider:
            print(f"Provider: {provider['name']}")
            print(f"Confidence: {result['confidence']}")
    else:
        print('Provider not recognized')
except requests.HTTPError as e:
    print(f'Detection failed: {e}')
```

---

## Wildcard Domains

The API automatically handles wildcard domain prefixes:

```json
{
  "domain": "*.example.com"
}
```

The wildcard prefix (`*.`) is automatically removed before DNS lookup, so the response will show:

```json
{
  "domain": "example.com",
  ...
}
```

---

## Caching

Detection results are cached for **1 hour** to:

- Reduce DNS lookup overhead
- Improve response times
- Minimize external DNS queries

Failed lookups (DNS errors) are cached for **5 minutes** only.

**Cache Characteristics:**

- Cache hits: <1ms response time
- Cache misses: 100-200ms (typical DNS lookup)
- Thread-safe implementation
- Automatic expiration cleanup

---

## Error Handling

### Client Errors (4xx)

**400 Bad Request:**

```json
{
  "error": "domain is required"
}
```

**401 Unauthorized:**

```json
{
  "error": "invalid or missing token"
}
```

### Server Errors (5xx)

**500 Internal Server Error:**

```json
{
  "error": "Failed to detect DNS provider"
}
```

---

## Rate Limiting

The API uses built-in rate limiting through:

- **DNS Lookup Timeout:** 10 seconds maximum per request
- **Caching:** Reduces repeated lookups for same domain
- **Authentication:** Required for all endpoints

No explicit rate limiting is applied beyond authentication requirements.

---

## Performance

- **Typical Detection Time:** 100-200ms
- **Maximum Detection Time:** <500ms
- **Cache Hit Response:** <1ms
- **Concurrent Requests:** Fully thread-safe
- **Nameserver Timeout:** 10 seconds

---

## Integration Tips

### Frontend Auto-Detection

Integrate detection in your proxy host form:

```typescript
useEffect(() => {
  if (hasWildcardDomain && domain) {
    const baseDomain = domain.replace(/^\*\./, '');

    detectDNSProvider(baseDomain)
      .then(result => {
        if (result.suggested_provider) {
          setDNSProviderID(result.suggested_provider.id);
          toast.success(
            `Auto-detected: ${result.suggested_provider.name}`
          );
        } else if (result.detected) {
          toast.info(
            `Detected ${result.provider_type} but not configured`
          );
        }
      })
      .catch(error => {
        console.error('Detection failed:', error);
        // Fail silently - manual selection still available
      });
  }
}, [domain, hasWildcardDomain]);
```

### Manual Override

Always allow users to manually override auto-detection:

```typescript
<select
  value={dnsProviderID}
  onChange={(e) => setDNSProviderID(e.target.value)}
>
  <option value="">Select DNS Provider</option>
  {providers.map(p => (
    <option key={p.id} value={p.id}>
      {p.name} {p.is_default && '(Default)'}
    </option>
  ))}
</select>
```

---

## Troubleshooting

### Provider Not Detected

If a provider isn't detected but should be:

1. **Check Nameservers Manually:**

   ```bash
   dig NS example.com +short
   # or
   nslookup -type=NS example.com
   ```

2. **Compare Against Patterns:**
   Use the `GET /api/v1/dns-providers/detection-patterns` endpoint to see if the nameserver matches any pattern.

3. **Check Confidence Level:**
   Low confidence might indicate mixed nameservers or custom configurations.

### DNS Lookup Failures

Common causes:

- Domain doesn't exist
- Nameserver temporarily unavailable
- Firewall blocking DNS queries
- Network connectivity issues

The API gracefully handles these and returns an error message in the response.

---

## Security Considerations

1. **Authentication Required:** All endpoints require valid JWT tokens
2. **Input Validation:** Domain names are sanitized and normalized
3. **No Credentials Exposed:** Detection only uses public nameserver information
4. **Rate Limiting:** Built-in through timeouts and caching
5. **DNS Spoofing:** Cached results limit exposure window

---

## Future Enhancements

Planned improvements (not yet implemented):

- Custom pattern management (admin feature)
- WHOIS data integration for fallback detection
- Detection statistics dashboard
- Machine learning for unknown provider classification
- Audit logging for detection attempts

---

## Support

For issues or questions:

- Check logs for detailed error messages
- Verify authentication tokens are valid
- Ensure domains are properly formatted
- Test DNS resolution independently

---

**API Version:** 1.0
**Last Updated:** January 4, 2026
**Status:** Production Ready
