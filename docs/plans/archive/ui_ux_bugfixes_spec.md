# 📋 Plan: UI/UX and Backend Bug Fixes - Multi-Issue Resolution

**Created**: December 5, 2025
**Status**: Planning Complete - Ready for Implementation

---

## 🧐 UX & Context Analysis

The user has identified **12 distinct issues** affecting usability, consistency, and functionality. These span both frontend (UI/UX) and backend (API/data) concerns.

### Issue Summary Matrix

| # | Issue | Category | Severity | Component |
|---|-------|----------|----------|-----------|
| 1 | Uptime card not updated when editing proxy host | Backend/Frontend | High | ProxyHostForm, UptimeService |
| 2 | Certificates missing delete action | Frontend | Medium | CertificateList |
| 3 | Inconsistent app sizing between IP and domain access | Frontend/CSS | Medium | index.css, Layout |
| 4 | Notification Provider template dropdown invisible text | Frontend | High | Notifications |
| 5 | Templates should be in Provider section with assignment | Frontend | Medium | Notifications |
| 6 | Banner/header sizing issues (tiny on desktop, huge on mobile) | Frontend | Medium | Layout |
| 7 | Mobile drawer icon should be on left side | Frontend | Low | Layout |
| 8 | Mobile view should show logo instead of banner | Frontend | Low | Layout |
| 9 | CrowdSec card buttons truncated on smaller screens | Frontend | Medium | Security |
| 10 | /security/crowdsec shows blank page | Frontend | High | CrowdSecConfig, Layout |
| 11 | Reorganize sidebar: Users → Account Management under Settings | Frontend | Medium | Layout, Router |
| 12 | Missing loading overlay when adding/removing ACL from proxy host | Frontend | High | ProxyHosts, useProxyHosts |

---

## 🤝 Handoff Contracts (API Specifications)

### Issue #1: Uptime Card Sync on Proxy Host Edit

**Problem**: When editing a proxy host (e.g., changing name or domain), the associated UptimeMonitor is not updated. Users must manually delete and recreate uptime cards.

**Current Behavior**: `syncMonitors()` only creates new monitors or updates name; it doesn't handle domain/URL changes properly.

**Required Backend Changes**:

```json
// PUT /api/v1/proxy-hosts/:uuid
// Backend should automatically trigger uptime monitor sync when relevant fields change
{
  "request_payload": {
    "name": "Updated Name",
    "domain_names": "new.example.com",
    "forward_host": "192.168.1.100",
    "forward_port": 8080
  },
  "response_success": {
    "uuid": "abc123",
    "name": "Updated Name",
    "domain_names": "new.example.com",
    "uptime_monitor_synced": true
  }
}
```

**Implementation**: Modify `updateProxyHost` handler to call `UptimeService.SyncMonitorForHost(hostID)` after successful update. The sync should update URL, name, and upstream_host on the linked monitor.

---

### Issue #2: Certificate Delete Action

**Problem**: Certificates page shows no actions in the table. Delete button only appears conditionally and conditions are too restrictive.

**Frontend Fix**: Always show delete action for custom and staging certificates. Improve visibility logic.

```tsx
// CertificateList.tsx - Actions column should always render delete for deletable certs
// Current condition is too restrictive:
// cert.id && (cert.provider === 'custom' || cert.issuer?.toLowerCase().includes('staging'))
// AND status check: cert.status !== 'valid' && cert.status !== 'expiring'

// New condition: Remove status restriction, allow delete for custom/staging certs
// Only block if certificate is in use by a proxy host
```

---

### Issue #3: Inconsistent App Sizing

**Root Cause**: `body { zoom: 0.75; }` in `index.css` causes different rendering based on browser zoom behavior when accessed via IP vs domain.

**Solution**: Remove the `zoom: 0.75` property and instead use proper CSS scaling or adjust layout max-widths for consistent sizing.

```css
/* BEFORE */
body {
  zoom: 0.75; /* REMOVE THIS */
}

/* AFTER */
body {
  margin: 0;
  min-width: 320px;
  min-height: 100vh;
  /* Use consistent font sizing and container widths instead */
}
```

---

### Issue #4: Notification Template Dropdown Invisible Text

**Problem**: In `ProviderForm`, the template `<select>` dropdown options have text that matches the background color.

**Fix**: Add proper text color classes to the select element.

