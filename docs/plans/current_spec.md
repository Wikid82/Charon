# Implementation Plan: Application URL Setting for User Invitations

**Issue**: When inviting users, verification email links use the internal address (e.g., `localhost:8080`) instead of the public-facing URL. External users cannot access these links.

**Target Files**:

- Backend: [settings_handler.go](../../backend/internal/api/handlers/settings_handler.go), [user_handler.go](../../backend/internal/api/handlers/user_handler.go), [mail_service.go](../../backend/internal/services/mail_service.go)
- Frontend: [SystemSettings.tsx](../../frontend/src/pages/SystemSettings.tsx), [UsersPage.tsx](../../frontend/src/pages/UsersPage.tsx)
- Translations: [translation.json](../../frontend/src/locales/en/translation.json)

---

## 1. Recommended Setting Name: "Application URL"

### Research & Justification

| Application | Setting Name | Notes |
|-------------|--------------|-------|
| Nginx Proxy Manager | N/A | No email system |
| GitLab | `external_url` | Most common in self-hosted apps |
| Grafana | `root_url` | Part of `[server]` config |
| Nextcloud | `overwrite.cli.url` | Complex naming |
| WordPress | `Site Address (URL)` | User-facing name |
| Portainer | `Public URL` | Clean, simple |
| Home Assistant | `external_url` | Paired with `internal_url` |
| Authentik | `AUTHENTIK_HOST` | Environment variable |

### Recommendation: **"Application URL"**

**Why this name:**

1. **User-Friendly**: "Application URL" is clearer than "Base URL" or "External URL" for non-technical users
2. **Self-Explanatory**: Immediately conveys "the URL used to access this application"
3. **Consistent with Charon's naming**: Follows the pattern of human-readable settings (e.g., "Caddy Admin API Endpoint", "SSL Provider")
4. **Internal Key**: `app.public_url` - clear, namespaced, and follows existing `caddy.admin_api` pattern

**Alternative consideration**: "Public URL" is slightly more descriptive but may confuse users who don't understand internal vs external networking.

---

## 2. Database Schema Changes

**No schema changes required.** The existing `settings` table structure supports this:

```go
// backend/internal/models/setting.go (existing)
type Setting struct {
    ID        uint      `json:"id" gorm:"primaryKey"`
    Key       string    `json:"key" gorm:"uniqueIndex"`  // "app.public_url"
    Value     string    `json:"value" gorm:"type:text"`  // "https://charon.example.com"
    Type      string    `json:"type" gorm:"index"`       // "string"
    Category  string    `json:"category" gorm:"index"`   // "general"
    UpdatedAt time.Time `json:"updated_at"`
}
```

The new setting will be stored as:

- **Key**: `app.public_url`
- **Value**: User-provided URL (e.g., `https://charon.example.com`)
- **Type**: `string`
- **Category**: `general`

---

## 3. Backend Changes

### 3.1 Settings Handler Updates

**File**: [backend/internal/api/handlers/settings_handler.go](../../backend/internal/api/handlers/settings_handler.go)

Add a new helper function to retrieve the public URL:

```go
// GetPublicURL retrieves the configured public URL or falls back to request host.
// This should be used for all user-facing URLs (emails, invite links).
func GetPublicURL(db *gorm.DB, c *gin.Context) string {
    var setting models.Setting
    if err := db.Where("key = ?", "app.public_url").First(&setting).Error; err == nil {
        if setting.Value != "" {
            return strings.TrimSuffix(setting.Value, "/")
        }
    }
    // Fallback to request-derived URL
    return getBaseURL(c)
}
```

Add a new endpoint to validate URL format:

```go
// ValidatePublicURL validates a URL is properly formatted and accessible.
func (h *SettingsHandler) ValidatePublicURL(c *gin.Context) {
    var req struct {
        URL string `json:"url" binding:"required,url"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Validate URL structure
    parsed, err := url.Parse(req.URL)
    if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
        c.JSON(http.StatusBadRequest, gin.H{
            "valid": false,
            "error": "URL must start with http:// or https://",
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "valid": true,
        "normalized": strings.TrimSuffix(req.URL, "/"),
    })
}
```

**Route Registration** in [routes.go](../../backend/internal/api/routes/routes.go):

```go
settings.POST("/validate-url", settingsHandler.ValidatePublicURL)
```

### 3.2 User Handler Updates

**File**: [backend/internal/api/handlers/user_handler.go](../../backend/internal/api/handlers/user_handler.go)

**Current Issue** (lines 392-400):

```go
// getBaseURL extracts the base URL from the request.
func getBaseURL(c *gin.Context) string {
    scheme := "https"
    if c.Request.TLS == nil {
        if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
            scheme = proto
        } else {
            scheme = "http"
        }
    }
    return scheme + "://" + c.Request.Host
}
```

**Solution**: Modify `InviteUser` to use the configured public URL:

```go
// InviteUser creates a new user with an invite token and sends an email (admin only).
func (h *UserHandler) InviteUser(c *gin.Context) {
    // ... existing validation code ...

    // Try to send invite email
    emailSent := false
    if h.MailService.IsConfigured() {
        baseURL := GetPublicURL(h.DB, c)  // Changed from getBaseURL(c)
        appName := getAppName(h.DB)
        if err := h.MailService.SendInvite(user.Email, inviteToken, appName, baseURL); err == nil {
            emailSent = true
        }
    }

    // ... rest of handler ...
}
```

Add a new endpoint for previewing the invite URL:

```go
// PreviewInviteURLRequest represents the request for previewing an invite URL.
type PreviewInviteURLRequest struct {
    Email string `json:"email" binding:"required,email"`
}

