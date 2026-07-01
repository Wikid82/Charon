# Theme System Extensions — Banner Upload + Named User Themes

**Author:** Planning Agent
**Date:** 2026-06-21
**Branch:** feature/theme
**Scope:** Backend + Frontend additions extending the existing theme system

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Research Findings](#2-research-findings)
3. [Technical Specifications](#3-technical-specifications)
4. [Implementation Plan](#4-implementation-plan)
5. [Acceptance Criteria](#5-acceptance-criteria)
6. [Commit Slicing Strategy](#6-commit-slicing-strategy)

---

## 1. Introduction

### 1.1 Overview

This plan extends the existing Charon theme system (fully implemented on `feature/theme`) with two new capabilities:

1. **Banner Image Upload** — a dedicated upload endpoint and UI for the expanded-sidebar banner image, separate from the logo upload already implemented.
2. **Custom User-Created Named Themes** — multi-slot, DB-persisted color-scheme themes that users can create, name, edit, switch between, and delete. This replaces the single `localStorage`-only "Custom" theme slot.

### 1.2 Objectives

- Provide a clean separation between the small collapsed-state logo and the wide expanded-state banner, both customizable independently.
- Persist user-created themes in the database so they survive browser cache clears and are shared across devices logged into the same Charon instance.
- Maintain backward compatibility with existing single-slot custom theme data stored in `localStorage`.
- Follow all existing patterns: DRY, GORM auto-migrate, UUID server-side, `filepath.Clean`, 85%+ coverage, no new npm packages.

### 1.3 Non-Goals

- Do NOT redesign the FOUC fix or the existing built-in theme gallery — those are complete and working.
- Do NOT support SVG uploads (same restriction as logo handler).
- Do NOT break the existing `custom` theme localStorage behavior for users who have not migrated.

---

## 2. Research Findings

### 2.1 Existing Architecture Summary

**Backend:**

- `backend/internal/api/handlers/logo_handler.go` — `LogoHandler` with `UploadLogo` / `DeleteLogo`. Uses `requireAuthenticatedAdmin`, `http.MaxBytesReader`, byte-sniff MIME detection, fixed filename `logo.<ext>`, upserts `ui.logo_url` and `ui.logo_type` into the `Setting` model.
- `backend/internal/models/setting.go` — `Setting` struct with `Key`, `Value`, `Type`, `Category`, `UpdatedAt`. All persistence uses `db.Where(Setting{Key:key}).Assign(s).FirstOrCreate(&s)`.
- `backend/internal/api/routes/routes.go` — `RegisterWithDeps` runs `AutoMigrate` for all models, wires `logoHandler` under `management` group (requires `RequireManagementAccess`), registers `POST /settings/logo` and `DELETE /settings/logo`.
- `backend/internal/server/server.go` — `NewRouter` serves `router.Static("/uploads", dataDir+"/uploads")` for uploaded files.

**Frontend:**

- `frontend/src/context/ThemeContextValue.ts` — `ThemeId = BuiltInTheme | MetaTheme`, `MetaTheme = 'system' | 'custom'`. `CustomTheme = { name, colors }`. Single `CUSTOM_THEME_STORAGE_KEY = 'charon-custom-theme'`.
- `frontend/src/context/ThemeContext.tsx` — `ThemeProvider` with single-slot `customTheme` state. `setCustomTheme(colors, name)` writes to `localStorage` and sets `theme` to `'custom'`.
- `frontend/src/components/Layout.tsx` — reads `settings['ui.logo_url']` as `customLogoUrl`. Sidebar header renders:
  - Collapsed: `<img src={logoSrc} />` (logoSrc = customLogoUrl or `/logo.png`)
  - Expanded with customLogoUrl: `<img src={bannerSrc} />` where `bannerSrc = customLogoUrl` (currently the SAME setting used for both)
  - Expanded without customLogoUrl: `<picture><source srcSet="/banner.webp" /><img src="/banner.png" /></picture>`
- `frontend/src/pages/AppearanceSettings.tsx` — uses `getSettings` query to read `ui.logo_url`, mounts `LogoCustomizer`, `ThemeGallery`, `CustomColorPicker`, `ThemeImportExport`.
- `frontend/src/api/settings.ts` — `uploadLogo(file)` posts multipart to `/settings/logo`, `deleteLogo()` sends DELETE.
- `frontend/index.html` — inline script reads only `localStorage`. Cannot fetch from backend. For `user:*` theme IDs it cannot resolve colors at paint time.

### 2.2 Key Observations

1. **Logo / Banner conflation**: `Layout.tsx` line 74 sets `bannerSrc = customLogoUrl || undefined`. This means uploading a logo currently replaces the banner too. The new banner upload must introduce `ui.banner_url` as a separate setting key and update Layout to read it independently.

2. **Single-slot custom theme**: `ThemeContext.tsx` has one `customTheme` state slot and one `CUSTOM_THEME_STORAGE_KEY`. Named user themes require a collection stored in the DB, accessed via React Query, without removing the existing single-slot code path.

3. **`data-theme` constraint**: The CSS theme system only has `data-theme` values for the static built-ins and `custom`. User-created named themes must reuse `data-theme="custom"` and inject their colors via `style.setProperty` on `<html>`, exactly like the existing custom theme does.

4. **FOUC constraint**: The inline script in `index.html` can only access `localStorage`, not the network. If `localStorage['charon-theme']` is `user:<uuid>`, the script cannot fetch that theme's colors at paint time. It must fall back to `data-theme="dark"` for any `user:*` value. React's hydration will then apply the correct colors after the network fetch completes.

5. **Shared upsert logic**: `LogoHandler.upsertSetting` is private. The new `BannerHandler` will need the same helper. The architectural choice of whether to share this via a dedicated `ImageUploadHandler` or duplicate is addressed in Section 3.1.

### 2.3 Architectural Decision: Shared Image Upload Handler

**Decision: Create `image_upload_handler.go` as a shared handler with a configuration struct.**

**Rationale:**

The `upsertSetting`, `acceptedMIME`, and the full upload pipeline in `logo_handler.go` are identical for any image asset upload. Duplicating them into `banner_handler.go` violates the DRY principle (CLAUDE.md: "Consolidate duplicate patterns into reusable functions after the second occurrence"). A generalized `ImageUploadHandler` parameterized by asset type (`logo`, `banner`) is cleaner and testable once.

The implementation will:
- Define an `AssetConfig` struct carrying `FormField`, `URLSettingKey`, `TypeSettingKey`, `FileBaseName`.
- `ImageUploadHandler` holds `db`, `dataDir`, and `cfg AssetConfig`.
- `UploadAsset(c *gin.Context)` and `DeleteAsset(c *gin.Context)` are the generic methods.
- `LogoHandler` and `BannerHandler` become thin wrappers that instantiate `ImageUploadHandler` with the appropriate config, keeping the existing public constructor signatures and route method names for backward compatibility.

This means:
- `logo_handler.go` is refactored to use `ImageUploadHandler` internally.
- `banner_handler.go` is a new file instantiating `ImageUploadHandler` for the banner asset.
- Tests for both share helper setup functions.

### 2.4 Architectural Decision: Named Themes Storage (Option B — New GORM Model)

**Decision: Option B — New `CustomTheme` GORM model.**

**Rationale:**

Option A (JSON array blob in Setting) has significant drawbacks: no efficient ID-based lookup for `PUT /themes/:id` and `DELETE /themes/:id`, no indexing, size limits for large theme libraries, and GORM's `FirstOrCreate` pattern is awkward for array mutation. Option B provides:
- First-class row identity (`id` UUID) for direct API addressing.
- GORM auto-migrate handles schema creation.
- Efficient per-row updates and deletes.
- Consistent with how all other Charon entities are modeled (users, notifications, uptime monitors).
- No scaling concerns: themes are a handful of rows.

---

## 3. Technical Specifications

### 3.1 Feature 1: Banner Image Upload

#### 3.1.1 Backend — `image_upload_handler.go` (Refactor + Extend)

**File:** `backend/internal/api/handlers/image_upload_handler.go`

This file absorbs the shared upload logic. The existing constants `maxLogoSize`, `mimeSniffBytes`, `logoFilePerm`, `uploadsDirPerm` and function `acceptedMIME` move here. The constant `maxLogoSize` is renamed `maxImageSize` for generality.

**Admin enforcement:** Since both logo and banner uploads are admin-only operations, `requireAuthenticatedAdmin(c)` MUST be called inside `ImageUploadHandler.UploadAsset` and `ImageUploadHandler.DeleteAsset` directly — NOT in the thin `LogoHandler`/`BannerHandler` wrappers. This avoids duplicating the admin check in every thin wrapper and ensures any future image asset type is automatically admin-only. The thin wrappers (`LogoHandler.UploadLogo`, `BannerHandler.UploadBanner`, etc.) do not need to call `requireAuthenticatedAdmin` themselves.

Signatures:

```
// AssetConfig parameterizes a specific image asset type.
type AssetConfig struct {
    FormField      string // multipart field name, e.g. "logo" or "banner"
    URLSettingKey  string // e.g. "ui.logo_url" or "ui.banner_url"
    TypeSettingKey string // e.g. "ui.logo_type" or "ui.banner_type"
    FileBaseName   string // e.g. "logo" or "banner" (extension appended at runtime)
}

// ImageUploadHandler handles generic image asset upload and deletion.
// Both UploadAsset and DeleteAsset enforce requireAuthenticatedAdmin internally.
type ImageUploadHandler struct {
    db      *gorm.DB
    dataDir string
    cfg     AssetConfig
}

func NewImageUploadHandler(db *gorm.DB, dataDir string, cfg AssetConfig) *ImageUploadHandler

// UploadAsset handles POST for any image asset type.
// Calls requireAuthenticatedAdmin(c) at the top — returns 401/403 and aborts if not satisfied.
func (h *ImageUploadHandler) UploadAsset(c *gin.Context)

// DeleteAsset handles DELETE for any image asset type.
// Calls requireAuthenticatedAdmin(c) at the top — returns 401/403 and aborts if not satisfied.
func (h *ImageUploadHandler) DeleteAsset(c *gin.Context)

// upsertSetting creates or updates a Setting row (moved from logo_handler.go).
func (h *ImageUploadHandler) upsertSetting(key, value, category, settingType string) error

// acceptedMIME maps a detected MIME type to file extension (moved from logo_handler.go).
// Returns ("", false) for disallowed types including SVG.
func acceptedMIME(mime string) (string, bool)
```

**File:** `backend/internal/api/handlers/logo_handler.go` (Refactored)

Becomes a thin wrapper. `LogoHandler` delegates to `ImageUploadHandler`:

```
// LogoHandler wraps ImageUploadHandler for the logo asset.
type LogoHandler struct {
    inner *ImageUploadHandler
}

func NewLogoHandler(db *gorm.DB, dataDir string) *LogoHandler

func (h *LogoHandler) UploadLogo(c *gin.Context)  // delegates to h.inner.UploadAsset
func (h *LogoHandler) DeleteLogo(c *gin.Context)  // delegates to h.inner.DeleteAsset
```

Public API and method signatures are unchanged. All existing tests in `logo_handler_test.go` remain valid with zero changes to the test file.

**File:** `backend/internal/api/handlers/banner_handler.go` (New)

```
// BannerHandler wraps ImageUploadHandler for the banner asset.
type BannerHandler struct {
    inner *ImageUploadHandler
}

func NewBannerHandler(db *gorm.DB, dataDir string) *BannerHandler

func (h *BannerHandler) UploadBanner(c *gin.Context)  // delegates to h.inner.UploadAsset
func (h *BannerHandler) DeleteBanner(c *gin.Context)  // delegates to h.inner.DeleteAsset
```

The `AssetConfig` for `BannerHandler`:
- `FormField:      "banner"`
- `URLSettingKey:  "ui.banner_url"`
- `TypeSettingKey: "ui.banner_type"`
- `FileBaseName:   "banner"`

#### 3.1.2 Backend — Route Registration

**File:** `backend/internal/api/routes/routes.go`

Add after the existing logo route wiring (after line 349):

```go
// Banner upload/delete — admin only (enforced inside ImageUploadHandler.UploadAsset / DeleteAsset)
bannerHandler := handlers.NewBannerHandler(db, dataRoot)
management.POST("/settings/banner", bannerHandler.UploadBanner)
management.DELETE("/settings/banner", bannerHandler.DeleteBanner)
```

Note: The admin check (`requireAuthenticatedAdmin`) is enforced inside `ImageUploadHandler.UploadAsset` and `ImageUploadHandler.DeleteAsset` (see Section 3.1.1). `BannerHandler.UploadBanner` and `BannerHandler.DeleteBanner` are pure delegation wrappers and do NOT need to duplicate that check.

No AutoMigrate change needed: banner URL is stored in the existing `Setting` model under `ui.banner_url`.

#### 3.1.3 Backend — API Contract

| Method | Path | Auth | Body | Success Response |
|--------|------|------|------|-----------------|
| `POST` | `/api/v1/settings/banner` | admin | `multipart/form-data` field `banner` (PNG/JPG/WebP, max 2MB) | `200 { "url": "/uploads/banner.png" }` |
| `DELETE` | `/api/v1/settings/banner` | admin | — | `200 { "message": "banner deleted" }` |

Error responses follow the same pattern as logo:
- `400 { "error": "missing banner field" }` — field name wrong or absent
- `400 { "error": "unsupported file type: text/xml" }` — bad MIME
- `413 { "error": "file exceeds 2 MB limit" }` — file too large
- `401` / `403` — auth failures

File is stored as `data/uploads/banner.<ext>`. Setting keys: `ui.banner_url` = `/uploads/banner.png`, `ui.banner_type` = `"upload"`.

#### 3.1.4 Frontend — API Client

**File:** `frontend/src/api/settings.ts` (extend)

Add after `deleteLogo`:

```typescript
/**
 * Uploads a banner image file.
 * Accepted types: image/png, image/jpeg, image/webp (max 2 MB).
 * @param file - The image file to upload
 * @returns Promise resolving to the served URL of the uploaded banner
 */
export const uploadBanner = async (file: File): Promise<{ url: string }> => {
  const form = new FormData()
  form.append('banner', file)
  const response = await client.post('/settings/banner', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return response.data
}

/**
 * Deletes the custom banner, restoring the default banner image.
 */
export const deleteBanner = async (): Promise<void> => {
  await client.delete('/settings/banner')
}
```

#### 3.1.5 Frontend — `BannerCustomizer.tsx` (New Component)

**File:** `frontend/src/components/theme/BannerCustomizer.tsx`

This component is structurally identical to `LogoCustomizer.tsx` but:
- Uses `alt="Banner preview"` on the preview image.
- Uses `id="banner-file-input"` on the file input.
- Props: `currentBannerUrl`, `onUpload`, `onUrlSave`, `onReset`, `isSaving`.
- Preview renders the banner in a wide aspect-ratio container (`max-w-full h-16 object-contain`) to match the sidebar's expanded state.
- Translation keys use `appearance.banner*` prefix (see Section 3.1.8).
- Admin check: shows read-only notice for non-admins, same pattern as `LogoCustomizer`.

**URL tab security requirements:**

The URL tab input for entering a banner URL MUST enforce `https://`-only URLs:

1. The input element MUST have `type="url"` and `pattern="https://.*"` attributes.
2. A helper function `isValidBannerUrl(url: string): boolean` MUST be defined in `BannerCustomizer.tsx`:
   ```typescript
   function isValidBannerUrl(url: string): boolean {
     return url.startsWith('https://')
   }
   ```
3. When the user submits the URL form, client-side validation MUST call `isValidBannerUrl`. If it returns `false`, show an inline error message (e.g., `t('appearance.bannerUrlHttpsRequired')`) and do NOT call `onUrlSave`.
4. Schemes `http://`, `javascript:`, `data:`, and bare paths are all rejected — only `https://` is accepted.

**Important note on server-side URL validation:** The `saveBannerUrlMutation` in `AppearanceSettings.tsx` calls `updateSetting('ui.banner_url', url, ...)` directly. This path does NOT pass through the banner MIME/size enforcement — it is a URL stored as a setting value, not a server-side fetch. The backend settings handler does not validate `ui.*_url` key values for URL scheme. The client-side `https://` guard in `BannerCustomizer.tsx` is the minimum required security control for this PR. A complete server-side fix (validating `ui.*_url` keys in the settings handler to reject non-`https://` schemes) is a future enhancement tracked as a separate issue.

**Analogous requirement for `LogoCustomizer.tsx`:** The existing `LogoCustomizer.tsx` already uses `type="url"` on its URL input, but it does NOT enforce `https://`-only. The same `https://` client-side validation MUST be added to `LogoCustomizer.tsx`:
- Add an `isValidLogoUrl(url: string): boolean` helper (identical logic: `url.startsWith('https://')`)
- Show an inline error if the submitted URL is not `https://`
- The `BannerCustomizer.test.tsx` test suite (Section 4 unit tests) MUST include a test that verifies `http://` URLs are rejected with an inline error. A corresponding note about adding the same test to `LogoCustomizer` tests should be added alongside.

```typescript
export interface BannerCustomizerProps {
  currentBannerUrl: string | null
  onUpload: (file: File) => void
  onUrlSave: (url: string) => void
  onReset: () => void
  isSaving: boolean
}

export function BannerCustomizer({ currentBannerUrl, onUpload, onUrlSave, onReset, isSaving }: BannerCustomizerProps)
```

#### 3.1.6 Frontend — `Layout.tsx` Update

**File:** `frontend/src/components/Layout.tsx`

Current line 74: `const bannerSrc = customLogoUrl || undefined`

Replace lines 72-74 with:

```typescript
const customLogoUrl = settings?.['ui.logo_url'] ?? null
const customBannerUrl = settings?.['ui.banner_url'] ?? null
const logoSrc = customLogoUrl || '/logo.png'
```

Sidebar header conditional (lines 177-186) updated to use `customBannerUrl`:

```tsx
{isCollapsed ? (
  <img src={logoSrc} alt="Charon" className="h-12 w-auto" fetchPriority="high" decoding="async" />
) : customBannerUrl ? (
  <img src={customBannerUrl} alt="Charon" className="h-14 w-auto max-w-[200px] object-contain" fetchPriority="high" decoding="async" />
) : (
  <picture>
    <source srcSet="/banner.webp" type="image/webp" />
    <img src="/banner.png" alt="Charon" className="h-14 w-auto max-w-[200px] object-contain" fetchPriority="high" decoding="async" />
  </picture>
)}
```

This cleanly separates the two customization surfaces. A custom logo only affects the collapsed state; a custom banner only affects the expanded state.

#### 3.1.7 Frontend — `AppearanceSettings.tsx` Update

**File:** `frontend/src/pages/AppearanceSettings.tsx`

Add `BannerCustomizer` section below the existing Logo Customization card. New imports and mutations:

```typescript
import { BannerCustomizer } from '../components/theme/BannerCustomizer'
import { deleteBanner, uploadBanner } from '../api/settings'

// In component body:
const currentBannerUrl = settings?.['ui.banner_url'] ?? null

const uploadBannerMutation = useMutation({
  mutationFn: uploadBanner,
  onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['settings'] }) },
})

// NOTE: saveBannerUrlMutation stores the URL as a plain setting value.
// It does NOT perform a server-side fetch of the URL, so it bypasses MIME/size
// enforcement. Client-side https:// validation in BannerCustomizer.tsx is the
// security boundary for this code path. Server-side url-scheme validation is a
// future enhancement (see Section 3.1.5 security note).
const saveBannerUrlMutation = useMutation({
  mutationFn: async (url: string) => {
    await updateSetting('ui.banner_url', url, 'ui', 'string')
    await updateSetting('ui.banner_type', 'url', 'ui', 'string')
  },
  onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['settings'] }) },
})

const deleteBannerMutation = useMutation({
  mutationFn: deleteBanner,
  onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['settings'] }) },
})

const isSavingBanner =
  uploadBannerMutation.isPending ||
  saveBannerUrlMutation.isPending ||
  deleteBannerMutation.isPending
```

New Card rendered immediately after the existing Logo Customization card:

```tsx
<Card>
  <CardHeader>
    <div className="flex items-center gap-2">
      <ImageIcon className="h-5 w-5 text-content-secondary" />
      <CardTitle>{t('appearance.bannerCustomization')}</CardTitle>
    </div>
    <CardDescription>{t('appearance.bannerCustomizationDescription')}</CardDescription>
  </CardHeader>
  <CardContent>
    <BannerCustomizer
      currentBannerUrl={currentBannerUrl}
      onUpload={(file) => uploadBannerMutation.mutate(file)}
      onUrlSave={(url) => saveBannerUrlMutation.mutate(url)}
      onReset={() => deleteBannerMutation.mutate()}
      isSaving={isSavingBanner}
    />
  </CardContent>
</Card>
```

#### 3.1.8 Translation Keys Required

Add to all locale files. The correct paths are:
- `frontend/src/locales/en/translation.json`
- `frontend/src/locales/de/translation.json`
- `frontend/src/locales/es/translation.json`
- `frontend/src/locales/fr/translation.json`
- `frontend/src/locales/zh/translation.json`

New banner translation keys to add:

```json
"appearance.bannerCustomization": "Sidebar Banner",
"appearance.bannerCustomizationDescription": "Upload a wide banner image shown in the expanded sidebar. Recommended aspect ratio 4:1 or wider.",
"appearance.bannerPreview": "Banner Preview",
"appearance.bannerUploadTab": "Upload File",
"appearance.bannerUrlTab": "Enter URL",
"appearance.bannerUrlPlaceholder": "https://example.com/banner.png",
"appearance.bannerSaveButton": "Save Banner",
"appearance.bannerResetButton": "Reset to Default",
"appearance.bannerUploadHint": "PNG, JPG or WebP — max 2 MB",
"appearance.bannerUrlHttpsRequired": "URL must start with https://"
```

---

### 3.2 Feature 2: Custom User-Created Named Themes

#### 3.2.1 Backend — New `CustomTheme` Model

**File:** `backend/internal/models/custom_theme.go` (New)

```go
package models

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

// CustomTheme stores a user-created named color-scheme theme.
// Colors is stored as a JSON text blob matching the frontend CustomThemeColors type.
type CustomTheme struct {
    ID        string    `json:"id"         gorm:"primaryKey;type:text"`
    Name      string    `json:"name"       gorm:"type:text;not null;uniqueIndex"`
    Colors    string    `json:"colors"     gorm:"type:text;not null"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate generates a UUID if ID is empty.
func (ct *CustomTheme) BeforeCreate(tx *gorm.DB) error {
    if ct.ID == "" {
        ct.ID = uuid.New().String()
    }
    return nil
}
```

**GORM notes:**
- `Colors` is `gorm:"type:text"` — SQLite stores JSON as TEXT. The backend stores the raw JSON string produced by the frontend and returns it verbatim. The backend does not parse or validate the color token structure beyond valid JSON.
- `Name` has `uniqueIndex` to enforce uniqueness at the DB level.
- `ID` is `gorm:"type:text"` (UUID string) consistent with other models using string primary keys.

**File:** `backend/internal/api/routes/routes.go` (AutoMigrate update)

Add `&models.CustomTheme{}` to the `db.AutoMigrate(...)` call list, after `&models.RequestLog{}`.

#### 3.2.2 Backend — `custom_theme_handler.go` (New)

**File:** `backend/internal/api/handlers/custom_theme_handler.go`

```go
package handlers

// CustomThemeHandler handles CRUD for user-created named themes.
type CustomThemeHandler struct {
    db *gorm.DB
}

func NewCustomThemeHandler(db *gorm.DB) *CustomThemeHandler

// ListThemes handles GET /api/v1/themes
// Returns all user-created themes ordered by created_at ASC.
// Always returns a JSON array (never null) — empty array when no themes exist.
func (h *CustomThemeHandler) ListThemes(c *gin.Context)

// CreateTheme handles POST /api/v1/themes
// Body: { "name": string, "colors": string (JSON) }
// Validates: name non-empty, max 100 chars; colors is valid JSON string.
// Returns: 201 with the created CustomTheme record.
func (h *CustomThemeHandler) CreateTheme(c *gin.Context)

// UpdateTheme handles PUT /api/v1/themes/:id
// Body: { "name"?: string, "colors"?: string (JSON) }
// Partial update — name and/or colors can be provided.
// Returns: 200 with the updated record.
func (h *CustomThemeHandler) UpdateTheme(c *gin.Context)

// DeleteTheme handles DELETE /api/v1/themes/:id
// Returns: 200 { "message": "theme deleted" }
func (h *CustomThemeHandler) DeleteTheme(c *gin.Context)
```

**Request binding types (unexported, in handler file):**

```go
type createThemeRequest struct {
    Name   string `json:"name"   binding:"required,max=100"`
    Colors string `json:"colors" binding:"required"`
}

type updateThemeRequest struct {
    Name   *string `json:"name"`
    Colors *string `json:"colors"`
}
```

Note: `Colors` is `string` in the request body (the frontend serializes `CustomThemeColors` to a JSON string before sending). The handler validates it is non-empty and parses as valid JSON with `json.Valid([]byte(req.Colors))`.

**Error handling:**
- `GET` — returns `[]models.CustomTheme{}` (empty slice, serializes to `[]`) on zero rows. Never returns null.
- `POST` — `400` if `binding:"required"` fails or `colors` is not valid JSON; `409` if name already exists (detect UNIQUE constraint error); `201` on success.
- `PUT` — `404` if `:id` not found; `400` on invalid JSON in colors; `400` if `req.Name` is non-nil but empty or exceeds 100 chars; `409` on name collision; `200` on success.
- `DELETE` — `404` if not found; `200` otherwise.

**UNIQUE constraint error detection:** GORM wraps SQLite errors. Use the dual-check pattern that combines GORM's sentinel error with a string fallback for SQLite driver compatibility. Both `CreateTheme` and `UpdateTheme` handlers MUST use this pattern:

```go
if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE constraint failed") {
    c.JSON(http.StatusConflict, gin.H{"error": "a theme with that name already exists"})
    return
}
```

The import list for `custom_theme_handler.go` MUST include `"errors"` and `"gorm.io/gorm"` in addition to the existing imports.

**`UpdateTheme` empty-name validation:** If `req.Name` is non-nil (i.e., the client explicitly sent a `"name"` key), it must pass the same validation as `createThemeRequest`. A provided but empty `req.Name` (client sent `"name": ""`) or a name longer than 100 characters MUST return `400 { "error": "name cannot be empty" }` or `400 { "error": "name exceeds 100 characters" }` respectively. The validation logic:

```go
if req.Name != nil && (len(*req.Name) == 0 || len(*req.Name) > 100) {
    c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
    return
}
```

#### 3.2.3 Backend — Route Registration

**File:** `backend/internal/api/routes/routes.go`

Under the `management` group (after banner routes):

```go
// User-created named themes — available to all management users (not admin-only)
themeHandler := handlers.NewCustomThemeHandler(db)
management.GET("/themes", themeHandler.ListThemes)
management.POST("/themes", themeHandler.CreateTheme)
management.PUT("/themes/:id", themeHandler.UpdateTheme)
management.DELETE("/themes/:id", themeHandler.DeleteTheme)
```

Note: These routes are under `management` (requires `RequireManagementAccess`) but NOT under `securityAdmin` (not admin-only). Any authenticated non-passthrough user can manage their themes.

#### 3.2.4 Backend — API Contract

| Method | Path | Auth | Body | Success |
|--------|------|------|------|---------|
| `GET` | `/api/v1/themes` | management | — | `200 [{ id, name, colors, created_at, updated_at }]` |
| `POST` | `/api/v1/themes` | management | `{ "name": string, "colors": string }` | `201 { id, name, colors, created_at, updated_at }` |
| `PUT` | `/api/v1/themes/:id` | management | `{ "name"?: string, "colors"?: string }` | `200 { id, name, colors, created_at, updated_at }` |
| `DELETE` | `/api/v1/themes/:id` | management | — | `200 { "message": "theme deleted" }` |

The `colors` field in all responses is the raw JSON string stored in the DB. The frontend deserializes it into `CustomThemeColors`.

**Example POST request body:**
```json
{
  "name": "My Dark Theme",
  "colors": "{\"bgBase\":\"15 23 42\",\"bgSubtle\":\"30 41 59\",\"colorScheme\":\"dark\",...}"
}
```

**Example GET response:**
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "My Dark Theme",
    "colors": "{\"bgBase\":\"15 23 42\",...}",
    "created_at": "2026-06-21T10:00:00Z",
    "updated_at": "2026-06-21T10:00:00Z"
  }
]
```

#### 3.2.5 Frontend — Type System Extensions

**File:** `frontend/src/context/ThemeContextValue.ts` (extend)

**Canonical type location:** The `UserTheme` interface (defined below, with `colors: CustomThemeColors`) is the canonical definition and lives exclusively in `ThemeContextValue.ts`. Do NOT declare a second `UserTheme` type anywhere else in the codebase (including `themes.ts`). The `UserThemeDTO` type in `themes.ts` (see Section 3.2.6) is a separate wire-format type with `colors: string` and is intentionally distinct from `UserTheme`.

```typescript
// Branded string for user-created theme IDs stored in localStorage
export type UserThemeId = `user:${string}`

// Type guard — narrows string to UserThemeId
export function isUserThemeId(id: string): id is UserThemeId {
  return id.startsWith('user:')
}

// Full theme identifier — now includes user theme IDs
// Replace the existing: export type ThemeId = BuiltInTheme | MetaTheme
export type ThemeId = BuiltInTheme | MetaTheme | UserThemeId

// A user-created named theme (fetched from and stored in the backend)
export interface UserTheme {
  id: string               // UUID
  name: string
  colors: CustomThemeColors
  created_at: string       // ISO 8601
  updated_at: string
}

// Extend ThemeContextType — add user theme fields
// The existing fields are unchanged; add below the existing importTheme field:
//   userThemes: UserTheme[]
//   activeUserTheme: UserTheme | null
//   setUserTheme: (theme: UserTheme) => void
```

**`DataThemeValue` stays unchanged** — user themes always use `data-theme="custom"` (they inject colors via CSS custom properties, exactly like the existing single-slot custom theme).

**`ThemeExport` version stays at `1`** — named theme export/import is a separate future feature.

`resolveDataTheme` is updated: `isUserThemeId(theme)` resolves to `'custom'`.

#### 3.2.6 Frontend — API Client

**File:** `frontend/src/api/themes.ts` (New)

**Type discipline:** `themes.ts` imports `UserTheme` from `ThemeContextValue.ts` — it does NOT redeclare it. The `UserThemeDTO` interface defined here is a wire-format type only (`colors: string`) and is intentionally separate from the domain `UserTheme` type (`colors: CustomThemeColors`). See Section 3.2.5 for the canonical location rule.

```typescript
import client from './client'
import type { CustomThemeColors, UserTheme } from '../context/ThemeContextValue'

// DTO as returned by the backend (colors is a JSON string, NOT a parsed object)
export interface UserThemeDTO {
  id: string
  name: string
  colors: string   // Raw JSON string — must be parsed before use
  created_at: string
  updated_at: string
}

export interface CreateThemePayload {
  name: string
  colors: CustomThemeColors
}

export interface UpdateThemePayload {
  name?: string
  colors?: CustomThemeColors
}

// Parse a backend DTO into a typed UserTheme.
// NOTE: JSON.parse is intentionally not wrapped in try/catch here.
// React Query's queryFn wrapper will catch any parse error and surface it as a
// query error state. Silent failure (returning a default) would hide data corruption.
export function parseUserThemeDTO(dto: UserThemeDTO): UserTheme {
  return {
    id: dto.id,
    name: dto.name,
    colors: JSON.parse(dto.colors) as CustomThemeColors,
    created_at: dto.created_at,
    updated_at: dto.updated_at,
  }
}

export const listUserThemes = async (): Promise<UserThemeDTO[]> => {
  const response = await client.get('/themes')
  return response.data
}

export const createUserTheme = async (payload: CreateThemePayload): Promise<UserThemeDTO> => {
  const response = await client.post('/themes', {
    name: payload.name,
    colors: JSON.stringify(payload.colors),
  })
  return response.data
}

export const updateUserTheme = async (id: string, payload: UpdateThemePayload): Promise<UserThemeDTO> => {
  const body: Record<string, unknown> = {}
  if (payload.name !== undefined) body.name = payload.name
  if (payload.colors !== undefined) body.colors = JSON.stringify(payload.colors)
  const response = await client.put(`/themes/${id}`, body)
  return response.data
}

export const deleteUserTheme = async (id: string): Promise<void> => {
  await client.delete(`/themes/${id}`)
}
```

#### 3.2.7 Frontend — `useUserThemes` Hook

**File:** `frontend/src/hooks/useUserThemes.ts` (New)

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  listUserThemes,
  createUserTheme,
  updateUserTheme,
  deleteUserTheme,
  parseUserThemeDTO,
} from '../api/themes'
import type { UserTheme, CustomThemeColors } from '../context/ThemeContextValue'

export function useUserThemes() {
  const queryClient = useQueryClient()

  const { data: userThemes = [], isLoading, error } = useQuery({
    queryKey: ['user-themes'],
    queryFn: async (): Promise<UserTheme[]> => {
      const dtos = await listUserThemes()
      return dtos.map(parseUserThemeDTO)
    },
    staleTime: 1000 * 60 * 5,  // 5 minutes
  })

  const createMutation = useMutation({
    mutationFn: createUserTheme,
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['user-themes'] }),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: { name?: string; colors?: CustomThemeColors } }) =>
      updateUserTheme(id, payload),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['user-themes'] }),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteUserTheme,
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['user-themes'] }),
  })

  return {
    userThemes,
    isLoading,
    error,
    createTheme: (name: string, colors: CustomThemeColors) =>
      createMutation.mutateAsync({ name, colors }),
    updateTheme: (id: string, payload: { name?: string; colors?: CustomThemeColors }) =>
      updateMutation.mutateAsync({ id, payload }),
    deleteTheme: (id: string) => deleteMutation.mutateAsync(id),
    isCreating: createMutation.isPending,
    isUpdating: updateMutation.isPending,
    isDeleting: deleteMutation.isPending,
  }
}
```

#### 3.2.8 Frontend — `ThemeContext.tsx` Update

**File:** `frontend/src/context/ThemeContext.tsx`

The `ThemeProvider` is extended to incorporate user themes from the backend.

**Key changes:**

1. `ThemeProvider` calls `useUserThemes()` internally. Since `ThemeProvider` lives inside `QueryClientProvider` (confirmed by `main.tsx` structure), this is valid.

2. `resolveDataTheme` updated: any `UserThemeId` resolves to `'custom'`.

3. New `setUserTheme(theme: UserTheme)` function:
   ```typescript
   const setUserTheme = useCallback((theme: UserTheme) => {
     const id: UserThemeId = `user:${theme.id}`
     setThemeState(id)
     applyCustomTokens(theme.colors)
     document.documentElement.setAttribute('data-theme', 'custom')
     try {
       localStorage.setItem(THEME_STORAGE_KEY, id)
     } catch { /* silently ignore */ }
   }, [])
   ```

4. `activeUserTheme` computed value:
   ```typescript
   const activeUserTheme: UserTheme | null = isUserThemeId(theme)
     ? userThemes.find(t => `user:${t.id}` === theme) ?? null
     : null
   ```

5. Extended `useEffect` that handles `UserThemeId` in the theme change effect. **Critical:** the fallback logic MUST guard on `isLoading` from `useUserThemes()` to avoid a race condition where the fallback fires before the query has resolved, permanently clobbering `localStorage` with `'dark'`. Add `isLoading` to the `useEffect` dependency array.

   ```typescript
   // Only run fallback logic after the query has settled (isLoading = false)
   if (isUserThemeId(theme as string)) {
     if (isLoading) return  // Wait for query to settle before applying fallback
     const ut = userThemes.find(t => `user:${t.id}` === theme)
     if (ut) {
       document.documentElement.setAttribute('data-theme', 'custom')
       applyCustomTokens(ut.colors)
     } else {
       // Theme was deleted or DB unavailable — fall back to dark
       document.documentElement.setAttribute('data-theme', 'dark')
       clearCustomTokens()
       setThemeState('dark')
       try { localStorage.setItem(THEME_STORAGE_KEY, 'dark') } catch { /* ignore */ }
     }
     return
   }
   ```

   Behavior on initial load with a `user:*` theme: the page renders dark (set by the inline script fallback in `index.html`), stays dark while the query loads (`isLoading === true` → early return), then correctly applies the user theme colors after the query resolves — all without permanently clobbering `localStorage` during the loading phase.

6. Context value extended with `userThemes`, `activeUserTheme`, `setUserTheme`.

7. **Backward compatibility**: the existing `setCustomTheme(colors, name)` is unchanged. Users on `theme === 'custom'` (without `user:` prefix) continue to work exactly as before. The `customTheme` state and `CUSTOM_THEME_STORAGE_KEY` are untouched.

#### 3.2.9 Frontend — `index.html` Inline Script Update

**File:** `frontend/index.html`

The existing inline FOUC-fix script must handle `user:*` theme IDs by falling back to `dark`. The updated script MUST preserve the existing legacy `'theme'` key fallback chain — this is the key-reading logic and is NOT changed:

```javascript
var k = 'charon-theme';
var t = localStorage.getItem(k) || localStorage.getItem('theme') || 'dark';
```

The `user:*` branch is added ONLY to the theme resolution logic (`var r = ...`), NOT to the key-reading logic. The complete updated resolution block:

```javascript
var r;
if (t === 'system') {
  r = window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
} else if (t === 'custom') {
  r = 'custom';
} else if (t.indexOf('user:') === 0) {
  // Cannot fetch user theme colors at paint time — fall back to dark.
  // React will apply the correct colors after hydration.
  r = 'dark';
} else {
  r = t; // built-in theme: 'dark', 'light', 'solarized', etc.
}
```

**Legacy key note:** If the legacy `'theme'` key (the fallback in `localStorage.getItem('theme')`) happens to contain a `user:*` value (an edge case that is near-impossible in practice but theoretically possible if a user manually set it), the new `user:*` branch correctly falls back to `'dark'` — which is the desired behavior.

The custom token application block (when `r === 'custom'`) is only reached when the stored theme is exactly `'custom'`, not for `user:*` IDs (since those now resolve to `'dark'` in the script). The minified version of the complete script replaces the existing inline script in `<head>`.

For `user:*` themes, the page initially renders in dark mode. React corrects this within one render cycle after `useUserThemes` query resolves. This is an acceptable transient state (styled with dark mode, not unstyled).

#### 3.2.10 Frontend — `UserThemeManager.tsx` (New Component)

**File:** `frontend/src/components/theme/UserThemeManager.tsx`

```typescript
export interface UserThemeManagerProps {
  activeThemeId: ThemeId
  onActivate: (theme: UserTheme) => void
}

