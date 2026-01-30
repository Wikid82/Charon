# Reddit Feedback Implementation Plan: Logs UI, Caddy Import, Settings 400 Errors

**Version:** 1.1
**Status:** Supervisor Review Complete - Ready for Implementation
**Priority:** HIGH
**Created:** 2026-01-29
**Updated:** 2026-01-30
**Source:** Reddit user feedback

---

## Executive Summary

Three user-reported issues from Reddit requiring investigation and fixes:

1. **Logs UI on widescreen** - "logs on widescreen could be stretched and one line for better view (and more lines)"
2. **Caddy import not working** - "import from my caddy is not working"
3. **Settings 400 errors** - "i'm getting several i think error 400 on saving different settings"

---

## Issue 1: Logs UI Widescreen Enhancement

### Root Cause Analysis

**Affected File:** [frontend/src/components/LiveLogViewer.tsx](../../frontend/src/components/LiveLogViewer.tsx)

**Current Implementation (Lines 435-440):**
```tsx
<div
  ref={logContainerRef}
  onScroll={handleScroll}
  className="h-96 overflow-y-auto p-3 font-mono text-xs bg-black"
  style={{ scrollBehavior: 'smooth' }}
>
```

**Problems Identified:**

1. **Fixed height (`h-96` = 384px)** - Does not adapt to viewport, wastes vertical space on large monitors
2. **Multi-span log entries (Lines 448-497)** - Each log has multiple `<span>` elements wrapped to new lines:
   - Timestamp
   - Source badge (security mode)
   - Level badge (application mode)
   - Client IP
   - Message
   - Block reason
   - Status code
   - Duration
   - Details object
3. **No horizontal layout optimization** - Missing `whitespace-nowrap` for single-line display
4. **Font size fixed at `text-xs`** - No density options for users who prefer more/fewer lines

### Requirements (EARS Notation)

**R1.1 - Responsive Height**
WHEN the LiveLogViewer is rendered on a viewport taller than 768px,
THE SYSTEM SHALL expand the log container to use available vertical space (minimum 50vh).

**R1.2 - Single-Line Log Format**
WHEN the user enables "compact mode",
THE SYSTEM SHALL display each log entry on a single line with horizontal scrolling.

**R1.3 - Display Density Control**
WHERE the logs panel settings are available,
THE SYSTEM SHALL provide font size options: compact (text-xs), normal (text-sm), comfortable (text-base).

**R1.4 - Preserve Existing Features**
WHEN displaying logs in any mode,
THE SYSTEM SHALL maintain all existing functionality: filtering, auto-scroll, pause, source badges, level colors.

### Implementation Approach

**Phase 1A: Responsive Height (Priority: Critical)**

**File:** `frontend/src/components/LiveLogViewer.tsx`

Replace fixed height with flex layout (avoids brittle pixel calculations):

```tsx
// Wrap the log viewer in a flex container
<div className="flex flex-col h-[calc(100vh-200px)]">
  <div className="flex-shrink-0">
    {/* Filter bar - fixed at top */}
  </div>
  <div
    ref={logContainerRef}
    onScroll={handleScroll}
    className="flex-1 min-h-0 overflow-y-auto p-3 font-mono text-xs bg-black"
    style={{ scrollBehavior: 'smooth' }}
  >
    {/* Log entries */}
  </div>
</div>
```

**Key Changes:**
- `flex flex-col` - Vertical flex container
- `flex-shrink-0` - Filter bar doesn't shrink
- `flex-1 min-h-0` - Log area grows to fill remaining space, `min-h-0` allows overflow scroll
- Removed brittle `calc(100vh-300px)` in favor of flex layout

**Phase 1B: Single-Line Compact Mode (Priority: High)**

Add state and UI toggle:

```tsx
// Add after line ~55 (state declarations)
const [compactMode, setCompactMode] = useState(false);

// Add toggle in filter bar (after line ~395)
<label
  className="flex items-center gap-1 text-xs text-gray-400 cursor-pointer"
  aria-label="Enable compact single-line log display"
>
  <input
    type="checkbox"
    checked={compactMode}
    onChange={(e) => setCompactMode(e.target.checked)}
    className="rounded border-gray-600 bg-gray-700 text-blue-500 focus:ring-blue-500"
  />
  Compact
</label>
```

**Log Entry Scroll Behavior (UX Decision):**

**Chosen: Option B - Truncate with Tooltip**

Rationale: Per-entry horizontal scrolling (Option A) creates a poor UX with multiple scrollbars and makes it difficult to scan logs quickly. Truncation with tooltips provides:
- Clean visual appearance
- Full text available on hover
- No horizontal scrollbar clutter
- Better accessibility (screen readers announce full text)