// PreviewInviteURL returns what the invite URL would look like with current settings.
func (h *UserHandler) PreviewInviteURL(c *gin.Context) {
    role, _ := c.Get("role")
    if role != "admin" {
        c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
        return
    }

    var req PreviewInviteURLRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    baseURL := GetPublicURL(h.DB, c)
    // Generate a sample token for preview (not stored)
    sampleToken := "SAMPLE_TOKEN_PREVIEW"
    inviteURL := fmt.Sprintf("%s/accept-invite?token=%s", strings.TrimSuffix(baseURL, "/"), sampleToken)

    // Check if public URL is configured
    var setting models.Setting
    isConfigured := h.DB.Where("key = ?", "app.public_url").First(&setting).Error == nil && setting.Value != ""

    c.JSON(http.StatusOK, gin.H{
        "preview_url":    inviteURL,
        "base_url":       baseURL,
        "is_configured":  isConfigured,
        "email":          req.Email,
        "warning":        !isConfigured,
        "warning_message": "Application URL not configured. The invite link may not be accessible from external networks.",
    })
}
```

**Route Registration**:

```go
r.POST("/users/preview-invite-url", h.PreviewInviteURL)
```

### 3.3 Mail Service Updates

**File**: [backend/internal/services/mail_service.go](../../backend/internal/services/mail_service.go)

No changes required - the `SendInvite` function already accepts `baseURL` as a parameter:

```go
func (s *MailService) SendInvite(email, inviteToken, appName, baseURL string) error {
    inviteURL := fmt.Sprintf("%s/accept-invite?token=%s", strings.TrimSuffix(baseURL, "/"), inviteToken)
    // ...
}
```

---

## 4. Frontend Changes

### 4.1 System Settings UI

**File**: [frontend/src/pages/SystemSettings.tsx](../../frontend/src/pages/SystemSettings.tsx)

Add a new section for Application URL after the General Configuration card:

```tsx
// Add state
const [publicURL, setPublicURL] = useState('')
const [publicURLValid, setPublicURLValid] = useState<boolean | null>(null)

// Update useEffect to load setting
useEffect(() => {
  if (settings) {
    // ... existing settings ...
    if (settings['app.public_url']) setPublicURL(settings['app.public_url'])
  }
}, [settings])

// Add validation function
const validatePublicURL = async (url: string) => {
  if (!url) {
    setPublicURLValid(null)
    return
  }
  try {
    const response = await client.post('/settings/validate-url', { url })
    setPublicURLValid(response.data.valid)
  } catch {
    setPublicURLValid(false)
  }
}

// Add to saveSettingsMutation
const saveSettingsMutation = useMutation({
  mutationFn: async () => {
    // ... existing saves ...
    await updateSetting('app.public_url', publicURL, 'general', 'string')
  },
  // ...
})
```

**New UI Section** (add after General Configuration card):

```tsx
{/* Application URL */}
<Card>
  <CardHeader>
    <CardTitle>{t('systemSettings.applicationUrl.title')}</CardTitle>
    <CardDescription>{t('systemSettings.applicationUrl.description')}</CardDescription>
  </CardHeader>
  <CardContent className="space-y-4">
    <Alert variant="info">
      <Info className="h-4 w-4" />
      <AlertDescription>
        {t('systemSettings.applicationUrl.infoMessage')}
      </AlertDescription>
    </Alert>

    <div className="space-y-2">
      <Label htmlFor="public-url">{t('systemSettings.applicationUrl.label')}</Label>
      <div className="flex gap-2">
        <Input
          id="public-url"
          type="url"
          value={publicURL}
          onChange={(e) => {
            setPublicURL(e.target.value)
            validatePublicURL(e.target.value)
          }}
          placeholder="https://charon.example.com"
          className={cn(
            publicURLValid === false && 'border-red-500',
            publicURLValid === true && 'border-green-500'
          )}
        />
        {publicURLValid !== null && (
          publicURLValid ? (
            <CheckCircle2 className="h-5 w-5 text-green-500 self-center" />
          ) : (
            <XCircle className="h-5 w-5 text-red-500 self-center" />
          )
        )}
      </div>
      <p className="text-sm text-content-muted">
        {t('systemSettings.applicationUrl.helper')}
      </p>
      {publicURLValid === false && (
        <p className="text-sm text-red-500">
          {t('systemSettings.applicationUrl.invalidUrl')}
        </p>
      )}
    </div>

    {!publicURL && (
      <Alert variant="warning">
        <AlertTriangle className="h-4 w-4" />
        <AlertDescription>
          {t('systemSettings.applicationUrl.notConfiguredWarning')}
        </AlertDescription>
      </Alert>
    )}
  </CardContent>