export function UserThemeManager({ activeThemeId, onActivate }: UserThemeManagerProps)
```

**UI layout description:**

The component renders:
1. A header row with the section label and a "+ Create New Theme" button.
2. A grid of user theme cards (same grid class as `ThemeGallery`: `grid grid-cols-2 gap-3 sm:grid-cols-3`).
3. An empty state message when no themes exist: `t('appearance.noUserThemes')`.
4. A "Create New Theme" dialog/modal (controlled by `createDialogOpen` state).
5. An "Edit Theme" dialog/modal (controlled by `editingThemeId` state).
6. A delete confirmation (controlled by `confirmDeleteId` state).

Each user theme card shows:
- Theme name (truncated with `truncate` class if long)
- A color swatch bar: 5 `<span>` elements with inline `background: rgb(${color})` for `bgBase`, `bgSubtle`, `brandPrimary`, `textPrimary`, `borderDefault`
- An "Activate" button (or active checkmark if `activeThemeId === 'user:' + theme.id`)
- An "Edit" icon button
- A "Delete" icon button

The "Create New Theme" and "Edit Theme" dialogs contain:
- An `<input type="text">` for the theme name (labeled `t('appearance.themeNameLabel')`)
- A `<CustomColorPicker>` component for color selection
- Pre-filled with `DARK_THEME_DEFAULTS` for new themes; pre-filled with existing colors for edit

The component uses `useUserThemes()` hook internally for all mutations.

**Accessibility:**
- Dialog uses `role="dialog"` with `aria-modal="true"` and `aria-labelledby`.
- Delete confirm uses a `role="alertdialog"`.
- All interactive elements have descriptive `aria-label` attributes.

#### 3.2.11 Frontend — `AppearanceSettings.tsx` Update

**File:** `frontend/src/pages/AppearanceSettings.tsx`

Additions:
- Import `UserThemeManager` from `'../components/theme/UserThemeManager'`
- Destructure `setUserTheme` from `useTheme()`
- Add a new "Your Themes" Card section between the Theme Gallery card and the Custom Theme (color picker) card

```tsx
{/* Your Themes Section */}
<Card>
  <CardHeader>
    <div className="flex items-center gap-2">
      <Palette className="h-5 w-5 text-content-secondary" />
      <CardTitle>{t('appearance.userThemes')}</CardTitle>
    </div>
    <CardDescription>{t('appearance.userThemesDescription')}</CardDescription>
  </CardHeader>
  <CardContent>
    <UserThemeManager
      activeThemeId={theme}
      onActivate={(userTheme) => {
        setPreviewTheme(null)
        setUserTheme(userTheme)
      }}
    />
  </CardContent>