```tsx
// BEFORE (line ~119 in Notifications.tsx)
<select {...register('template')} className="mt-1 block w-full rounded-md border-gray-300">

// AFTER
<select {...register('template')} className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
```

---

### Issue #5: Template Assignment UX Redesign

**Current Flow**:

1. User creates provider
2. User separately manages templates
3. No direct way to assign template to provider

**New Flow**:

1. Provider form includes template selector dropdown
2. Dropdown shows: "Minimal (built-in)", "Detailed (built-in)", "Custom", and any saved external templates
3. If "Custom" selected, inline textarea appears
4. Templates can be saved with a name for reuse

```tsx
// ProviderForm structure update - consolidate template selection
<div>
  <label>Message Template</label>
  <select value={templateSelection}>
    <optgroup label="Built-in Templates">
      <option value="minimal">Minimal</option>
      <option value="detailed">Detailed</option>
    </optgroup>
    <optgroup label="Saved Templates">
      {externalTemplates.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
    </optgroup>
    <option value="custom">Custom...</option>
  </select>

  {templateSelection === 'custom' && (
    <>
      <textarea value={customTemplate} />
      <button onClick={saveAsTemplate}>Save as Template</button>
    </>
  )}
</div>
```

---

### Issue #6, #7, #8: Mobile/Desktop Header & Banner Fixes

**Problems**:

- Banner tiny on desktop header
- Banner huge on mobile header
- Drawer toggle icon on right (should be left)
- Mobile should show logo instead of banner

**Layout.tsx Changes**:

```tsx
// Mobile Header (line ~97) - Move menu button to left, show logo instead of banner
// BEFORE:
<div className="lg:hidden fixed top-0 left-0 right-0 h-16 ...">
  <img src="/banner.png" alt="Charon" height={1280} width={640} />
  <div className="flex items-center gap-2">
    ...
    <Button variant="ghost" size="sm" onClick={() => setMobileSidebarOpen(!mobileSidebarOpen)}>
      {mobileSidebarOpen ? '✕' : '☰'}
    </Button>
  </div>
</div>

// AFTER:
<div className="lg:hidden fixed top-0 left-0 right-0 h-16 ...">
  <div className="flex items-center gap-2">
    <Button variant="ghost" size="sm" onClick={() => setMobileSidebarOpen(!mobileSidebarOpen)}>
      <Menu className="w-5 h-5" />
    </Button>
    <img src="/logo.png" alt="Charon" className="h-10 w-auto" />
  </div>
  <div className="flex items-center gap-2">
    <NotificationCenter />
    <ThemeToggle />
  </div>
</div>

// Sidebar banner (line ~109) - consistent sizing
<div className="h-20 flex items-center justify-center ...">
  {isCollapsed ? (
    <img src="/logo.png" alt="Charon" className="h-12 w-auto" />
  ) : (
    <img src="/banner.png" alt="Charon" className="h-12 w-auto max-w-[200px]" />
  )}
</div>
```

---

### Issue #9: CrowdSec Card Button Truncation

**Problem**: On smaller screens, CrowdSec card buttons overflow and get cut off.

**Fix**: Make button container responsive with flex-wrap.

```tsx
// Security.tsx - CrowdSec card buttons (around line ~173)
// BEFORE:
<div className="mt-4 flex gap-2">
  <Button>View Logs</Button>
  <Button>Export</Button>
  <Button>Configure</Button>
  <div className="flex gap-2 w-full">
    <Button>Start</Button>
    <Button>Stop</Button>
  </div>
</div>

// AFTER:
<div className="mt-4 grid grid-cols-2 gap-2">
  <Button variant="secondary" size="sm" className="text-xs">Logs</Button>
  <Button variant="secondary" size="sm" className="text-xs">Export</Button>
  <Button variant="secondary" size="sm" className="text-xs">Configure</Button>
  <Button variant="primary" size="sm" className="text-xs">Start</Button>
  <Button variant="secondary" size="sm" className="text-xs" disabled={!crowdsecStatus?.running}>Stop</Button>
</div>
```

---

### Issue #10: /security/crowdsec Blank Page

**Problem**: The CrowdSecConfig page renders blank with no header/sidebar.

**Root Cause**: The route `/security/crowdsec` exists and is inside the Layout wrapper in App.tsx. However, in `CrowdSecConfig.tsx`, the early return `if (!status) return <div>Loading...</div>` renders a plain div without proper styling.

**Fix**: Ensure loading states are styled consistently.