```tsx
{/* Log entry container - truncate long messages with tooltip */}
<div
  key={index}
  className={`
    ${compactMode
      ? 'flex items-center gap-2 truncate'
      : 'mb-1'}
    hover:bg-gray-900 px-1 -mx-1 rounded ${getEntryStyle(log)}
  `}
  title={compactMode ? formatFullLogEntry(log) : undefined}
  data-testid="log-entry"
>
  {/* In compact mode, all spans are inline with flex-shrink-0 */}
  {/* Timestamp */}
  <span className={`text-gray-500 ${compactMode ? 'flex-shrink-0' : ''}`}>
    {formatTimestamp(log.timestamp)}
  </span>
  {/* Message - truncate in compact mode */}
  <span className={`text-gray-300 ${compactMode ? 'truncate' : ''}`}>
    {log.message}
  </span>
  {/* ... rest of spans */}
</div>

// Helper function for tooltip
const formatFullLogEntry = (log: LogEntry): string => {
  return `${formatTimestamp(log.timestamp)} [${log.level}] ${log.client_ip || ''} ${log.message}`;
};
```

**Phase 1C: Font Size/Density Control (Priority: Medium)**

```tsx
// State for density
const [density, setDensity] = useState<'compact' | 'normal' | 'comfortable'>('compact');

// Density dropdown in filter bar
<select
  value={density}
  onChange={(e) => setDensity(e.target.value as typeof density)}
  className="px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded text-white"
>
  <option value="compact">Compact</option>
  <option value="normal">Normal</option>
  <option value="comfortable">Comfortable</option>
</select>

// Dynamic font class
const fontSizeClass = {
  compact: 'text-xs',
  normal: 'text-sm',
  comfortable: 'text-base',
}[density];

// Apply to container
className={`... font-mono ${fontSizeClass} bg-black`}
```

### Testing Strategy

**Unit Tests:** `frontend/src/components/__tests__/LiveLogViewer.test.tsx`
- Test compact mode toggle renders single-line entries
- Test density selection changes font class
- Test responsive height classes applied
- Test truncation with tooltip in compact mode

**E2E Tests:** `tests/live-logs.spec.ts`
- Verify compact mode toggle works
- Verify tooltips show full log content on hover
- Verify density selector changes log appearance
- Visual regression tests for widescreen layout

### Files to Modify