</Card>
```

### 4.2 User Invitation Flow Updates

**File**: [frontend/src/pages/UsersPage.tsx](../../frontend/src/pages/UsersPage.tsx)

Add URL preview to the `InviteModal` component:

```tsx
// Add to InviteModal component
const [urlPreview, setUrlPreview] = useState<{
  preview_url: string
  base_url: string
  is_configured: boolean
  warning: boolean
  warning_message: string
} | null>(null)

// Add preview fetch when email changes
useEffect(() => {
  if (email && email.includes('@')) {
    const fetchPreview = async () => {
      try {
        const response = await client.post('/users/preview-invite-url', { email })
        setUrlPreview(response.data)
      } catch {
        setUrlPreview(null)
      }
    }
    const debounce = setTimeout(fetchPreview, 500)
    return () => clearTimeout(debounce)
  } else {
    setUrlPreview(null)
  }
}, [email])
```

Add preview UI to modal (before the submit buttons):

```tsx
{/* URL Preview */}
{urlPreview && (
  <div className="space-y-2 p-4 bg-surface-subtle rounded-lg border border-border">
    <div className="flex items-center gap-2">
      <ExternalLink className="h-4 w-4 text-content-muted" />
      <Label className="text-sm font-medium">
        {t('users.inviteUrlPreview')}
      </Label>
    </div>
    <div className="text-sm font-mono text-content-secondary break-all bg-surface-elevated p-2 rounded">
      {urlPreview.preview_url.replace('SAMPLE_TOKEN_PREVIEW', '...')}
    </div>
    {urlPreview.warning && (
      <Alert variant="warning" className="mt-2">
        <AlertTriangle className="h-4 w-4" />
        <AlertDescription className="text-xs">
          {t('users.inviteUrlWarning')}
          <Link to="/settings/system" className="ml-1 underline">
            {t('users.configureApplicationUrl')}
          </Link>
        </AlertDescription>
      </Alert>
    )}
  </div>
)}
```

### 4.3 API Client Updates

**File**: [frontend/src/api/settings.ts](../../frontend/src/api/settings.ts)

Add new function:

```typescript
/**
 * Validates a URL for use as the application URL.
 * @param url - The URL to validate
 * @returns Promise resolving to validation result
 */
export const validatePublicURL = async (url: string): Promise<{
  valid: boolean
  normalized?: string
  error?: string
}> => {
  const response = await client.post('/settings/validate-url', { url })
  return response.data
}
```

**File**: [frontend/src/api/users.ts](../../frontend/src/api/users.ts)

Add new function:

```typescript
/** Response from invite URL preview. */
export interface PreviewInviteURLResponse {
  preview_url: string
  base_url: string
  is_configured: boolean
  email: string
  warning: boolean
  warning_message: string
}

/**
 * Previews what the invite URL will look like for a given email.
 * @param email - The email to preview
 * @returns Promise resolving to PreviewInviteURLResponse
 */
