# Feature Spec: Post-Import Notification & Certificate Status Dashboard

**Status:** Planning
**Created:** December 11, 2025
**Priority:** Medium

---

## Overview

Two related features to improve user experience around the import workflow and certificate provisioning visibility:

1. **Post-Import Success Notification** - Replace the current `alert()` with a proper modal showing import results and guidance about certificate provisioning
2. **Certificate Status Indicator on Dashboard** - Add visibility into certificate provisioning status with counts and visual indicators

---

## Feature 1: Post-Import Success Notification

### Current State

In [ImportCaddy.tsx](../../../frontend/src/pages/ImportCaddy.tsx#L42), after a successful import commit:

```tsx
const handleCommit = async (resolutions: Record<string, string>, names: Record<string, string>) => {
  try {
    await createBackup()
    await commit(resolutions, names)
    setContent('')
    setShowReview(false)
    alert('Import completed successfully!')  // ← Replace this
  } catch {
    // Error is already set by hook
  }
}
```

### Backend Response (import_handler.go)

The `/import/commit` endpoint returns a detailed response:

```json
{
  "created": 5,
  "updated": 2,
  "skipped": 1,
  "errors": []
}
```

This data is currently **not captured** by the frontend.

### Requirements

1. Replace `alert()` with a modal dialog component
2. Display import summary:
   - ✅ Number of hosts created
   - 🔄 Number of hosts updated (overwrites)
   - ⏭️ Number of hosts skipped
   - ❌ Any errors encountered
3. Include informational message about certificate provisioning:
   - "Certificate provisioning may take 1-5 minutes"
   - "Monitor the Dashboard for certificate status"
4. Provide navigation options:
   - "Go to Dashboard" button
   - "View Proxy Hosts" button
   - "Close" button

### Implementation Plan

#### 1. Create Import Success Modal Component

**File:** `frontend/src/components/dialogs/ImportSuccessModal.tsx`

```tsx
interface ImportSuccessModalProps {
  visible: boolean
  onClose: () => void
  onNavigateDashboard: () => void
  onNavigateHosts: () => void
  results: {
    created: number
    updated: number
    skipped: number
    errors: string[]
  }
}
```

**Design Pattern:** Follow existing modal patterns from:

- [ImportSitesModal.tsx](../../../frontend/src/components/ImportSitesModal.tsx) - Portal/overlay structure
- [CertificateCleanupDialog.tsx](../../../frontend/src/components/dialogs/CertificateCleanupDialog.tsx) - Form submission pattern

#### 2. Update API Types

**File:** `frontend/src/api/import.ts`

Add return type for commit:

```typescript
export interface ImportCommitResult {
  created: number
  updated: number
  skipped: number
  errors: string[]
}

export const commitImport = async (
  sessionUUID: string,
  resolutions: Record<string, string>,
  names: Record<string, string>
): Promise<ImportCommitResult> => {
  const { data } = await client.post<ImportCommitResult>('/import/commit', {
    session_uuid: sessionUUID,
    resolutions,
    names
  })
  return data
}
```

#### 3. Update useImport Hook

**File:** `frontend/src/hooks/useImport.ts`

```typescript
// Add to return type
commitResult: ImportCommitResult | null

// Capture result in mutation
const commitMutation = useMutation({
  mutationFn: async ({ resolutions, names }) => {
    const sessionId = statusQuery.data?.session?.id
    if (!sessionId) throw new Error("No active session")
    return commitImport(sessionId, resolutions, names)  // Now returns result
  },
  onSuccess: (result) => {
    setCommitResult(result)  // New state
    setCommitSucceeded(true)
    // ... existing invalidation logic
  },
})
```

#### 4. Update ImportCaddy.tsx

**File:** `frontend/src/pages/ImportCaddy.tsx`

```tsx
import { useNavigate } from 'react-router-dom'
import ImportSuccessModal from '../components/dialogs/ImportSuccessModal'

// In component:
const navigate = useNavigate()
const { commitResult, clearCommitResult } = useImport()  // New fields
const [showSuccessModal, setShowSuccessModal] = useState(false)

const handleCommit = async (resolutions, names) => {
  try {
    await createBackup()
    await commit(resolutions, names)
    setContent('')
    setShowReview(false)
    setShowSuccessModal(true)  // Show modal instead of alert
  } catch {
    // Error is already set by hook
  }
}

// In JSX:
<ImportSuccessModal
  visible={showSuccessModal}
  onClose={() => {
    setShowSuccessModal(false)
    clearCommitResult()
  }}
  onNavigateDashboard={() => navigate('/')}
  onNavigateHosts={() => navigate('/proxy-hosts')}
  results={commitResult}
/>
```

---

## Feature 2: Certificate Status Indicator on Dashboard

### Current State

In [Dashboard.tsx](../../../frontend/src/pages/Dashboard.tsx#L43-L47), the certificates card shows:

```tsx
<Link to="/certificates" className="bg-dark-card p-6 rounded-lg...">
  <div className="text-sm text-gray-400 mb-2">SSL Certificates</div>
  <div className="text-3xl font-bold text-white mb-1">{certificates.length}</div>
  <div className="text-xs text-gray-500">{certificates.filter(c => c.status === 'valid').length} valid</div>
</Link>
```

### Requirements

1. Show certificate breakdown by status:
   - ✅ Valid (production, trusted)
   - ⏳ Pending (hosts without certs yet)
   - ⚠️ Expiring (within 30 days)
   - 🔸 Staging/Untrusted
2. Visual progress indicator for pending certificates
3. Link to filtered certificate list or proxy hosts without certs
4. Auto-refresh to show provisioning progress

### Certificate Provisioning Detection

Certificates are provisioned by Caddy automatically. A host is "pending" if:

- `ProxyHost.certificate_id` is NULL
- `ProxyHost.ssl_forced` is true (expects a cert)
- No matching certificate exists in the certificates list

**Key insight:** The certificate service ([certificate_service.go](../../../backend/internal/services/certificate_service.go)) syncs certificates from Caddy's cert directory every 5 minutes. New hosts won't have certificates immediately.

### Implementation Plan

#### 1. Create Certificate Status Summary API Endpoint (Optional Enhancement)

**File:** `backend/internal/api/handlers/certificate_handler.go`

Add new endpoint for dashboard summary:

```go
// GET /certificates/summary
func (h *CertificateHandler) Summary(c *gin.Context) {
    certs, _ := h.service.ListCertificates()

    summary := gin.H{
        "total":     len(certs),
        "valid":     0,
        "expiring":  0,
        "expired":   0,
        "untrusted": 0,  // staging certs
    }

    for _, c := range certs {
        switch c.Status {
        case "valid":
            summary["valid"] = summary["valid"].(int) + 1
        case "expiring":
            summary["expiring"] = summary["expiring"].(int) + 1
        case "expired":
            summary["expired"] = summary["expired"].(int) + 1
        case "untrusted":
            summary["untrusted"] = summary["untrusted"].(int) + 1
        }
    }

    c.JSON(http.StatusOK, summary)
}
```

**Note:** This is optional. The frontend can compute this from existing certificate list.

#### 2. Add Pending Hosts Detection

The more important metric is "hosts awaiting certificates":

**Option A: Client-side calculation (simpler, no backend change)**

```tsx
// In Dashboard.tsx
const hostsWithSSL = hosts.filter(h => h.ssl_forced && h.enabled)
const hostsWithCerts = hosts.filter(h => h.certificate_id != null)
const pendingCerts = hostsWithSSL.length - hostsWithCerts.length
```

**Option B: Backend endpoint (more accurate)**

Add to proxy_host_handler.go:

```go
// GET /proxy-hosts/cert-status
func (h *ProxyHostHandler) CertStatus(c *gin.Context) {
    hosts, _ := h.service.List()

    withSSL := 0
    withCert := 0

    for _, h := range hosts {
        if h.SSLForced && h.Enabled {
            withSSL++
            if h.CertificateID != nil {
                withCert++
            }
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "total_ssl_enabled": withSSL,
        "with_certificate":  withCert,
        "pending":           withSSL - withCert,
    })
}
```

**Recommendation:** Start with Option A (client-side) for simplicity.

#### 3. Create CertificateStatusCard Component

**File:** `frontend/src/components/CertificateStatusCard.tsx`

```tsx
interface CertificateStatusCardProps {
  certificates: Certificate[]
  hosts: ProxyHost[]
}

export default function CertificateStatusCard({ certificates, hosts }: CertificateStatusCardProps) {
  const validCount = certificates.filter(c => c.status === 'valid').length
  const expiringCount = certificates.filter(c => c.status === 'expiring').length
  const untrustedCount = certificates.filter(c => c.status === 'untrusted').length

  // Pending = hosts with ssl_forced but no certificate_id
  const sslHosts = hosts.filter(h => h.ssl_forced && h.enabled)
  const hostsWithCerts = sslHosts.filter(h => h.certificate_id != null)
  const pendingCount = sslHosts.length - hostsWithCerts.length

  const hasProvisioning = pendingCount > 0

  return (
    <Link to="/certificates" className="bg-dark-card p-6 rounded-lg border border-gray-800...">
      <div className="text-sm text-gray-400 mb-2">SSL Certificates</div>
      <div className="text-3xl font-bold text-white mb-1">{certificates.length}</div>

      {/* Status breakdown */}
      <div className="flex flex-wrap gap-2 mt-2 text-xs">
        <span className="text-green-400">{validCount} valid</span>
        {expiringCount > 0 && <span className="text-yellow-400">{expiringCount} expiring</span>}
        {untrustedCount > 0 && <span className="text-orange-400">{untrustedCount} staging</span>}
      </div>

      {/* Pending indicator */}
      {hasProvisioning && (
        <div className="mt-3 pt-3 border-t border-gray-700">
          <div className="flex items-center gap-2 text-blue-400 text-xs">
            <svg className="animate-spin h-3 w-3" ...>...</svg>
            <span>{pendingCount} host{pendingCount > 1 ? 's' : ''} awaiting certificate</span>
          </div>
          <div className="mt-1 h-1 bg-gray-700 rounded-full overflow-hidden">
            <div
              className="h-full bg-blue-500 transition-all duration-500"
              style={{ width: `${(hostsWithCerts.length / sslHosts.length) * 100}%` }}
            />
          </div>
        </div>
      )}
    </Link>
  )
}
```

#### 4. Update Dashboard.tsx

**File:** `frontend/src/pages/Dashboard.tsx`

```tsx
import CertificateStatusCard from '../components/CertificateStatusCard'

// Add auto-refresh when there are pending certs
const hasPendingCerts = useMemo(() => {
  const sslHosts = hosts.filter(h => h.ssl_forced && h.enabled)
  return sslHosts.some(h => !h.certificate_id)
}, [hosts])

// Use React Query with conditional refetch
const { certificates } = useCertificates({
  refetchInterval: hasPendingCerts ? 15000 : false  // Poll every 15s when pending
})

// In JSX, replace static certificates card:
<CertificateStatusCard certificates={certificates} hosts={hosts} />
```

#### 5. Update useCertificates Hook

**File:** `frontend/src/hooks/useCertificates.ts`

```typescript
interface UseCertificatesOptions {
  refetchInterval?: number | false
}

export function useCertificates(options?: UseCertificatesOptions) {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['certificates'],
    queryFn: getCertificates,
    refetchInterval: options?.refetchInterval,
  })

  return {
    certificates: data || [],
    isLoading,
    error,
    refetch,
  }
}
```

---

## File Changes Summary

### New Files

| File | Description |
|------|-------------|
| `frontend/src/components/dialogs/ImportSuccessModal.tsx` | Modal for import completion |
| `frontend/src/components/CertificateStatusCard.tsx` | Dashboard card with cert status |

### Modified Files

| File | Changes |
|------|---------|
| `frontend/src/api/import.ts` | Add `ImportCommitResult` type, update `commitImport` return |
| `frontend/src/hooks/useImport.ts` | Capture and expose commit result |
| `frontend/src/hooks/useCertificates.ts` | Add optional refetch interval |
| `frontend/src/pages/ImportCaddy.tsx` | Replace alert with modal, add navigation |
| `frontend/src/pages/Dashboard.tsx` | Use new CertificateStatusCard component |

### Optional Backend Changes (for enhanced accuracy)

| File | Changes |
|------|---------|
| `backend/internal/api/handlers/certificate_handler.go` | Add `/certificates/summary` endpoint |
| `backend/internal/api/routes/routes.go` | Register summary route |

---

## Unit Test Requirements

### Feature 1: ImportSuccessModal

**File:** `frontend/src/components/dialogs/__tests__/ImportSuccessModal.test.tsx`

```typescript
describe('ImportSuccessModal', () => {
  it('renders import summary correctly', () => {
    render(<ImportSuccessModal results={{ created: 5, updated: 2, skipped: 1, errors: [] }} ... />)
    expect(screen.getByText('5 hosts created')).toBeInTheDocument()
    expect(screen.getByText('2 hosts updated')).toBeInTheDocument()
    expect(screen.getByText('1 host skipped')).toBeInTheDocument()
  })

  it('displays certificate provisioning guidance', () => {
    render(<ImportSuccessModal results={{ created: 5, updated: 0, skipped: 0, errors: [] }} ... />)
    expect(screen.getByText(/certificate provisioning/i)).toBeInTheDocument()
  })

  it('shows errors when present', () => {
    render(<ImportSuccessModal results={{ created: 0, updated: 0, skipped: 0, errors: ['example.com: duplicate'] }} ... />)
    expect(screen.getByText('example.com: duplicate')).toBeInTheDocument()
  })

  it('calls onNavigateDashboard when clicking Dashboard button', () => {
    const onNavigate = vi.fn()
    render(<ImportSuccessModal onNavigateDashboard={onNavigate} ... />)
    fireEvent.click(screen.getByText('Go to Dashboard'))
    expect(onNavigate).toHaveBeenCalled()
  })

  it('does not render when visible is false', () => {
    render(<ImportSuccessModal visible={false} ... />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
```

### Feature 2: CertificateStatusCard

**File:** `frontend/src/components/__tests__/CertificateStatusCard.test.tsx`

```typescript
describe('CertificateStatusCard', () => {
  it('shows valid certificate count', () => {
    render(<CertificateStatusCard certificates={mockCerts} hosts={mockHosts} />)
    expect(screen.getByText('3 valid')).toBeInTheDocument()
  })

  it('shows pending indicator when hosts lack certificates', () => {
    const hostsWithPending = [
      { ...mockHost, ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, ssl_forced: true, certificate_id: 1, enabled: true },
    ]
    render(<CertificateStatusCard certificates={mockCerts} hosts={hostsWithPending} />)
    expect(screen.getByText(/1 host awaiting certificate/)).toBeInTheDocument()
  })

  it('hides pending indicator when all hosts have certificates', () => {
    const hostsComplete = [
      { ...mockHost, ssl_forced: true, certificate_id: 1, enabled: true },
    ]
    render(<CertificateStatusCard certificates={mockCerts} hosts={hostsComplete} />)
    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('shows expiring count when certificates are expiring', () => {
    const expiringCerts = [{ ...mockCert, status: 'expiring' }]
    render(<CertificateStatusCard certificates={expiringCerts} hosts={[]} />)
    expect(screen.getByText('1 expiring')).toBeInTheDocument()
  })

  it('shows staging count for untrusted certificates', () => {
    const stagingCerts = [{ ...mockCert, status: 'untrusted' }]
    render(<CertificateStatusCard certificates={stagingCerts} hosts={[]} />)
    expect(screen.getByText('1 staging')).toBeInTheDocument()
  })

  it('calculates progress bar correctly', () => {
    const hosts = [
      { ...mockHost, ssl_forced: true, certificate_id: 1, enabled: true },
      { ...mockHost, ssl_forced: true, certificate_id: null, enabled: true },
    ]
    const { container } = render(<CertificateStatusCard certificates={mockCerts} hosts={hosts} />)
    const progressBar = container.querySelector('[style*="width: 50%"]')
    expect(progressBar).toBeInTheDocument()
  })
})
```

### Integration Tests for useImport Hook

**File:** `frontend/src/hooks/__tests__/useImport.test.tsx`

```typescript
describe('useImport - commit result', () => {
  it('captures commit result on success', async () => {
    mockCommitImport.mockResolvedValue({ created: 3, updated: 1, skipped: 0, errors: [] })

    const { result } = renderHook(() => useImport(), { wrapper })
    await result.current.commit({}, {})

    expect(result.current.commitResult).toEqual({ created: 3, updated: 1, skipped: 0, errors: [] })
    expect(result.current.commitSuccess).toBe(true)
  })
})
```

---

## UI/UX Design Notes

### ImportSuccessModal

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│           ✅  Import Completed                       │
│                                                      │
│   ┌────────────────────────────────────────────┐    │
│   │  📦 5 hosts created                        │    │
│   │  🔄 2 hosts updated                        │    │
│   │  ⏭️  1 host skipped                        │    │
│   └────────────────────────────────────────────┘    │
│                                                      │
│   ℹ️  Certificate Provisioning                       │
│   SSL certificates will be automatically             │
│   provisioned by Let's Encrypt. This typically      │
│   takes 1-5 minutes per domain.                     │
│                                                      │
│   Monitor the Dashboard to track certificate        │
│   provisioning progress.                            │
│                                                      │
│   ┌──────────┐  ┌───────────────┐  ┌────────┐      │
│   │ Dashboard│  │ View Hosts    │  │ Close  │      │
│   └──────────┘  └───────────────┘  └────────┘      │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### Certificate Status Card (Dashboard)

```
┌────────────────────────────────────────┐
│ SSL Certificates                       │
│                                        │
│        12                              │
│                                        │
│ ✅ 10 valid  ⚠️ 1 expiring             │
│ 🔸 1 staging                           │
│                                        │
│ ────────────────────────────────────── │
│ ⏳ 3 hosts awaiting certificate        │
│ ████████████░░░░░░░  75%               │
└────────────────────────────────────────┘
```

---

## Implementation Order

1. **Phase 1: Import Success Modal** (Higher priority)
   - Update `api/import.ts` types
   - Update `useImport` hook
   - Create `ImportSuccessModal` component
   - Update `ImportCaddy.tsx`
   - Write unit tests

2. **Phase 2: Certificate Status Card** (Depends on Phase 1 for testing flow)
   - Update `useCertificates` hook with refetch option
   - Create `CertificateStatusCard` component
   - Update `Dashboard.tsx`
   - Write unit tests

3. **Phase 3: Polish**
   - Add loading states
   - Responsive design adjustments
   - Accessibility review (ARIA labels, focus management)

---

## Acceptance Criteria

### Feature 1: Post-Import Success Notification

- [ ] No `alert()` calls remain in import flow
- [ ] Modal displays created/updated/skipped counts
- [ ] Modal shows certificate provisioning guidance
- [ ] Navigation buttons work correctly
- [ ] Modal closes properly and clears state
- [ ] Unit tests pass with >80% coverage

### Feature 2: Certificate Status Indicator

- [ ] Dashboard shows certificate breakdown by status
- [ ] Pending count reflects hosts without certificates
- [ ] Progress bar animates as certs are provisioned
- [ ] Auto-refresh when there are pending certificates
- [ ] Links navigate to appropriate views
- [ ] Unit tests pass with >80% coverage

---

## References

- Existing modal pattern: [ImportSitesModal.tsx](../../../frontend/src/components/ImportSitesModal.tsx)
- Dialog pattern: [CertificateCleanupDialog.tsx](../../../frontend/src/components/dialogs/CertificateCleanupDialog.tsx)
- Toast utility: [toast.ts](../../../frontend/src/utils/toast.ts)
- Certificate types: [certificates.ts](../../../frontend/src/api/certificates.ts)
- Import hook: [useImport.ts](../../../frontend/src/hooks/useImport.ts)
- Dashboard: [Dashboard.tsx](../../../frontend/src/pages/Dashboard.tsx)