</Card>
```

The existing "Custom Theme" card (single-slot color picker) is already conditionally rendered only when `theme === 'custom'`. User themes use `theme === 'user:<uuid>'`, so the old picker card is hidden when a user theme is active — no change needed to that condition.

#### 3.2.12 Translation Keys Required

```json
"appearance.userThemes": "Your Themes",
"appearance.userThemesDescription": "Create and save named color themes that persist across devices.",
"appearance.createNewTheme": "Create New Theme",
"appearance.saveTheme": "Save Theme",
"appearance.editTheme": "Edit Theme",
"appearance.deleteTheme": "Delete Theme",
"appearance.confirmDeleteTheme": "Delete this theme? This cannot be undone.",
"appearance.themeNameLabel": "Theme Name",
"appearance.themeNamePlaceholder": "e.g. My Dark Theme",
"appearance.noUserThemes": "No saved themes yet. Create one to get started.",
"appearance.activateTheme": "Activate"
```

#### 3.2.13 localStorage / Persistence Contract

| Scenario | `localStorage['charon-theme']` | Behavior |
|---|---|---|
| Built-in theme active | `"dark"` / `"light"` / etc. | Existing behavior — unchanged |
| Single-slot custom theme active | `"custom"` | Existing behavior — unchanged |
| User-created theme active | `"user:550e8400-..."` | React fetches user themes, applies matching theme's colors |
| User theme deleted after activation | `"user:550e8400-..."` but DB row gone | ThemeContext falls back to `dark`, updates localStorage to `"dark"` |
| Fresh browser (no localStorage) | absent | Falls back to `"dark"` — existing behavior |
| `user:*` in localStorage on page paint | script sets `data-theme="dark"` | React corrects after hydration (first render after query resolves) |

---

## 4. Implementation Plan

### Phase 1: Playwright E2E Tests (Spec Behavior First)

Write failing E2E tests in `tests/theme-extensions.spec.ts` that define the expected behavior before implementation.

**Banner tests:**
- `banner-customization-section-visible` — navigate to `/settings/appearance`, assert "Sidebar Banner" section card is present
- `banner-upload-applies` — upload a valid PNG to banner, assert expanded sidebar `<img>` src changes to `/uploads/banner.png`
- `banner-delete-restores-default` — after banner upload and delete, assert sidebar returns to `<picture>` / `<img src="/banner.png">`
- `banner-logo-independent` — upload both logo and banner, assert logo only affects collapsed state and banner only affects expanded state

**Named theme tests:**
- `user-themes-section-visible` — navigate to `/settings/appearance`, assert "Your Themes" card is present with "+ Create New Theme" button
- `user-theme-create` — click "Create New Theme", enter name "Test Theme", adjust colors, click "Save Theme", assert card appears in list
- `user-theme-activate` — click "Activate" on a user theme card, assert `data-theme="custom"` on `<html>` and at least one CSS custom property is set via `getComputedStyle`
- `user-theme-rename` — click Edit on a user theme, change name, save, assert new name visible
- `user-theme-delete` — click Delete, confirm, assert card no longer in list
- `user-theme-persists-after-reload` — activate user theme, reload page, assert `data-theme="custom"` after hydration
- `user-theme-fallback-when-deleted` — set `localStorage['charon-theme']` to `user:nonexistent-uuid`, reload, assert `data-theme="dark"`

### Phase 2: Backend Implementation (TDD)

Order (each step followed by `go test ./...`):

1. Write `image_upload_handler.go` with `AssetConfig`, `ImageUploadHandler`, shared logic moved from `logo_handler.go`.
2. Refactor `logo_handler.go` to delegate to `ImageUploadHandler`. Run `logo_handler_test.go` — all must pass unchanged.
3. Write `banner_handler.go` thin wrapper.
4. Write `banner_handler_test.go` — full test suite.
5. Write `custom_theme.go` model.
6. Write `custom_theme_test.go` — model-level tests (UUID generation in `BeforeCreate`, field constraints).
7. Write `custom_theme_handler.go` CRUD handler.
8. Write `custom_theme_handler_test.go` — handler tests for all four endpoints.
9. Update `routes.go` — banner routes, theme CRUD routes, `CustomTheme` in AutoMigrate.
10. Run `./scripts/scan-gorm-security.sh --check` — zero CRITICAL/HIGH.

### Phase 3: Frontend Implementation

Order (each step followed by `npm run type-check`):

1. `frontend/src/api/themes.ts` — API client, `parseUserThemeDTO`, types.
2. `frontend/src/hooks/useUserThemes.ts` — React Query hook.
3. `frontend/src/context/ThemeContextValue.ts` — `UserThemeId`, `isUserThemeId`, `UserTheme`, extend `ThemeContextType`.
4. `frontend/src/context/ThemeContext.tsx` — integrate `useUserThemes`, `setUserTheme`, `activeUserTheme`.
5. `frontend/index.html` — update inline FOUC script for `user:*` fallback.
6. `frontend/src/api/settings.ts` — add `uploadBanner`, `deleteBanner`.
7. `frontend/src/components/theme/BannerCustomizer.tsx` — new component.
8. `frontend/src/components/theme/UserThemeManager.tsx` — new component.
9. `frontend/src/components/Layout.tsx` — split `customBannerUrl` from `customLogoUrl`.
10. `frontend/src/pages/AppearanceSettings.tsx` — add banner and user themes sections.
11. Add translation keys to locale files.

### Phase 4: Unit Tests

**Backend tests** (in `backend/internal/api/handlers/` package):
- `banner_handler_test.go` — mirrors `logo_handler_test.go` pattern: valid upload, file too large, SVG rejected, spoofed Content-Type, no field, DELETE clears file and settings, unauthenticated → 401, non-admin → 403.
- `custom_theme_handler_test.go` — GET returns empty array, POST creates with UUID, POST duplicate name → 409, POST empty name → 400, PUT updates, PUT non-existent → 404, DELETE removes, DELETE non-existent → 404, unauthenticated → 401.
- `custom_theme_test.go` (model) — `BeforeCreate` sets UUID when ID is empty, does not overwrite existing ID.

**Frontend tests** (Vitest + Testing Library):
- `frontend/src/components/theme/__tests__/BannerCustomizer.test.tsx` — mirrors `LogoCustomizer.test.tsx`: admin sees file input, non-admin sees notice, file too large shows error, wrong MIME shows error, valid file calls `onUpload`, URL tab works, reset button calls `onReset`.
- `frontend/src/components/theme/__tests__/UserThemeManager.test.tsx` — empty state shows message, themes list renders cards, activate calls `onActivate`, edit dialog opens with existing values, delete confirm dialog appears, create dialog opens with defaults.
- `frontend/src/hooks/__tests__/useUserThemes.test.ts` — mocks `listUserThemes`, verifies `parseUserThemeDTO` parses colors, verifies mutation invalidates `['user-themes']` query.
- `frontend/src/api/__tests__/themes.test.ts` — mocks `client`, verifies `colors` is JSON-stringified in POST body, verifies `parseUserThemeDTO` parses colors from string to object.

### Phase 5: Integration and DoD

In order:
1. `npx playwright test --project=firefox` (tests from `tests/theme-extensions.spec.ts`)
2. `./scripts/scan-gorm-security.sh --check` (zero CRITICAL/HIGH)
3. `bash scripts/local-patch-report.sh` (produces `test-results/local-patch-report.md`)
4. `lefthook run pre-commit`
5. `make lint-fast`
6. `scripts/go-test-coverage.sh` (≥85%)
7. `scripts/frontend-test-coverage.sh` (≥85%)
8. `cd frontend && npm run type-check`
9. `cd backend && go build ./...`
10. `cd frontend && npm run build`

---

## 5. Acceptance Criteria

### Feature 1: Banner Image Upload

| ID | Criterion |
|----|-----------|
| BN-01 | `POST /api/v1/settings/banner` with valid PNG returns `200 { "url": "/uploads/banner.png" }` and writes file to `data/uploads/banner.png` |
| BN-02 | `POST /api/v1/settings/banner` with valid WebP returns `200 { "url": "/uploads/banner.webp" }` |
| BN-03 | `POST /api/v1/settings/banner` with file > 2MB returns `413` |
| BN-04 | `POST /api/v1/settings/banner` with SVG bytes returns `400` (byte-sniff detection) |
| BN-05 | `POST /api/v1/settings/banner` with spoofed Content-Type (SVG bytes, `image/png` header) returns `400` |
| BN-06 | `POST /api/v1/settings/banner` with missing `banner` field returns `400` |
| BN-07 | `DELETE /api/v1/settings/banner` removes the uploaded file and clears `ui.banner_url` and `ui.banner_type` settings |
| BN-08 | Unauthenticated `POST /api/v1/settings/banner` returns `401` |
| BN-09 | Non-admin `POST /api/v1/settings/banner` returns `403` |
| BN-10 | After banner upload, the expanded sidebar renders `<img src="/uploads/banner.png">` |
| BN-11 | After banner delete, the expanded sidebar returns to the default `<picture>` element |
| BN-12 | Logo and banner settings are independent: uploading a banner does not change `ui.logo_url`; uploading a logo does not change `ui.banner_url` |
| BN-13 | `BannerCustomizer` renders a file input with `id="banner-file-input"` for admin users |
| BN-14 | `BannerCustomizer` shows a read-only notice for non-admin users, no file input rendered |
| BN-15 | All `BannerCustomizer` unit tests pass |
| BN-16 | `banner_handler_test.go` achieves ≥85% coverage of the banner handler code paths |

### Feature 2: Custom User-Created Named Themes

| ID | Criterion |
|----|-----------|
| UT-01 | `GET /api/v1/themes` returns `[]` (not null) when no user themes exist |
| UT-02 | `POST /api/v1/themes` with valid `{ name, colors }` returns `201` with a non-empty UUID `id` and persists to DB |
| UT-03 | `POST /api/v1/themes` with a duplicate name returns `409` |
| UT-04 | `POST /api/v1/themes` with empty name returns `400` |
| UT-05 | `POST /api/v1/themes` with invalid JSON in `colors` returns `400` |
| UT-06 | `PUT /api/v1/themes/:id` updates name and/or colors, returns `200` with updated record |
| UT-07 | `PUT /api/v1/themes/:nonexistent-id` returns `404` |
| UT-08 | `DELETE /api/v1/themes/:id` removes the DB row, returns `200 { "message": "theme deleted" }` |
| UT-09 | `DELETE /api/v1/themes/:nonexistent-id` returns `404` |
| UT-10 | Unauthenticated calls to all `/api/v1/themes` endpoints return `401` |
| UT-11 | `useUserThemes` hook fetches themes from backend, calls `parseUserThemeDTO`, returns typed `UserTheme[]` with parsed colors |
| UT-12 | Creating a theme via `UserThemeManager` "Create New Theme" dialog saves to backend and the new card appears in the grid |
| UT-13 | Activating a user theme sets `data-theme="custom"` on `<html>` and applies the theme's colors as CSS custom properties |
| UT-14 | `localStorage['charon-theme']` is set to `user:<uuid>` when a user theme is activated |
| UT-15 | Reloading with `user:<uuid>` in localStorage restores `data-theme="custom"` and correct colors after React hydration |
| UT-16 | Reloading with `user:nonexistent-uuid` in localStorage results in `data-theme="dark"` after hydration |
| UT-17 | The existing single-slot `custom` theme behavior (localStorage only, `setCustomTheme`) is unchanged — no regression |
| UT-18 | Renaming a user theme via the Edit dialog updates the card label in `UserThemeManager` |
| UT-19 | Deleting the active user theme causes the theme to fall back to `dark` |
| UT-20 | `UserThemeManager` renders `t('appearance.noUserThemes')` when no user themes exist |
| UT-21 | GORM security scan reports zero CRITICAL/HIGH findings for `CustomTheme` model and handler queries |
| UT-22 | `custom_theme_handler_test.go` achieves ≥85% coverage |
| UT-23 | Frontend unit tests for `UserThemeManager`, `useUserThemes`, and `themes.ts` all pass |
| UT-24 | The inline `index.html` script sets `data-theme="dark"` (not `"custom"`) when `localStorage['charon-theme']` is a `user:*` ID |

---

## 6. Commit Slicing Strategy

**Decision: Single PR (`feature/theme`), three ordered logical commits.**

Each commit is self-consistent: the backend compiles and passes tests before the frontend is wired up; the frontend compiles (with feature-flagged hooks that return empty state) before full integration is tested.

---

### Commit 1: Backend — Shared Image Handler Refactor + Banner + Named Themes CRUD

**Scope:** Backend only. Zero frontend changes. All existing logo tests continue to pass unchanged.

**Files changed:**

| File | Change |
|------|--------|
| `backend/internal/api/handlers/image_upload_handler.go` | NEW — shared `ImageUploadHandler`, `AssetConfig`, `acceptedMIME`, `upsertSetting`, constants |
| `backend/internal/api/handlers/logo_handler.go` | REFACTORED — delegates to `ImageUploadHandler`; public API unchanged |
| `backend/internal/api/handlers/banner_handler.go` | NEW — `BannerHandler` thin wrapper |
| `backend/internal/api/handlers/banner_handler_test.go` | NEW — full test suite |
| `backend/internal/api/handlers/logo_handler_test.go` | NO CHANGE — must remain green |
| `backend/internal/models/custom_theme.go` | NEW — `CustomTheme` model with `BeforeCreate` UUID hook |
| `backend/internal/models/custom_theme_test.go` | NEW — model unit tests |
| `backend/internal/api/handlers/custom_theme_handler.go` | NEW — CRUD handler |
| `backend/internal/api/handlers/custom_theme_handler_test.go` | NEW — handler tests |
| `backend/internal/api/routes/routes.go` | EXTENDED — banner routes, theme CRUD routes, `CustomTheme` in AutoMigrate |

**Dependencies:** None (first commit).

**Validation gates:**
- `cd backend && go build ./...` — must succeed
- `go test ./...` — all tests pass including new handler and model tests
- `make lint-fast` — zero errors
- `./scripts/scan-gorm-security.sh --check` — zero CRITICAL/HIGH

**Commit message:** `feat(theme): add banner upload endpoint and named themes backend CRUD`

---

### Commit 2: Frontend — Banner Customizer + Named Theme Manager

**Scope:** Frontend only. Requires Commit 1 deployed for full E2E behavior, but compiles independently (hooks return empty state when backend is unavailable).

**Files changed:**

| File | Change |
|------|--------|
| `frontend/src/api/themes.ts` | NEW — themes API client with `parseUserThemeDTO` |
| `frontend/src/hooks/useUserThemes.ts` | NEW — React Query hook |
| `frontend/src/context/ThemeContextValue.ts` | EXTENDED — `UserThemeId`, `isUserThemeId`, `UserTheme`, `ThemeContextType` additions |
| `frontend/src/context/ThemeContext.tsx` | EXTENDED — `setUserTheme`, `activeUserTheme`, `userThemes` in context value |
| `frontend/index.html` | UPDATED — inline script `user:*` fallback to `dark` |
| `frontend/src/api/settings.ts` | EXTENDED — `uploadBanner`, `deleteBanner` |
| `frontend/src/components/theme/BannerCustomizer.tsx` | NEW |
| `frontend/src/components/theme/UserThemeManager.tsx` | NEW |
| `frontend/src/components/Layout.tsx` | UPDATED — `customBannerUrl` read from `ui.banner_url` |
| `frontend/src/pages/AppearanceSettings.tsx` | EXTENDED — banner card, user themes card |
| `frontend/src/locales/en/translation.json` | EXTENDED — new banner and user theme keys |
| `frontend/src/locales/de/translation.json` | EXTENDED — new banner and user theme keys |
| `frontend/src/locales/es/translation.json` | EXTENDED — new banner and user theme keys |
| `frontend/src/locales/fr/translation.json` | EXTENDED — new banner and user theme keys |
| `frontend/src/locales/zh/translation.json` | EXTENDED — new banner and user theme keys |
| `frontend/src/components/theme/__tests__/BannerCustomizer.test.tsx` | NEW |
| `frontend/src/components/theme/__tests__/UserThemeManager.test.tsx` | NEW |
| `frontend/src/hooks/__tests__/useUserThemes.test.ts` | NEW |
| `frontend/src/api/__tests__/themes.test.ts` | NEW |

**Dependencies:** Commit 1 (backend endpoints must exist for React Query to succeed; frontend compiles without it).

**Validation gates:**
- `cd frontend && npm test -- --run` — all unit tests pass (run FIRST, before coverage check)
- `cd frontend && npm run type-check` — zero type errors
- `cd frontend && npm run build` — succeeds
- `scripts/frontend-test-coverage.sh` — ≥85% coverage, all tests pass

**Commit message:** `feat(theme): add banner customizer and named user themes frontend`

---

### Commit 3: E2E Tests + Docs Update

**Scope:** Playwright tests + documentation only. No production code changes.

**Files changed:**

| File | Change |
|------|--------|
| `tests/theme-extensions.spec.ts` | NEW — E2E tests for banner upload and named themes (per Phase 1 list) |
| `ARCHITECTURE.md` | UPDATED — note `CustomTheme` model, `/api/v1/themes` routes, `/api/v1/settings/banner` routes, `UserThemeManager` component |
| `docs/features.md` | UPDATED — one-line mention of banner image upload and named themes |

**Dependencies:** Commits 1 and 2 (tests require both backend and frontend to be implemented).

**Validation gates:**
- `npx playwright test --project=firefox tests/theme-extensions.spec.ts` — all new tests pass
- `npx playwright test --project=firefox tests/theme.spec.ts` — existing theme tests still pass (no regression)

**Commit message:** `test(theme): add E2E tests for banner upload and named user themes`

---

### Rollback and Contingency

**If Commit 1 introduces a regression in logo upload:**
The refactored `LogoHandler` is a pure delegation layer. The `UploadLogo` and `DeleteLogo` public method signatures are unchanged. The test file `logo_handler_test.go` is not modified and must remain green. If `image_upload_handler.go` has a defect, it is isolated — revert only that file and restore the original private methods in `logo_handler.go`.

**If `CustomTheme` AutoMigrate causes a startup failure:**
SQLite `AutoMigrate` is additive-only — it creates new tables, never drops columns or tables. A new `custom_themes` table cannot break existing functionality. If table creation fails (e.g., disk full), the application logs the error but continues. `/api/v1/themes` endpoints return `500`; all other endpoints are unaffected.

**If `user:*` theme IDs cause visible flash on page load:**
The fallback (`data-theme="dark"`) renders the page fully styled in dark mode, not unstyled. This is acceptable. The React correction happens within one render cycle after the `useUserThemes` query resolves (typically under 100ms on a local instance). If this flash is deemed unacceptable in a future iteration, a `localStorage['charon-user-theme-colors']` key can be introduced for the inline script to apply colors synchronously — this is explicitly out of scope for this PR.

**If a user theme is deleted while another browser session has it active:**
On the next navigation or page load, `ThemeContext` detects `activeUserTheme === null` (theme ID not found in fetched list) and falls back to `dark`, clearing the stale `localStorage` key. No error is shown to the user.

---

## Appendix A: File Change Summary

| File | Change Type | Feature |
|------|------------|---------|
| `backend/internal/api/handlers/image_upload_handler.go` | NEW | Banner (shared) |
| `backend/internal/api/handlers/logo_handler.go` | REFACTOR | Banner (shared) |
| `backend/internal/api/handlers/banner_handler.go` | NEW | Banner |
| `backend/internal/api/handlers/banner_handler_test.go` | NEW | Banner |
| `backend/internal/models/custom_theme.go` | NEW | Named Themes |
| `backend/internal/models/custom_theme_test.go` | NEW | Named Themes |
| `backend/internal/api/handlers/custom_theme_handler.go` | NEW | Named Themes |
| `backend/internal/api/handlers/custom_theme_handler_test.go` | NEW | Named Themes |
| `backend/internal/api/routes/routes.go` | EXTEND | Both |
| `frontend/src/api/settings.ts` | EXTEND | Banner |
| `frontend/src/api/themes.ts` | NEW | Named Themes |
| `frontend/src/hooks/useUserThemes.ts` | NEW | Named Themes |
| `frontend/src/context/ThemeContextValue.ts` | EXTEND | Named Themes |
| `frontend/src/context/ThemeContext.tsx` | EXTEND | Named Themes |
| `frontend/index.html` | UPDATE | Named Themes |
| `frontend/src/components/theme/BannerCustomizer.tsx` | NEW | Banner |
| `frontend/src/components/theme/UserThemeManager.tsx` | NEW | Named Themes |
| `frontend/src/components/Layout.tsx` | UPDATE | Banner |
| `frontend/src/pages/AppearanceSettings.tsx` | EXTEND | Both |
| `frontend/src/locales/en/translation.json` | EXTEND | Both |
| `frontend/src/locales/de/translation.json` | EXTEND | Both |
| `frontend/src/locales/es/translation.json` | EXTEND | Both |
| `frontend/src/locales/fr/translation.json` | EXTEND | Both |
| `frontend/src/locales/zh/translation.json` | EXTEND | Both |
| `frontend/src/components/theme/__tests__/BannerCustomizer.test.tsx` | NEW | Banner |
| `frontend/src/components/theme/__tests__/UserThemeManager.test.tsx` | NEW | Named Themes |
| `frontend/src/hooks/__tests__/useUserThemes.test.ts` | NEW | Named Themes |
| `frontend/src/api/__tests__/themes.test.ts` | NEW | Named Themes |
| `tests/theme-extensions.spec.ts` | NEW | Both |
| `ARCHITECTURE.md` | UPDATE | Both |
| `docs/features.md` | UPDATE | Both |

---

## Appendix B: GORM Security Notes

The `CustomTheme` model uses only `db.Find`, `db.Create`, `db.Save`, and `db.Delete` with primary key or unique index lookups. No raw SQL is constructed. All user-controlled values (`name`, `colors`) are bound parameters via GORM's safe query API — they are never interpolated into query strings. The `colors` field stores a JSON string; no dynamic query construction occurs on any field value.

The GORM security scan (`./scripts/scan-gorm-security.sh --check`) must be run and pass before the PR is merged.

---

## Appendix C: `.gitignore` / `.dockerignore` Impact

The banner file is stored in `data/uploads/banner.<ext>`. The `data/` directory is already excluded from git (`.gitignore` line `140: /data/`) and from the Docker build context (`.dockerignore` line `96: data/`). No new ignore rules are needed for the banner feature.

All new source files (`frontend/src/api/themes.ts`, `frontend/src/hooks/useUserThemes.ts`, `backend/internal/models/custom_theme.go`, etc.) are committed source code — no changes to ignore files required.