| File | Changes |
|------|---------|
| [frontend/src/components/LiveLogViewer.tsx](../../frontend/src/components/LiveLogViewer.tsx#L435) | Responsive height, compact mode, density control |
| `tests/live-logs.spec.ts` | E2E tests for new features |

### Complexity Estimate

- Phase 1A (Responsive): **S** (1-2 hours)
- Phase 1B (Compact): **M** (3-4 hours)
- Phase 1C (Density): **S** (1-2 hours)

---

## Issue 2: Caddy Import Not Working

### Root Cause Analysis

**Affected Files:**
- [backend/internal/api/handlers/import_handler.go](../../backend/internal/api/handlers/import_handler.go)
- [backend/internal/caddy/importer.go](../../backend/internal/caddy/importer.go)
- [frontend/src/pages/ImportCaddy.tsx](../../frontend/src/pages/ImportCaddy.tsx)

**Import Flow:**
1. User uploads/pastes Caddyfile content
2. Backend writes to temp file (`import/uploads/<uuid>.caddyfile`)
3. Calls `caddy adapt --config <file> --adapter caddyfile` to convert to JSON
4. Parses JSON to extract `reverse_proxy` handlers
5. Returns hosts for review

**Potential Failure Points Identified:**

**Point A: Caddy Binary Not Available (Lines 12-17 of importer.go)**
```go
func (i *Importer) ValidateCaddyBinary() error {
    _, err := i.executor.Execute(i.caddyBinaryPath, "version")
    if err != nil {
        return errors.New("caddy binary not found or not executable")
    }
    return nil
}
```
- User's system may not have `caddy` in PATH
- Docker container has it, but local dev might not

**Point B: Complex Caddyfile Syntax (Lines 280-286 of importer.go)**
```go
if handler.Handler == "rewrite" {
    host.Warnings = append(host.Warnings, "Rewrite rules not supported")
}
if handler.Handler == "file_server" {
    host.Warnings = append(host.Warnings, "File server directives not supported")
}
```
- Only `reverse_proxy` handlers are extracted
- `file_server`, `rewrite`, `respond`, `redir` are not imported
- Snippet blocks and macros may confuse the parser

**Point C: Import Directives Require Multi-File (Lines 297-305 of import_handler.go)**
```go
if len(result.Hosts) == 0 {
    imports := detectImportDirectives(req.Content)
    if len(imports) > 0 {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "no sites found in uploaded Caddyfile; imports detected; please upload the referenced site files using the multi-file import flow",
            "imports": imports,
        })
        return
    }
}
```
- If Caddyfile uses `import ./sites.d/*`, hosts are in external files
- User must use multi-file upload flow but may not realize this

**Point D: Silent Host Skipping (Lines 315-318 of importer.go)**
```go
if parsed.ForwardHost == "" || parsed.ForwardPort == 0 {
    continue // Skip invalid entries
}
```
- Hosts with no `reverse_proxy` backend are silently skipped
- No feedback to user about which sites were ignored

**Point E: Parse Errors Not Surfaced**
- If `caddy adapt` fails, error is returned but may be cryptic
- User sees "import failed: <error>" without actionable guidance

### Requirements (EARS Notation)

**R2.1 - Import Directive Detection**
WHEN user uploads a Caddyfile containing import directives,
THE SYSTEM SHALL detect the import paths and prompt user to use multi-file import flow.

**R2.2 - Parse Error Clarity**
WHEN `caddy adapt` fails to parse the Caddyfile,
THE SYSTEM SHALL return a user-friendly error message with the line number and suggestion.

**R2.3 - Skipped Hosts Feedback**
WHEN hosts are skipped due to missing `reverse_proxy` configuration,
THE SYSTEM SHALL include a list of skipped domains with reasons in the response.

**R2.4 - Supported Directives Documentation**
WHEN displaying the import wizard,
THE SYSTEM SHALL show a list of supported Caddyfile directives and known limitations.

**R2.5 - Caddy Binary Validation**
WHEN the import handler initializes,
THE SYSTEM SHALL validate that the Caddy binary is available and return a clear error if not.

### Implementation Approach

**Phase 2A: Enhanced Error Messages (Priority: Critical)**

**File:** `backend/internal/caddy/importer.go`

```go
// ParseCaddyfile - enhance error handling
func (i *Importer) ParseCaddyfile(caddyfilePath string) ([]byte, error) {
    output, err := i.executor.Execute(i.caddyBinaryPath, "adapt", "--config", caddyfilePath, "--adapter", "caddyfile")
    if err != nil {
        // Parse caddy error output for line numbers
        errMsg := string(output)
        if strings.Contains(errMsg, "line") {
            return nil, fmt.Errorf("Caddyfile syntax error: %s", extractLineError(errMsg))
        }
        return nil, fmt.Errorf("failed to parse Caddyfile: %v", err)
    }
    return output, nil
}

func extractLineError(errOutput string) string {
    // Extract "line X: error message" format from caddy output
    re := regexp.MustCompile(`(?i)line\s+(\d+):\s*(.+)`)
    if match := re.FindStringSubmatch(errOutput); len(match) > 2 {
        return fmt.Sprintf("Line %s: %s", match[1], match[2])
    }
    return errOutput
}
```

**Phase 2B: Skipped Hosts Feedback (Priority: High)**

**File:** `backend/internal/caddy/importer.go`

```go
type ImportResult struct {
    Hosts    []ParsedHost
    Skipped  []SkippedHost  // NEW: Add skipped hosts tracking
    Warnings []string
    ParsedAt time.Time
}

type SkippedHost struct {
    DomainNames string `json:"domain_names"`
    Reason      string `json:"reason"`
}

// In ConvertToProxyHosts
func ConvertToProxyHostsWithSkipped(parsedHosts []ParsedHost) ([]models.ProxyHost, []SkippedHost) {
    hosts := make([]models.ProxyHost, 0)
    skipped := make([]SkippedHost, 0)

    for _, parsed := range parsedHosts {
        if parsed.ForwardHost == "" || parsed.ForwardPort == 0 {
            skipped = append(skipped, SkippedHost{
                DomainNames: parsed.DomainNames,
                Reason:      "No reverse_proxy backend defined",
            })
            continue
        }
        // ... normal conversion
    }
    return hosts, skipped
}
```

**File:** `backend/internal/api/handlers/import_handler.go`

```go
// In Upload handler response (line ~335)
c.JSON(http.StatusOK, gin.H{
    "session":          gin.H{"id": sid, "state": "transient"},
    "preview":          transient,
    "conflict_details": conflictDetails,
    "skipped_hosts":    skippedHosts,  // NEW: Include in response
})
```

**Phase 2C: Frontend Skipped Hosts Display (Priority: High)**

**File:** `frontend/src/pages/ImportCaddy.tsx`

```tsx
// Add skipped hosts display in review step
// Note: Use snake_case to match backend JSON field naming convention
{importData.skipped_hosts?.length > 0 && (
  <div className="mt-4 p-4 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
    <h4 className="font-medium text-yellow-800 dark:text-yellow-200 mb-2">
      Skipped Sites ({importData.skipped_hosts.length})
    </h4>
    <p className="text-sm text-yellow-700 dark:text-yellow-300 mb-2">
      The following sites were not imported because they don't have a reverse proxy configuration:
    </p>
    <ul className="text-sm text-yellow-600 dark:text-yellow-400 list-disc pl-5">
      {importData.skipped_hosts.map((host: {domain_names: string, reason: string}) => (
        <li key={host.domain_names}>{host.domain_names} - {host.reason}</li>
      ))}
    </ul>
  </div>
)}
```

**Phase 2D: Import Directive Auto-Detection UI (Priority: Medium)**

**File:** `frontend/src/pages/ImportCaddy.tsx`

When error contains `imports` array, show multi-file upload prompt:

```tsx
// In error handling
if (error.imports && error.imports.length > 0) {
  return (
    <div className="p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
      <h4 className="font-medium text-blue-800 dark:text-blue-200">
        Import Directives Detected
      </h4>
      <p className="text-sm mt-2">
        Your Caddyfile references external files. Please use the multi-file import:
      </p>
      <ul className="text-sm mt-2 list-disc pl-5">
        {error.imports.map((imp: string) => (
          <li key={imp}>{imp}</li>
        ))}
      </ul>
      <Button onClick={() => setShowMultiFileUpload(true)} className="mt-4">
        Switch to Multi-File Import
      </Button>
    </div>
  );
}
```

### Testing Strategy

**Unit Tests:** `backend/internal/caddy/importer_test.go`
- Test parse error extraction with line numbers
- Test skipped hosts tracking
- Test import directive detection

**E2E Tests:** `tests/tasks/import-caddyfile.spec.ts`
- Test error message display for invalid Caddyfile
- Test skipped hosts warning display
- Test import directive detection and multi-file prompt

**Backward Compatibility E2E Test:** `tests/tasks/import-caddyfile.spec.ts`
```typescript
test('existing import flow works for standard Caddyfile', async ({ page }) => {
  // Navigate to import page
  await page.goto('/import');

  // Upload known-good Caddyfile with reverse_proxy
  const caddyfileContent = `
    example.com {
      reverse_proxy localhost:8080
    }
    api.example.com {
      reverse_proxy localhost:3000
    }
  `;

  await page.getByTestId('caddyfile-input').fill(caddyfileContent);
  await page.getByRole('button', { name: /parse|upload/i }).click();

  // Verify hosts are detected
  await expect(page.getByText('example.com')).toBeVisible();
  await expect(page.getByText('api.example.com')).toBeVisible();

  // Verify no regression - import can proceed
  await expect(page.getByRole('button', { name: /import|confirm/i })).toBeEnabled();
});
```

### Files to Modify

| File | Changes |
|------|---------|
| [backend/internal/caddy/importer.go](../../backend/internal/caddy/importer.go) | Skipped host tracking, error parsing |
| [backend/internal/api/handlers/import_handler.go](../../backend/internal/api/handlers/import_handler.go#L335) | Include skipped hosts in response |
| [frontend/src/pages/ImportCaddy.tsx](../../frontend/src/pages/ImportCaddy.tsx) | Display skipped hosts, import directive UI |

### Complexity Estimate

- Phase 2A (Error Messages): **S** (1-2 hours)
- Phase 2B (Skipped Feedback Backend): **M** (2-3 hours)
- Phase 2C (Skipped Feedback UI): **S** (1-2 hours)
- Phase 2D (Import Directive UI): **M** (2-3 hours)

---

## Issue 3: Error 400 on Saving Settings

### Root Cause Analysis

**Affected File:** [backend/internal/api/handlers/settings_handler.go](../../backend/internal/api/handlers/settings_handler.go)

**400 Error Sources Identified:**

| Line | Condition | Current Error Message |
|------|-----------|----------------------|
| 84-85 | `ShouldBindJSON` failure | `err.Error()` (cryptic Go validation) |
| 88-91 | Invalid admin whitelist CIDR | `"Invalid admin_whitelist: <err>"` |
| 136-143 | `syncAdminWhitelist` failure | `"Invalid admin_whitelist"` |
| 185-192 | Bulk settings bind failure | `err.Error()` |
| 418-420 | SMTP config bind failure | `err.Error()` |
| 480-484 | Test email bind failure | `err.Error()` |

**Common User Mistakes:**

1. **Admin Whitelist CIDR Format**
   - User enters: `192.168.1.1` (no CIDR suffix)
   - Expected: `192.168.1.1/32` or `192.168.1.0/24`
   - Error: `"Invalid admin_whitelist: invalid CIDR notation"`

2. **Public URL Format**
   - User enters: `mysite.com` (no scheme)
   - Expected: `https://mysite.com`
   - Error: `"Invalid URL format"`

3. **Missing Required Fields**
   - User sends: `{"key": "theme"}` (missing value)
   - Error: `"Key: 'UpdateSettingRequest.Value' Error:Field validation for 'Value' failed on the 'required' tag"`

4. **Boolean String Format**
   - User sends: `{"value": true}` (JSON boolean instead of string)
   - Error: JSON binding error

### Requirements (EARS Notation)

**R3.1 - User-Friendly Validation Errors**
WHEN a setting validation fails,
THE SYSTEM SHALL return a clear error message explaining the expected format with examples.

**R3.2 - CIDR Auto-Correction**
WHEN user enters an IP without CIDR suffix for admin_whitelist,
THE SYSTEM SHALL automatically append `/32` for IPv4 single IPs and `/128` for IPv6 single IPs.

**R3.3 - URL Auto-Correction**
WHEN user enters a URL without scheme for public_url that contains a recognized TLD,
THE SYSTEM SHALL automatically prepend `https://`.
WHEN user enters a private IP, localhost, or domain without TLD,
THE SYSTEM SHALL leave the value unchanged (prompting validation error if scheme is required).

**R3.4 - Frontend Validation**
WHEN user enters a value in a settings form,
THE SYSTEM SHALL validate the format client-side before submission.

**R3.5 - Specific Error Field Identification**
WHEN multiple settings are submitted via bulk update,
THE SYSTEM SHALL identify which specific setting failed validation.

### Implementation Approach

**Phase 3A: Backend Error Message Enhancement (Priority: Critical)**

**File:** `backend/internal/api/handlers/settings_handler.go`

```go
// Enhanced UpdateSetting error handling
func (h *SettingsHandler) UpdateSetting(c *gin.Context) {
    var req UpdateSettingRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        // Map validation errors to user-friendly messages
        c.JSON(http.StatusBadRequest, gin.H{
            "error":   formatValidationError(err),
            "details": gin.H{"field": extractFieldName(err)},
        })
        return
    }

    // Auto-correct common mistakes
    if req.Key == "security.admin_whitelist" {
        req.Value = normalizeCIDR(req.Value)
        if err := validateAdminWhitelist(req.Value); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": "Invalid IP/CIDR format. Use format like 192.168.1.0/24 or 10.0.0.1/32",
                "details": gin.H{
                    "field":    "admin_whitelist",
                    "received": req.Value,
                    "examples": []string{"192.168.1.0/24", "10.0.0.0/8", "192.168.1.100/32"},
                },
            })
            return
        }
    }

    if req.Key == "general.public_url" {
        req.Value = normalizeURL(req.Value)
    }
    // ... rest of handler
}

// Auto-append /32 for IPv4 and /128 for IPv6 single IPs
func normalizeCIDR(value string) string {
    entries := strings.Split(value, ",")
    normalized := make([]string, 0, len(entries))
    for _, entry := range entries {
        entry = strings.TrimSpace(entry)
        if entry == "" {
            continue
        }
        if !strings.Contains(entry, "/") {
            // Check if it's a valid IP first
            ip := net.ParseIP(entry)
            if ip != nil {
                if ip.To4() != nil {
                    entry = entry + "/32"  // IPv4
                } else {
                    entry = entry + "/128" // IPv6
                }
            }
        }
        normalized = append(normalized, entry)
    }
    return strings.Join(normalized, ", ")
}

// Auto-prepend https:// for URLs with recognized TLDs only
// Private IPs and localhost are left unchanged for explicit user handling
func normalizeURL(value string) string {
    value = strings.TrimSpace(value)
    if value == "" {
        return value
    }
    if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
        return value
    }

    // Extract hostname (before any port or path)
    host := value
    if colonIdx := strings.Index(value, ":"); colonIdx != -1 {
        host = value[:colonIdx]
    }
    if slashIdx := strings.Index(host, "/"); slashIdx != -1 {
        host = host[:slashIdx]
    }

    // Don't auto-add scheme for private IPs, localhost, or no TLD
    if isPrivateOrLocal(host) {
        return value // Leave as-is; validation will prompt user if scheme needed
    }

    // Check for recognized TLD (public domain)
    if hasRecognizedTLD(host) {
        return "https://" + value
    }

    return value // Leave as-is for ambiguous cases
}

// Check if host is private IP, localhost, or local network
func isPrivateOrLocal(host string) bool {
    // Check localhost variants
    if host == "localhost" || strings.HasSuffix(host, ".local") {
        return true
    }

    // Check if it's an IP address
    ip := net.ParseIP(host)
    if ip != nil {
        // Check private ranges (RFC 1918, RFC 4193)
        privateNets := []string{
            "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", // IPv4 private
            "127.0.0.0/8",                                   // IPv4 loopback
            "::1/128", "fc00::/7", "fe80::/10",              // IPv6 loopback/private/link-local
        }
        for _, cidr := range privateNets {
            _, network, _ := net.ParseCIDR(cidr)
            if network.Contains(ip) {
                return true
            }
        }
    }
    return false
}

// Check if host has a recognized public TLD
func hasRecognizedTLD(host string) bool {
    // Common TLDs - can extend as needed
    tlds := []string{".com", ".org", ".net", ".io", ".dev", ".app", ".co", ".me", ".info", ".biz", ".edu", ".gov"}
    hostLower := strings.ToLower(host)
    for _, tld := range tlds {
        if strings.HasSuffix(hostLower, tld) {
            return true
        }
    }
    // Country code TLDs (2 chars after dot)
    if parts := strings.Split(hostLower, "."); len(parts) >= 2 {
        lastPart := parts[len(parts)-1]
        if len(lastPart) == 2 { // Likely a ccTLD
            return true
        }
    }
    return false
}

// Map Go validation errors to user-friendly messages
func formatValidationError(err error) string {
    errStr := err.Error()

    switch {
    case strings.Contains(errStr, "'required' tag"):
        return "This field is required"
    case strings.Contains(errStr, "'url' tag"):
        return "Invalid URL format. Use format like https://example.com"
    case strings.Contains(errStr, "'email' tag"):
        return "Invalid email format"
    case strings.Contains(errStr, "cannot unmarshal"):
        return "Invalid value type. Expected a text string."
    default:
        return "Invalid input format"
    }
}
```

**Phase 3B: Frontend Form Validation (Priority: High)**

**File:** `frontend/src/pages/Settings.tsx` (or relevant settings component)

```tsx
// CIDR validation helper - supports IPv4 and IPv6
const validateCIDR = (value: string): string | null => {
  if (!value.trim()) return null; // Empty is ok

  const entries = value.split(',').map(e => e.trim());
  for (const entry of entries) {
    // Check IPv4 CIDR format
    const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/;
    // Check IPv6 format (simplified - accepts common formats)
    const ipv6Regex = /^([0-9a-fA-F:]+)(\/\d{1,3})?$/;

    if (!ipv4Regex.test(entry) && !ipv6Regex.test(entry)) {
      return `Invalid format: ${entry}. Use format like 192.168.1.0/24 or 2001:db8::/32`;
    }

    // Additional validation for IPv4 octets
    if (ipv4Regex.test(entry)) {
      const ip = entry.split('/')[0];
      const octets = ip.split('.').map(Number);
      if (octets.some(o => o > 255)) {
        return `Invalid IP: ${entry}. Octets must be 0-255`;
      }
    }
  }
  return null;
};

// URL validation helper - tolerates private IPs/localhost
const validateURL = (value: string): string | null => {
  if (!value.trim()) return null;

  // Check for obviously malformed URLs
  if (value.startsWith('://') || value === 'http://' || value === 'https://') {
    return 'Invalid URL format. Use format like https://example.com';
  }

  // For URLs with scheme, validate structure
  if (value.startsWith('http://') || value.startsWith('https://')) {
    try {
      new URL(value);
      return null;
    } catch {
      return 'Invalid URL format. Use format like https://example.com';
    }
  }

  // For values without scheme, allow private IPs and localhost without validation
  // Backend will handle auto-normalization for public domains
  return null;
};

// In form component
const [errors, setErrors] = useState<Record<string, string>>({});

const validateAndSubmit = async () => {
  const newErrors: Record<string, string> = {};

  // Validate each field
  if (formData.admin_whitelist) {
    const err = validateCIDR(formData.admin_whitelist);
    if (err) newErrors.admin_whitelist = err;
  }

  if (formData.public_url) {
    const err = validateURL(formData.public_url);
    if (err) newErrors.public_url = err;
  }

  if (Object.keys(newErrors).length > 0) {
    setErrors(newErrors);
    return;
  }

  // Submit
  await saveSetting(formData);
};
```

**Phase 3C: Inline Validation UI (Priority: Medium)**

```tsx
// Field with validation
<div className="form-group">
  <label>Admin Whitelist (CIDR)</label>
  <input
    value={adminWhitelist}
    onChange={(e) => setAdminWhitelist(e.target.value)}
    onBlur={() => setErrors({...errors, admin_whitelist: validateCIDR(adminWhitelist) || ''})}
    className={errors.admin_whitelist ? 'border-red-500' : ''}
    placeholder="192.168.1.0/24, ::1/128"
  />
  {errors.admin_whitelist && (
    <p className="text-red-500 text-sm mt-1">{errors.admin_whitelist}</p>
  )}
  <p className="text-gray-500 text-xs mt-1">
    Examples: 192.168.1.0/24, 10.0.0.1/32, ::1/128, 2001:db8::/32
  </p>
</div>

<div className="form-group">
  <label>Public URL</label>
  <input
    value={publicUrl}
    onChange={(e) => setPublicUrl(e.target.value)}
    onBlur={() => setErrors({...errors, public_url: validateURL(publicUrl) || ''})}
    className={errors.public_url ? 'border-red-500' : ''}
    placeholder="https://example.com"
  />
  {errors.public_url && (
    <p className="text-red-500 text-sm mt-1">{errors.public_url}</p>
  )}
  <p className="text-gray-500 text-xs mt-1">
    For public domains, https:// will be added automatically. Private IPs (192.168.x.x, localhost) require explicit scheme.
  </p>
</div>
```

### Testing Strategy

**Unit Tests:** `backend/internal/api/handlers/settings_handler_test.go`
- Test CIDR normalization (`192.168.1.1` → `192.168.1.1/32`)
- Test IPv6 CIDR normalization:
  - `::1` → `::1/128`
  - `2001:db8::1` → `2001:db8::1/128`
  - `fe80::1` → `fe80::1/128`
- Test mixed IPv4/IPv6 lists: `192.168.1.1, ::1` → `192.168.1.1/32, ::1/128`
- Test URL normalization:
  - `example.com` → `https://example.com`
  - `mysite.org/path` → `https://mysite.org/path`
  - `192.168.1.1:8080` → `192.168.1.1:8080` (unchanged - private IP)
  - `localhost:3000` → `localhost:3000` (unchanged - localhost)
  - `10.0.0.1` → `10.0.0.1` (unchanged - private IP)
  - `https://example.com` → `https://example.com` (unchanged - already has scheme)
  - `http://internal.local` → `http://internal.local` (unchanged - already has scheme)
- Test user-friendly error messages
- Test bulk settings with one invalid entry

**Frontend Validation Unit Tests:** `frontend/src/hooks/__tests__/useSettings.test.ts`
```typescript
import { validateCIDR, validateURL } from '../useSettings';

describe('validateCIDR', () => {
  test('accepts valid IPv4 CIDR', () => {
    expect(validateCIDR('192.168.1.0/24')).toBeNull();
    expect(validateCIDR('10.0.0.1/32')).toBeNull();
  });

  test('accepts valid IPv6 CIDR', () => {
    expect(validateCIDR('::1/128')).toBeNull();
    expect(validateCIDR('2001:db8::/32')).toBeNull();
  });

  test('accepts IP without CIDR (will be auto-normalized)', () => {
    expect(validateCIDR('192.168.1.1')).toBeNull();
    expect(validateCIDR('::1')).toBeNull();
  });

  test('accepts comma-separated list', () => {
    expect(validateCIDR('192.168.1.1, 10.0.0.0/8, ::1')).toBeNull();
  });

  test('rejects invalid IP', () => {
    expect(validateCIDR('999.999.999.999')).toContain('Invalid');
    expect(validateCIDR('not-an-ip')).toContain('Invalid');
  });

  test('empty value is valid', () => {
    expect(validateCIDR('')).toBeNull();
  });
});

describe('validateURL', () => {
  test('accepts full URL with scheme', () => {
    expect(validateURL('https://example.com')).toBeNull();
    expect(validateURL('http://localhost:3000')).toBeNull();
  });

  test('accepts domain without scheme (will be auto-normalized for public)', () => {
    expect(validateURL('example.com')).toBeNull();
    expect(validateURL('mysite.org')).toBeNull();
  });

  test('accepts private IP without scheme', () => {
    expect(validateURL('192.168.1.1:8080')).toBeNull();
    expect(validateURL('localhost:3000')).toBeNull();
  });

  test('rejects malformed URL', () => {
    expect(validateURL('http://')).toContain('Invalid');
    expect(validateURL('://no-scheme')).toContain('Invalid');
  });

  test('empty value is valid', () => {
    expect(validateURL('')).toBeNull();
  });
});
```

**E2E Tests:** `tests/settings.spec.ts`
- Test inline validation displays error on blur
- Test form submission blocked with invalid data
- Test successful save after fixing validation errors
- Test IPv6 CIDR accepts and saves correctly

### Files to Modify

| File | Changes |
|------|---------|
| [backend/internal/api/handlers/settings_handler.go](../../backend/internal/api/handlers/settings_handler.go#L84) | Normalization, enhanced errors |
| `backend/internal/api/handlers/settings_handler_test.go` | Unit tests for CIDR/URL normalization |
| `frontend/src/pages/Settings.tsx` | Client-side validation |
| `frontend/src/hooks/useSettings.ts` | Validation helpers (validateCIDR, validateURL) |
| `frontend/src/hooks/__tests__/useSettings.test.ts` | Unit tests for validation helpers |

### Complexity Estimate

- Phase 3A (Backend Normalization): **M** (2-3 hours)
- Phase 3B (Frontend Validation): **M** (3-4 hours)
- Phase 3C (Inline UI): **S** (1-2 hours)

---

## Implementation Phases

### Phase 1: Quick Wins (Day 1)

| Task | Issue | Priority | Estimate |
|------|-------|----------|----------|
| 1A: Responsive log height | Logs UI | Critical | 1-2h |
| 2A: Enhanced error messages | Import | Critical | 1-2h |
| 3A: Backend normalization | Settings 400 | Critical | 2-3h |

### Phase 2: Core Features (Day 2)

| Task | Issue | Priority | Estimate |
|------|-------|----------|----------|
| 1B: Compact log mode | Logs UI | High | 3-4h |
| 2B: Skipped hosts backend | Import | High | 2-3h |
| 3B: Frontend validation | Settings 400 | High | 3-4h |

### Phase 3: Polish (Day 3)

| Task | Issue | Priority | Estimate |
|------|-------|----------|----------|
| 1C: Density control | Logs UI | Medium | 1-2h |
| 2C: Skipped hosts UI | Import | High | 1-2h |
| 2D: Import directive UI | Import | Medium | 2-3h |
| 3C: Inline validation UI | Settings 400 | Medium | 1-2h |

---

## Testing Requirements

### Unit Test Coverage

- `frontend/src/components/__tests__/LiveLogViewer.test.tsx` - Compact mode, density
- `frontend/src/hooks/__tests__/useSettings.test.ts` - CIDR/URL validation helpers
- `backend/internal/caddy/importer_test.go` - Skipped hosts, error parsing
- `backend/internal/api/handlers/settings_handler_test.go` - Normalization, errors (IPv4/IPv6)

### E2E Test Coverage

- `tests/live-logs.spec.ts` - Widescreen layout, compact mode
- `tests/tasks/import-caddyfile.spec.ts` - Error messages, skipped hosts, backward compatibility
- `tests/settings.spec.ts` - Validation, 400 error handling

### Patch Coverage Requirement

All modified lines must have 100% patch coverage per Codecov requirements.

---

## Success Criteria

### Issue 1: Logs UI
- [ ] Log container height adapts to viewport using flex layout
- [ ] Compact mode displays single-line log entries with truncation
- [ ] Tooltips show full log content on hover in compact mode
- [ ] Density control allows font size selection
- [ ] Compact mode toggle has `aria-label` for accessibility
- [ ] All existing log features continue to work

### Issue 2: Caddy Import
- [ ] Parse errors include line numbers and suggestions
- [ ] Skipped hosts are listed with reasons (using snake_case JSON fields)
- [ ] Import directive detection prompts multi-file flow
- [ ] Import succeeds for standard Caddyfile with `reverse_proxy`
- [ ] Backward compatibility E2E test passes for existing import flows

### Issue 3: Settings 400
- [ ] IPv4 CIDR entries auto-normalized (IP → IP/32)
- [ ] IPv6 CIDR entries auto-normalized (IP → IP/128)
- [ ] Mixed IPv4/IPv6 lists normalized correctly
- [ ] URLs auto-normalized only for public domains with TLDs
- [ ] Private IPs and localhost left unchanged (explicit user handling)
- [ ] Error messages are user-friendly with examples
- [ ] Client-side validation prevents invalid submissions
- [ ] Frontend validation helpers have 100% unit test coverage

---

## Optional Enhancements (Post-MVP)

The following suggestions are non-blocking improvements to consider after initial implementation:

### Issue 1: Logs UI
- **Persist User Preferences:** Store `compactMode` and `density` settings in localStorage
  ```tsx
  useEffect(() => {
    const saved = localStorage.getItem('logViewerPrefs');
    if (saved) {
      const prefs = JSON.parse(saved);
      setCompactMode(prefs.compactMode ?? false);
      setDensity(prefs.density ?? 'compact');
    }
  }, []);

  useEffect(() => {
    localStorage.setItem('logViewerPrefs', JSON.stringify({ compactMode, density }));
  }, [compactMode, density]);
  ```
- **Log Virtualization:** For very large log volumes, consider `react-window` for virtual scrolling

### Issue 2: Caddy Import
- **Common Fixes Section:** Add a collapsible "Common Issues" panel with parse error suggestions:
  - "Missing closing brace" → Check line X
  - "Unknown directive" → List of supported directives

### Issue 3: Settings 400
- No additional optional enhancements identified

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Breaking existing log filters | Low | High | Extensive unit/E2E tests |
| CIDR auto-correction edge cases (IPv4) | Medium | Medium | Thorough regex and net.ParseIP testing |
| IPv6 CIDR edge cases (compressed notation) | Medium | Medium | Test with ::1, 2001:db8::, fe80:: variants |
| URL heuristic false positives (auto-adding https://) | Low | Low | Only apply to recognized TLDs; fallback to original |
| Import changes break existing flows | Low | High | Backward compatibility E2E test |
| JSON field naming mismatch (frontend/backend) | Medium | Medium | Explicit json tags + TypeScript types |

---

## Dependencies

- No external dependencies required
- All changes are internal to existing components
- No database schema changes needed

---

## Related Documentation

- [ARCHITECTURE.md](../../ARCHITECTURE.md) - System overview
- [docs/features.md](../features.md) - Feature documentation
- [E2E Test Architecture Plan](./current_spec.md) - Previous active plan

---

*End of Specification*