```tsx
// CrowdSecConfig.tsx (line ~75)
// BEFORE:
if (!status) return <div className="p-8 text-center">Loading...</div>

// This is fine - the issue might be elsewhere. Check if the query is failing silently.
// Add error handling:
const { data: status, isLoading, error } = useQuery({ queryKey: ['security-status'], queryFn: getSecurityStatus })

if (isLoading) return <div className="p-8 text-center text-white">Loading CrowdSec configuration...</div>
if (error) return <div className="p-8 text-center text-red-500">Failed to load security status</div>
if (!status) return <div className="p-8 text-center text-gray-400">No security status available</div>
```

---

### Issue #11: Sidebar Reorganization

**Current Structure**:

```
- Users (/users)
- Settings
  - System
  - Email (SMTP)
  - Account
```

**New Structure**:

```
- Settings
  - System
  - Email (SMTP)
  - Accounts (/settings/accounts)
     - Account Management (/settings/accounts/management)
```

**Changes Required**:

1. **Layout.tsx** - Update navigation array:

```tsx
// Remove standalone Users item
// Update Settings children:
{
  name: 'Settings',
  path: '/settings',
  icon: '⚙️',
  children: [
    { name: 'System', path: '/settings/system', icon: '⚙️' },
    { name: 'Email (SMTP)', path: '/settings/smtp', icon: '📧' },
    { name: 'Accounts', path: '/settings/accounts', icon: '🛡️' },
    { name: 'Account Management', path: '/settings/account/management', icon: '👥' },
  ]
}
```

1. **App.tsx** - Update routes:

```tsx
// Remove: <Route path="users" element={<UsersPage />} />
// Add under settings:
<Route path="settings/account-management" element={<UsersPage />} />
```

---

### Issue #12: Missing ACL Loading Overlay

**Problem**: When adding/removing ACL from proxy host (single or bulk), no loading overlay appears during Caddy reload.

**Current Code Analysis**: `ProxyHosts.tsx` uses `ConfigReloadOverlay` but the overlay condition checks:

```tsx
const isApplyingConfig = isCreating || isUpdating || isDeleting || isBulkUpdating
```

**Analysis**: The `isBulkUpdating` comes from `useProxyHosts` hook's `bulkUpdateACLMutation.isPending`. This should work.

**Potential Issue**: The single host ACL update uses `updateHost(uuid, { access_list_id: id })` which triggers `isUpdating`. This should also work.

**Fix**: Verify the overlay is not being hidden by other elements (z-index), and ensure the mutation states are properly connected.

```tsx
// ProxyHosts.tsx - Verify overlay is at highest z-index
{isApplyingConfig && (
  <ConfigReloadOverlay
    message={message}
    submessage={submessage}
    type="charon"
  />
)}

// Also check in ProxyHostForm.tsx for ACL changes within the form
// The AccessListSelector onChange should trigger the same loading state
```

---

## 🏗️ Phase 1: Backend Implementation (Go)

### Files to Modify

1. **`backend/internal/services/uptime_service.go`**
   - Add `SyncMonitorForHost(hostID uint)` method
   - Update existing linked monitor's URL, name, and upstream_host

2. **`backend/internal/api/handlers/proxy_host_handler.go`**
   - In `updateProxyHost`, call uptime sync after successful update

### New Method in uptime_service.go

```go
// SyncMonitorForHost updates the uptime monitor linked to a specific proxy host
func (s *UptimeService) SyncMonitorForHost(hostID uint) error {
    var host models.ProxyHost
    if err := s.DB.First(&host, hostID).Error; err != nil {
        return err
    }

    var monitor models.UptimeMonitor
    if err := s.DB.Where("proxy_host_id = ?", hostID).First(&monitor).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            // No monitor exists, nothing to sync
            return nil
        }
        return err
    }

    // Update monitor fields
    domains := strings.Split(host.DomainNames, ",")
    firstDomain := strings.TrimSpace(domains[0])

    scheme := "http"
    if host.SSLForced {
        scheme = "https"
    }

    monitor.Name = host.Name
    if monitor.Name == "" {
        monitor.Name = firstDomain
    }
    monitor.URL = fmt.Sprintf("%s://%s", scheme, firstDomain)
    monitor.UpstreamHost = host.ForwardHost

    return s.DB.Save(&monitor).Error
}
```

### Handler modification in proxy_host_handler.go