export const previewInviteURL = async (email: string): Promise<PreviewInviteURLResponse> => {
  const response = await client.post<PreviewInviteURLResponse>('/users/preview-invite-url', { email })
  return response.data
}
```

---

## 5. Translation Updates

**File**: [frontend/src/locales/en/translation.json](../../frontend/src/locales/en/translation.json)

Add under `systemSettings`:

```json
"applicationUrl": {
  "title": "Application URL",
  "description": "Configure the public URL used for user-facing links and emails.",
  "label": "Application URL",
  "helper": "The public URL where users access Charon (e.g., https://charon.example.com). Used in invitation emails and password reset links.",
  "infoMessage": "This URL is used when sending invitation emails. If not configured, Charon will use the URL from the current browser request, which may not work for external users.",
  "invalidUrl": "Please enter a valid URL starting with http:// or https://",
  "notConfiguredWarning": "Application URL is not configured. Invitation emails will use the current browser URL, which may not be accessible from external networks."
}
```

Add under `users`:

```json
"inviteUrlPreview": "Invite Link Preview",
"inviteUrlWarning": "Application URL is not configured. This link may not work for external users.",
"configureApplicationUrl": "Configure Application URL"
```

**Repeat for other locale files**: `de/translation.json`, `es/translation.json`, `fr/translation.json`, `zh/translation.json`

---

## 6. Implementation Phases

### Phase 1: Backend Foundation

**Files to modify:**

1. [backend/internal/api/handlers/settings_handler.go](../../backend/internal/api/handlers/settings_handler.go)
   - Add `GetPublicURL()` helper function
   - Add `ValidatePublicURL()` endpoint

2. [backend/internal/api/handlers/user_handler.go](../../backend/internal/api/handlers/user_handler.go)
   - Modify `InviteUser()` to use `GetPublicURL()`
   - Add `PreviewInviteURL()` endpoint

3. [backend/internal/api/routes/routes.go](../../backend/internal/api/routes/routes.go)
   - Register new routes

**Deliverable**: Backend can store, validate, and use the public URL setting.

### Phase 2: Frontend Settings UI

**Files to modify:**

1. [frontend/src/pages/SystemSettings.tsx](../../frontend/src/pages/SystemSettings.tsx)
   - Add Application URL input field
   - Add validation feedback
   - Add warning when not configured

2. [frontend/src/api/settings.ts](../../frontend/src/api/settings.ts)
   - Add `validatePublicURL()` function

3. [frontend/src/locales/en/translation.json](../../frontend/src/locales/en/translation.json)
   - Add `systemSettings.applicationUrl.*` translations

**Deliverable**: Admin can configure the Application URL in settings.

### Phase 3: User Invitation Preview

**Files to modify:**

1. [frontend/src/pages/UsersPage.tsx](../../frontend/src/pages/UsersPage.tsx)
   - Add URL preview to `InviteModal`
   - Add warning banner if URL not configured

2. [frontend/src/api/users.ts](../../frontend/src/api/users.ts)
   - Add `previewInviteURL()` function

3. [frontend/src/locales/en/translation.json](../../frontend/src/locales/en/translation.json)
   - Add `users.inviteUrlPreview`, `users.inviteUrlWarning`, `users.configureApplicationUrl`

**Deliverable**: Admin sees URL preview and warnings when inviting users.

### Phase 4: i18n & Polish

**Files to modify:**

1. All locale files:
   - `frontend/src/locales/de/translation.json`
   - `frontend/src/locales/es/translation.json`
   - `frontend/src/locales/fr/translation.json`
   - `frontend/src/locales/zh/translation.json`

2. Add unit tests for new endpoints

**Deliverable**: Complete feature with all translations.

---

## 7. Security Considerations

### 7.1 URL Validation

- **SSRF Prevention**: The URL is only used for generating email links, not for server-side requests
- **Input Validation**: Backend validates URL format (must start with `http://` or `https://`)
- **XSS Prevention**: URL is HTML-escaped in email templates (already handled by Go's `html/template`)

### 7.2 Authorization

- **Settings Access**: Only admins can modify `app.public_url` (existing middleware)
- **Preview Access**: Only admins can preview invite URLs

### 7.3 Data Exposure

- The preview endpoint only returns sanitized data, no sensitive tokens
- Sample tokens in preview are clearly marked and not stored

---

## 8. Testing Checklist

- [ ] Setting saves and persists across restarts
- [ ] URL validation rejects invalid formats
- [ ] Invite emails use the configured URL
- [ ] Invite emails fall back to request URL if not configured
- [ ] Preview shows warning when URL not configured
- [ ] Preview updates when email changes
- [ ] Accept invite page works with the configured URL
- [ ] All translations display correctly
- [ ] Setting is admin-only

---

## 9. Files Changed Summary

| File | Changes |
|------|---------|
| `backend/internal/api/handlers/settings_handler.go` | Add `GetPublicURL()`, `ValidatePublicURL()` |
| `backend/internal/api/handlers/user_handler.go` | Modify `InviteUser()`, add `PreviewInviteURL()` |
| `backend/internal/api/routes/routes.go` | Register 2 new routes |
| `frontend/src/pages/SystemSettings.tsx` | Add Application URL card |
| `frontend/src/pages/UsersPage.tsx` | Add URL preview to InviteModal |
| `frontend/src/api/settings.ts` | Add `validatePublicURL()` |
| `frontend/src/api/users.ts` | Add `previewInviteURL()` |
| `frontend/src/locales/*/translation.json` | Add new translation keys |

**No changes required to:**

- `backend/internal/models/setting.go` - Uses existing schema
- `backend/internal/services/mail_service.go` - Already accepts URL parameter
- `.gitignore`, `codecov.yml`, `.dockerignore`, `Dockerfile` - No changes needed
