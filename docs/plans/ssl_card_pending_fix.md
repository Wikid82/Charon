# Bug Fix: SSL Card Shows "Waiting for 14 Certs" When All Certificates Are Loaded

**Status:** Ready for Implementation
**Created:** December 12, 2025
**Priority:** High (User-facing display bug)

---

## Issue Summary

The SSL card on the Dashboard incorrectly shows "waiting for 14 certs" (or similar) when all SSL certificates have actually been provisioned and are available. This is a **logic bug in the "pending" certificate detection**.

---

## Root Cause Analysis

### The Current Logic

In [frontend/src/components/CertificateStatusCard.tsx](../../frontend/src/components/CertificateStatusCard.tsx#L16-L19):

```tsx
// Pending = hosts with ssl_forced and enabled but no certificate_id
const sslHosts = hosts.filter(h => h.ssl_forced && h.enabled)
const hostsWithCerts = sslHosts.filter(h => h.certificate_id != null)
const pendingCount = sslHosts.length - hostsWithCerts.length
```

### Why This Logic Is Wrong

**The fundamental misunderstanding**: The code assumes `certificate_id` on a `ProxyHost` indicates whether that host has a valid SSL certificate. **This is incorrect for ACME (Let's Encrypt/ZeroSSL) managed certificates.**

**How Caddy/Charon actually works:**

1. **ACME certificates (Let's Encrypt, ZeroSSL)**: Caddy **automatically** provisions and manages these certificates. The `certificate_id` field on `ProxyHost` is **NOT** used for ACME certs - it's only for custom uploaded certificates.

2. **Custom certificates**: Only when a user uploads a custom certificate is it stored in `ssl_certificates` table and linked via `certificate_id` on the proxy host.

3. **Backend evidence** - From [backend/internal/api/routes/routes.go](../../backend/internal/api/routes/routes.go#L70-L73):

   ```go
   // Let's Encrypt certs are auto-managed by Caddy and should not be assigned via certificate_id
   ```

4. **Certificate service** - From [backend/internal/services/certificate_service.go](../../backend/internal/services/certificate_service.go#L97-L105):
   The certificate service scans `data/certificates/` directory for `.crt` files and syncs them to the database. These certificates are discovered **by domain name**, not by any host relationship.

### The Actual Bug

When a user has:

- 14 proxy hosts with `ssl_forced: true`
- All 14 hosts have ACME-managed certificates (Let's Encrypt/ZeroSSL)
- **None** of these hosts have `certificate_id` set (because ACME certs don't use this field)

The current logic sees 14 SSL hosts with no `certificate_id` → reports "14 hosts awaiting certificate" ❌

### Data Flow Diagram

```
ProxyHost                              SSLCertificate (DB)
┌─────────────────────┐                ┌──────────────────────┐
│ uuid: "abc123"      │                │ domain: "example.com"│
│ domain_names:       │   NO LINK      │ provider: "letsencrypt"
│   "example.com"     │ ══════════════ │ status: "valid"      │
│ ssl_forced: true    │  (for ACME)    │ expires_at: ...      │
│ certificate_id: null│                └──────────────────────┘
└─────────────────────┘
          ↓
   Current logic sees certificate_id: null
          ↓
   Incorrectly reports as "pending"
```

---

## Correct Solution

### New Logic: Match by Domain Name

Instead of checking `certificate_id`, we should check if a certificate **exists for the host's domain(s)**:

```tsx
// A host is "pending" if:
// 1. ssl_forced is true AND enabled is true
// 2. AND there's no certificate with a matching domain

// Build a set of all domains that have certificates
const certifiedDomains = new Set<string>()
certificates.forEach(cert => {
  // Certificate domains can be comma-separated
  cert.domain.split(',').forEach(d => {
    certifiedDomains.add(d.trim().toLowerCase())
  })
})

// Check each SSL host
const sslHosts = hosts.filter(h => h.ssl_forced && h.enabled)
const pendingHosts = sslHosts.filter(host => {
  // Check if ANY of the host's domains have a certificate
  const hostDomains = host.domain_names.split(',').map(d => d.trim().toLowerCase())
  return !hostDomains.some(domain => certifiedDomains.has(domain))
})

const pendingCount = pendingHosts.length
```

---

## Files to Modify

### Frontend Changes

| File | Change |
|------|--------|
| [frontend/src/components/CertificateStatusCard.tsx](../../frontend/src/components/CertificateStatusCard.tsx) | Update pending detection logic to match by domain |
| [frontend/src/components/**tests**/CertificateStatusCard.test.tsx](../../frontend/src/components/__tests__/CertificateStatusCard.test.tsx) | Update tests for new domain-matching logic |

### No Backend Changes Required

The backend already provides:

- `ProxyHost.domain_names` - comma-separated list of domains
- `Certificate.domain` - the domain(s) covered by the certificate

---

## Implementation Details

### Updated CertificateStatusCard.tsx

```tsx
import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { Loader2 } from 'lucide-react'
import type { Certificate } from '../api/certificates'
import type { ProxyHost } from '../api/proxyHosts'

interface CertificateStatusCardProps {
  certificates: Certificate[]
  hosts: ProxyHost[]
}

export default function CertificateStatusCard({ certificates, hosts }: CertificateStatusCardProps) {
  const validCount = certificates.filter(c => c.status === 'valid').length
  const expiringCount = certificates.filter(c => c.status === 'expiring').length
  const untrustedCount = certificates.filter(c => c.status === 'untrusted').length

  // Build a set of all domains that have certificates (case-insensitive)
  const certifiedDomains = useMemo(() => {
    const domains = new Set<string>()
    certificates.forEach(cert => {
      // Certificate domain field can be comma-separated
      cert.domain.split(',').forEach(d => {
        const trimmed = d.trim().toLowerCase()
        if (trimmed) domains.add(trimmed)
      })
    })
    return domains
  }, [certificates])

  // Calculate pending hosts: SSL-enabled hosts without any domain covered by a certificate
  const { pendingCount, totalSSLHosts, hostsWithCerts } = useMemo(() => {
    const sslHosts = hosts.filter(h => h.ssl_forced && h.enabled)

    let withCerts = 0
    sslHosts.forEach(host => {
      // Check if any of the host's domains have a certificate
      const hostDomains = host.domain_names.split(',').map(d => d.trim().toLowerCase())
      if (hostDomains.some(domain => certifiedDomains.has(domain))) {
        withCerts++
      }
    })

    return {
      pendingCount: sslHosts.length - withCerts,
      totalSSLHosts: sslHosts.length,
      hostsWithCerts: withCerts,
    }
  }, [hosts, certifiedDomains])

  const hasProvisioning = pendingCount > 0
  const progressPercent = totalSSLHosts > 0
    ? Math.round((hostsWithCerts / totalSSLHosts) * 100)
    : 100

  return (
    <Link
      to="/certificates"
      className="bg-dark-card p-6 rounded-lg border border-gray-800 hover:border-gray-700 transition-colors block"
    >
      <div className="text-sm text-gray-400 mb-2">SSL Certificates</div>
      <div className="text-3xl font-bold text-white mb-1">{certificates.length}</div>

      {/* Status breakdown */}
      <div className="flex flex-wrap gap-x-3 gap-y-1 mt-2 text-xs">
        <span className="text-green-400">{validCount} valid</span>
        {expiringCount > 0 && <span className="text-yellow-400">{expiringCount} expiring</span>}
        {untrustedCount > 0 && <span className="text-orange-400">{untrustedCount} staging</span>}
      </div>

      {/* Pending indicator */}
      {hasProvisioning && (
        <div className="mt-3 pt-3 border-t border-gray-700">
          <div className="flex items-center gap-2 text-blue-400 text-xs">
            <Loader2 className="h-3 w-3 animate-spin" />
            <span>{pendingCount} host{pendingCount !== 1 ? 's' : ''} awaiting certificate</span>
          </div>
          <div className="mt-2 h-1.5 bg-gray-700 rounded-full overflow-hidden">
            <div
              className="h-full bg-blue-500 transition-all duration-500 rounded-full"
              style={{ width: `${progressPercent}%` }}
            />
          </div>
          <div className="text-xs text-gray-500 mt-1">{progressPercent}% provisioned</div>
        </div>
      )}
    </Link>
  )
}
```

---

## Updated Test Cases

### Key Test Changes

The existing tests rely on `certificate_id` for determining "pending" status. We need to update these to use domain-based matching.

**File:** `frontend/src/components/__tests__/CertificateStatusCard.test.tsx`

#### Tests to Keep (Unchanged)

- `shows total certificate count`
- `shows valid certificate count`
- `shows expiring count when certificates are expiring`
- `hides expiring count when no certificates are expiring`
- `shows staging count for untrusted certificates`
- `hides staging count when no untrusted certificates`
- `links to certificates page`
- `handles empty certificates array`
- `shows spinning loader icon when pending`

#### Tests to Update

**Remove** tests that check `certificate_id` directly:

- `shows pending indicator when hosts lack certificates` - needs domain matching
- `shows plural for multiple pending hosts` - needs domain matching
- `hides pending indicator when all hosts have certificates` - needs domain matching
- `ignores disabled hosts when calculating pending` - update to use domain
- `ignores hosts without SSL when calculating pending` - update to use domain
- `calculates progress percentage correctly` - update to use domain

**Add** new domain-based tests:

```tsx
const mockCertWithDomain = (domain: string, status: 'valid' | 'expiring' | 'expired' | 'untrusted' = 'valid'): Certificate => ({
  id: Math.random(),
  name: domain,
  domain: domain,
  issuer: "Let's Encrypt",
  expires_at: new Date(Date.now() + 90 * 24 * 60 * 60 * 1000).toISOString(),
  status,
  provider: 'letsencrypt',
})

describe('CertificateStatusCard - Domain Matching', () => {
  it('does not show pending when host domain matches certificate domain', () => {
    const certs: Certificate[] = [mockCertWithDomain('example.com')]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    // Should NOT show "awaiting certificate" since domain matches
    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('shows pending when host domain has no matching certificate', () => {
    const certs: Certificate[] = [mockCertWithDomain('other.com')]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.getByText('1 host awaiting certificate')).toBeInTheDocument()
  })

  it('handles case-insensitive domain matching', () => {
    const certs: Certificate[] = [mockCertWithDomain('EXAMPLE.COM')]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('handles multi-domain hosts with partial certificate coverage', () => {
    // Host has two domains, but only one has a certificate - should be "covered"
    const certs: Certificate[] = [mockCertWithDomain('example.com')]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com, www.example.com', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    // Host should be considered "covered" if any domain has a cert
    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('handles comma-separated certificate domains', () => {
    const certs: Certificate[] = [{
      ...mockCertWithDomain('example.com'),
      domain: 'example.com, www.example.com'
    }]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'www.example.com', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('ignores disabled hosts even without certificate', () => {
    const certs: Certificate[] = []
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com', ssl_forced: true, certificate_id: null, enabled: false }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('ignores hosts without SSL forced', () => {
    const certs: Certificate[] = []
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com', ssl_forced: false, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('calculates progress percentage with domain matching', () => {
    const certs: Certificate[] = [
      mockCertWithDomain('a.example.com'),
      mockCertWithDomain('b.example.com'),
    ]
    const hosts: ProxyHost[] = [
      { ...mockHost, uuid: 'h1', domain_names: 'a.example.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h2', domain_names: 'b.example.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h3', domain_names: 'c.example.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h4', domain_names: 'd.example.com', ssl_forced: true, certificate_id: null, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    // 2 out of 4 hosts have matching certs = 50%
    expect(screen.getByText('50% provisioned')).toBeInTheDocument()
    expect(screen.getByText('2 hosts awaiting certificate')).toBeInTheDocument()
  })

  it('shows all pending when no certificates exist', () => {
    const certs: Certificate[] = []
    const hosts: ProxyHost[] = [
      { ...mockHost, uuid: 'h1', domain_names: 'a.example.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h2', domain_names: 'b.example.com', ssl_forced: true, certificate_id: null, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.getByText('2 hosts awaiting certificate')).toBeInTheDocument()
    expect(screen.getByText('0% provisioned')).toBeInTheDocument()
  })
})
```

---

## Edge Cases to Handle

| Scenario | Expected Behavior |
|----------|-------------------|
| Multiple domains per host | Covered if **any** domain has a certificate |
| Case sensitivity | Case-insensitive matching (`Example.COM` = `example.com`) |
| Whitespace in domain list | Trim whitespace from both sides |
| Custom certificates | Hosts with `certificate_id` set should still be considered "covered" |
| Disabled hosts | Ignored (not counted as pending) |
| Non-SSL hosts (`ssl_forced: false`) | Ignored (not counted as pending) |
| Empty domain list | Ignored |
| Wildcard certs | Currently not specially handled (future enhancement) |

---

## Testing Plan

### Unit Tests (Automated)

1. ✅ Host with matching certificate domain → not pending
2. ✅ Host with no matching certificate → pending
3. ✅ Host with multiple domains, one matched → not pending
4. ✅ Case-insensitive domain matching
5. ✅ Disabled hosts ignored
6. ✅ Non-SSL hosts ignored
7. ✅ Progress percentage calculation with domain matching
8. ✅ Empty certificates list → all SSL hosts pending
9. ✅ Empty hosts list → no pending indicator

### Manual Testing

1. Import a Caddyfile with multiple hosts
2. Wait for ACME certificates to be provisioned (1-5 minutes)
3. Verify dashboard SSL card:
   - Shows correct number of certificates
   - Shows "X valid" count
   - Does NOT show "waiting for X certs" when all domains have certs
4. Verify the pending indicator appears only when:
   - A new host is added and certificate hasn't been provisioned yet
   - Truly no certificate exists for that domain

---

## Dashboard Polling Behavior

The Dashboard already has conditional polling logic:

```tsx
// Dashboard.tsx
const hasPendingCerts = useMemo(() => {
  const sslHosts = hosts.filter(h => h.ssl_forced && h.enabled)
  return sslHosts.some(h => !h.certificate_id)  // <-- This also needs updating
}, [hosts])

const { certificates } = useCertificates({
  refetchInterval: hasPendingCerts ? 15000 : false,
})
```

**Also update Dashboard.tsx** to use domain-based checking for the polling trigger:

```tsx
const hasPendingCerts = useMemo(() => {
  // Build set of certified domains
  const certifiedDomains = new Set<string>()
  certificates.forEach(cert => {
    cert.domain.split(',').forEach(d => {
      certifiedDomains.add(d.trim().toLowerCase())
    })
  })

  // Check if any SSL host lacks a certificate
  const sslHosts = hosts.filter(h => h.ssl_forced && h.enabled)
  return sslHosts.some(host => {
    const hostDomains = host.domain_names.split(',').map(d => d.trim().toLowerCase())
    return !hostDomains.some(domain => certifiedDomains.has(domain))
  })
}, [hosts, certificates])
```

---

## Rollback Plan

If issues arise, the fallback approach keeps backward compatibility:

```tsx
// Fallback: host is "pending" only if:
// 1. No certificate_id AND
// 2. No certificate with matching domain
const pendingHosts = sslHosts.filter(host => {
  if (host.certificate_id != null) return false  // Has custom cert
  const hostDomains = host.domain_names.split(',').map(d => d.trim().toLowerCase())
  return !hostDomains.some(domain => certifiedDomains.has(domain))
})
```

---

## Acceptance Criteria

- [ ] SSL card does not show "waiting for X certs" when all certificates are provisioned
- [ ] SSL card correctly shows pending count only for hosts without any matching certificate
- [ ] Domain matching is case-insensitive
- [ ] Multi-domain hosts are handled correctly
- [ ] Existing custom certificate workflow still works (hosts with `certificate_id`)
- [ ] Dashboard polling stops correctly once all domains have certificates
- [ ] All unit tests pass
- [ ] No regression in certificate status display

---

## Summary

| Aspect | Current (Bug) | Fixed |
|--------|---------------|-------|
| Check method | `certificate_id != null` | Domain name matching |
| ACME certs | Always "pending" | Correctly detected as covered |
| Custom certs | Works | Still works |
| Multi-domain | N/A | Covered if any domain matches |
| Case sensitivity | N/A | Case-insensitive |
| Polling | Never stops | Stops when all covered |

**Root Cause:** Using `certificate_id` (custom cert reference) instead of domain matching for ACME certificates.

**Fix:** Match host domains against certificate domains to determine coverage.

**Effort:** ~1 hour implementation + testing