```go
// In UpdateProxyHost handler, after successful save:
func (h *ProxyHostHandler) UpdateProxyHost(c *gin.Context) {
    // ... existing code ...

    if err := h.DB.Save(&host).Error; err != nil {
        // ... error handling ...
    }

    // Sync associated uptime monitor
    if h.UptimeService != nil {
        if err := h.UptimeService.SyncMonitorForHost(host.ID); err != nil {
            logger.Log().WithError(err).Warn("Failed to sync uptime monitor for host")
            // Don't fail the request, just log the warning
        }
    }

    // ... rest of handler ...
}
```

---

## 🎨 Phase 2: Frontend Implementation (React)

### Files to Modify

| File | Changes |
|------|---------|
| `frontend/src/index.css` | Remove `zoom: 0.75` |
| `frontend/src/components/Layout.tsx` | Mobile header, sidebar nav, banner sizing |
| `frontend/src/components/CertificateList.tsx` | Fix delete action visibility |
| `frontend/src/pages/Notifications.tsx` | Fix template dropdown styling, UX flow |
| `frontend/src/pages/Security.tsx` | Responsive CrowdSec card buttons |
| `frontend/src/pages/CrowdSecConfig.tsx` | Fix loading/error states |
| `frontend/src/App.tsx` | Route reorganization |

### Implementation Priority

1. **Critical (Broken functionality)**:
   - Issue #10: CrowdSec blank page
   - Issue #4: Notification dropdown text invisible
   - Issue #12: ACL loading overlay

2. **High (Poor UX)**:
   - Issue #1: Uptime card not syncing
   - Issue #2: Certificate delete missing
   - Issue #3: Inconsistent sizing

3. **Medium (Polish)**:
   - Issues #6-8: Mobile header/banner
   - Issue #9: CrowdSec button truncation
   - Issue #11: Sidebar reorganization

4. **Low (Enhancement)**:
   - Issue #5: Template UX redesign

---

## 🕵️ Phase 3: QA & Security

### Test Scenarios

1. **Uptime Sync**:
   - Edit proxy host name → Verify uptime card updates
   - Edit proxy host domain → Verify uptime URL updates
   - Delete proxy host with uptime → Verify cleanup

2. **Certificate Delete**:
   - Delete custom certificate (not in use)
   - Attempt delete certificate in use → Expect error
   - Delete staging certificate

3. **Responsive Testing**:
   - Access via localhost:8080 vs domain
   - Mobile viewport (< 768px)
   - Tablet viewport (768px - 1024px)
   - Desktop viewport (> 1024px)

4. **ACL Operations**:
   - Add ACL to single host → Verify overlay
   - Remove ACL from single host → Verify overlay
   - Bulk add ACL → Verify overlay with progress
   - Bulk remove ACL → Verify overlay

5. **CrowdSec Page**:
   - Navigate to /security/crowdsec directly
   - Navigate via Security dashboard button
   - Verify header/sidebar present

6. **Sidebar Navigation**:
   - Verify Users moved under Settings
   - Verify old /users route redirects or 404s appropriately
   - Verify Account Management accessible

---

## 📚 Phase 4: Documentation

### Files to Update

- `docs/features.md` - Update if any new features added
- Component JSDoc comments for modified files

---

## Implementation Sequence

```
1. Frontend Dev: Fix CSS zoom issue (#3) - Quick win, affects all users
2. Frontend Dev: Fix notification dropdown (#4) - High visibility bug
3. Frontend Dev: Fix CrowdSec page (#10) - Route/layout issue
4. Backend Dev: Implement uptime sync on proxy host edit (#1)
5. Frontend Dev: Fix certificate delete visibility (#2)
6. Frontend Dev: Verify ACL operation loading overlay (#12)
7. Frontend Dev: Mobile header/banner fixes (#6, #7, #8)
8. Frontend Dev: CrowdSec card responsive buttons (#9)
9. Frontend Dev: Sidebar reorganization (#11)
10. Frontend Dev: Notification template UX improvement (#5)
11. QA and Security: Test all changes
```

---

## Subagent Assignments

| Agent | Tasks |
|-------|-------|
| **Backend Dev** | Issue #1 (uptime sync on proxy host edit) |
| **Frontend Dev** | Issues #2, #3, #4, #5, #6, #7, #8, #9, #10, #11, #12 |
| **QA and Security** | All verification testing |
| **Doc Writer** | Update docs/features.md if needed |
