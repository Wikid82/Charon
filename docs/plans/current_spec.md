# Theme System Redo — FOUC Fix + Issue #34 Theme Customization

**Author:** Planning Agent
**Date:** 2026-06-20
**Branch:** development
**Scope:** Frontend-primary; backend addition for logo upload only

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

This specification covers two goals shipped in a single PR:

**Goal 1 — Bug Fix (FOUC):** The browser console emits the following warning on every page load in Firefox:

```
Layout was forced before the page was fully loaded. If stylesheets are not yet loaded
this may cause a flash of unstyled content.
```

**Goal 2 — Feature (GitHub Issue #34):** Implement a complete, first-class theme system with pre-built themes, a custom color picker, import/export, logo customization, and a "Follow System" mode.

### 1.2 Root Cause of the FOUC Warning

Two mechanisms compete to apply the theme class on `<html>`:

1. The inline `<script>` in `frontend/index.html` (line 5) runs _before_ any stylesheet `<link>` has been declared. Firefox emits the forced-layout warning because calling `classList.add('dark')` triggers a style recalculation against a document with no applied stylesheets.

2. `ThemeProvider` (`frontend/src/context/ThemeContext.tsx`) uses `useLayoutEffect` which redundantly re-applies the same class on every initial mount, blocking paint.

### 1.3 Technical Approach (Pre-Approved)

**FOUC fix:**
- Move the inline `<script>` to the last position in `<head>`, after Vite's injected `<link rel="stylesheet">` tags.
- Change the script to set `data-theme` attribute (not a class) on `<html>` — this aligns with the new theming foundation.
- Replace `useLayoutEffect` with `useEffect` + a `useRef` first-render skip in `ThemeProvider` so the initial DOM write is owned solely by the inline script.

**Theme system foundation:**
- Migrate from class-based theming (`class="dark"`, `class="light"`) to `data-theme` attribute on `<html>` (`data-theme="dark"`, `data-theme="light"`, etc.).
- CSS custom properties scoped to `[data-theme="dark"]`, `[data-theme="light"]`, etc. in `frontend/src/index.css`.
- Tailwind's `darkMode` config changed from `'class'` to `['selector', '[data-theme="dark"]']`.
- Theme data stored in `localStorage` under the key `charon-theme`.
- Logo customization stored in the backend `Setting` model under key `ui.logo_url` (URL) or served from an uploaded file at `/api/v1/settings/logo`.

### 1.4 Goals

1. Eliminate the FOUC console warning entirely.
2. Pre-built theme gallery: `dark`, `light`, `high-contrast-dark`, `high-contrast-light`, `solarized`.
3. Custom color picker for user-defined themes.
4. Theme import/export (JSON format).
5. Logo customization (upload file or enter URL).
6. "Follow System" option (respects `prefers-color-scheme`).
7. Theme preview before applying.
8. Themes change the entire UI — all components use CSS custom properties.
9. Minimum 85% test coverage for all new and modified code.

---

## 2. Research Findings

### 2.1 Current Theme Architecture

**`frontend/index.html` (line 5):**

The inline IIFE runs synchronously during HTML parsing, before any stylesheet link exists in the DOM. It applies a CSS class (`dark` or `light`) to `<html>`. This triggers the FOUC warning in Firefox.

```html
<script>!function(){try{var t=localStorage.getItem('theme');document.documentElement.classList.add(t==='light'?'light':'dark')}catch(e){document.documentElement.classList.add('dark')}}();</script>
```

Key problems:
- Position: line 5 in `<head>`, before the Vite-injected stylesheet link.
- Uses `classList.add` (class-based) — the new system will use `data-theme` attribute.
- Storage key is `'theme'`.

**`frontend/src/context/ThemeContextValue.ts`:**

```typescript
export type Theme = 'dark' | 'light'

export interface ThemeContextType {
  theme: Theme
  toggleTheme: () => void
}
```

Current `Theme` type is a union of two string literals with only a binary toggle.

**`frontend/src/context/ThemeContext.tsx`:**

- Uses `useLayoutEffect` — wrong hook for a write-only DOM operation.
- Re-applies class on initial mount, duplicating the inline script.
- Has no `useRef` guard, so the initial mount always triggers a DOM write.

**`frontend/src/hooks/useTheme.ts`:**

Simple context consumer that throws if used outside `ThemeProvider`.

**`frontend/src/hooks/__tests__/useTheme.test.tsx`:**

Only tests the error-guard path. No happy-path coverage.

**`frontend/src/context/__tests__/`:**

Only `AuthContext.test.tsx` exists. No `ThemeContext` tests.

**`frontend/src/components/ThemeToggle.tsx`:**

Binary toggle using emoji icons (☀️ / 🌙), calls `useTheme().toggleTheme()`. Used in `Layout.tsx` desktop header and mobile header.

**`frontend/src/pages/Settings.tsx`:**

Tab-based layout with sub-routes. Current tabs: System, Notifications, Email, Users. Theme settings tab needs to be added as a new route at `/settings/appearance`.

**`frontend/src/pages/SystemSettings.tsx`:**

Uses `getSettings()` / `updateSetting()` from `frontend/src/api/settings.ts` to read/write `Setting` key-value pairs from the backend. Pattern for backend-stored UI settings already established (e.g., `ui.domain_link_behavior`).

### 2.2 CSS Custom Properties Foundation

**`frontend/src/index.css`:**

Already has a comprehensive design token system under `:root` with:
- `--color-brand-*` (RGB format, brand palette)
- `--color-bg-*` (surface colors)
- `--color-border-*`
- `--color-text-*`
- `--color-success`, `--color-warning`, `--color-error`, `--color-info`
- Typography, spacing, effects tokens

The dark-mode-default values live in `:root`. A `.light` class block overrides surface, border, and text tokens.

**`frontend/tailwind.config.js`:**

- `darkMode: 'class'` — must change to `['selector', '[data-theme="dark"]']`
- All semantic colors already map to CSS custom properties (e.g., `surface.base` → `rgb(var(--color-bg-base) / <alpha-value>)`)

This means the CSS custom property foundation is largely already in place. The migration is primarily:
1. Rename the theming mechanism from `.dark`/`.light` classes to `[data-theme="dark"]`/`[data-theme="light"]` attributes.
2. Add `[data-theme="high-contrast-dark"]`, `[data-theme="high-contrast-light"]`, `[data-theme="solarized"]` blocks.
3. Add a `[data-theme="custom"]` block whose properties are overridden by CSS variables written by the custom color picker.

### 2.3 Layout and Logo Usage

**`frontend/src/components/Layout.tsx`:**

- Sidebar header: `<img src="/logo.png">` (collapsed state) or `<picture><source srcSet="/banner.webp"><img src="/banner.png"></picture>` (expanded state).
- Mobile header: `<img src="/logo.png">`.
- Uses raw Tailwind classes for dark mode (`dark:bg-dark-sidebar`, `dark:border-gray-800`, etc.) — these will need updating to use CSS custom property-based classes or `data-theme` selectors.
- `ThemeToggle` is rendered in both desktop and mobile headers.

**`frontend/public/`:**

Contains: `banner.png`, `banner.svg`, `banner.webp`, `favicon.png`, `logo.png`, `logo.svg`, `logo.webp`.

**`backend/internal/server/server.go`:**

Explicitly serves `/logo.png`, `/logo.webp`, `/logo.svg`, `/banner.png`, `/banner.webp`, `/banner.svg` as static files from `frontendDir`. A custom logo upload endpoint needs to write a file to the data directory and serve it, or store a URL in settings.

### 2.4 Backend Settings Pattern

`backend/internal/models/setting.go` stores key-value pairs. `backend/internal/api/handlers/settings_handler.go` provides `GET /settings` and `POST /settings` for reading and writing. The existing `updateSetting('ui.domain_link_behavior', ...)` pattern is exactly the pattern to use for `ui.logo_url` and `ui.logo_type`.

For file upload, a new endpoint `POST /api/v1/settings/logo` is needed. The file is written to `data/uploads/logo.<ext>` and served via a new static route. The setting `ui.logo_url` stores either a user-provided URL (type `url`) or the server-relative path `/uploads/logo.<ext>` (type `upload`).

### 2.5 Existing Components Available for Reuse

- `frontend/src/components/ui/Dialog.tsx` — modal wrapper
- `frontend/src/components/ui/Button.tsx`, `Card.tsx`, `Input.tsx`, `Label.tsx`, `Select.tsx` — form primitives
- `frontend/src/components/ui/Tabs.tsx` — tab switching within a page

### 2.6 Ignore Files Check

- `.gitignore`: `data/` and `backend/data/` are gitignored. Any upload directory under `data/uploads/` is covered by the existing `data/` rule. No new entries needed.
- `.dockerignore`: `data/` is covered. No new entries needed.
- `codecov.yml`: New test files under `__tests__/` are auto-discovered by existing patterns. No changes needed.
- `frontend/public/`: Already gittracked. Logo uploads go to `data/uploads/` (server-side), not `public/`. No changes needed.

### 2.7 Pre-Built Theme Color Values

All color values in RGB space-separated format (to support Tailwind's alpha modifier syntax `rgb(var(--x) / 0.5)`).

| Token | dark | light | high-contrast-dark | high-contrast-light | solarized |
|---|---|---|---|---|---|
| `--color-bg-base` | `15 23 42` | `248 250 252` | `0 0 0` | `255 255 255` | `0 43 54` |
| `--color-bg-subtle` | `30 41 59` | `241 245 249` | `17 17 17` | `245 245 245` | `7 54 66` |
| `--color-bg-muted` | `51 65 85` | `226 232 240` | `34 34 34` | `230 230 230` | `14 63 75` |
| `--color-bg-elevated` | `30 41 59` | `255 255 255` | `25 25 25` | `255 255 255` | `7 54 66` |
| `--color-border-default` | `51 65 85` | `226 232 240` | `85 85 85` | `170 170 170` | `42 75 86` |
| `--color-border-strong` | `71 85 105` | `203 213 225` | `128 128 128` | `128 128 128` | `88 110 117` |
| `--color-text-primary` | `248 250 252` | `15 23 42` | `255 255 255` | `0 0 0` | `131 148 150` |
| `--color-text-secondary` | `203 213 225` | `71 85 105` | `220 220 220` | `50 50 50` | `101 123 131` |
| `--color-text-muted` | `148 163 184` | `148 163 184` | `170 170 170` | `100 100 100` | `88 110 117` |
| `--color-brand-500` | `59 130 246` | `59 130 246` | `0 217 255` | `0 86 179` | `38 139 210` |
| `color-scheme` | `dark` | `light` | `dark` | `light` | `dark` |

---

## 3. Technical Specifications

### 3.1 TypeScript Types

**File: `frontend/src/context/ThemeContextValue.ts` (replace entirely)**

```typescript
import { createContext } from 'react'

// Built-in theme identifiers
export type BuiltInTheme = 'dark' | 'light' | 'high-contrast-dark' | 'high-contrast-light' | 'solarized'

// Special meta-themes
export type MetaTheme = 'system' | 'custom'

// All valid data-theme attribute values
export type DataThemeValue = BuiltInTheme | 'custom'

// Full theme identifier (includes 'system' which resolves to a DataThemeValue)
export type ThemeId = BuiltInTheme | MetaTheme

// A custom color token set
export interface CustomThemeColors {
  bgBase: string          // RGB e.g. "15 23 42"
  bgSubtle: string
  bgMuted: string
  bgElevated: string
  borderDefault: string
  borderStrong: string
  textPrimary: string
  textSecondary: string
  textMuted: string
  brandPrimary: string
  colorScheme: 'dark' | 'light'
}

// A custom theme definition (user-created)
export interface CustomTheme {
  name: string
  colors: CustomThemeColors
}

// Exported/imported theme bundle
export interface ThemeExport {
  // version is a literal type — do NOT change to number.
  // Add version: 2 as a new union member if the schema changes.
  version: 1
  exportedAt: string      // ISO 8601
  theme: ThemeId
  customTheme?: CustomTheme
}

// Context value shape
export interface ThemeContextType {
  // Current effective theme name (resolved, e.g. 'system' → 'dark'/'light')
  theme: ThemeId
  // Effective data-theme value applied to <html>
  resolvedTheme: DataThemeValue
  // Apply a theme
  setTheme: (theme: ThemeId) => void
  // Custom theme data (if theme === 'custom')
  customTheme: CustomTheme | null
  // Update the custom theme colors
  setCustomTheme: (colors: CustomThemeColors, name?: string) => void
  // Export current theme to JSON
  exportTheme: () => ThemeExport
  // Import theme from JSON
  importTheme: (data: ThemeExport) => void
}

export const ThemeContext = createContext<ThemeContextType | undefined>(undefined)

export const BUILT_IN_THEMES: BuiltInTheme[] = [
  'dark',
  'light',
  'high-contrast-dark',
  'high-contrast-light',
  'solarized',
]

export const BUILT_IN_THEME_LABELS: Record<BuiltInTheme, string> = {
  'dark': 'Dark',
  'light': 'Light',
  'high-contrast-dark': 'High Contrast Dark',
  'high-contrast-light': 'High Contrast Light',
  'solarized': 'Solarized',
}

export const THEME_STORAGE_KEY = 'charon-theme'
export const CUSTOM_THEME_STORAGE_KEY = 'charon-custom-theme'
```

### 3.2 Updated `ThemeProvider`

**File: `frontend/src/context/ThemeContext.tsx` (replace entirely)**

```typescript
import { useEffect, useRef, useState, useCallback, type ReactNode } from 'react'

import {
  ThemeContext,
  type ThemeId,
  type DataThemeValue,
  type CustomTheme,
  type CustomThemeColors,
  type ThemeExport,
  THEME_STORAGE_KEY,
  CUSTOM_THEME_STORAGE_KEY,
} from './ThemeContextValue'

// Resolves 'system' to a concrete data-theme value
function resolveSystemTheme(): DataThemeValue {
  if (typeof window === 'undefined') return 'dark'
  return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

// Maps a ThemeId to the data-theme attribute value applied to <html>
function resolveDataTheme(theme: ThemeId): DataThemeValue {
  if (theme === 'system') return resolveSystemTheme()
  if (theme === 'custom') return 'custom'
  return theme
}

// Applies custom color tokens as inline CSS variables on <html>
function applyCustomTokens(colors: CustomThemeColors) {
  const el = document.documentElement
  el.style.setProperty('--color-bg-base', colors.bgBase)
  el.style.setProperty('--color-bg-subtle', colors.bgSubtle)
  el.style.setProperty('--color-bg-muted', colors.bgMuted)
  el.style.setProperty('--color-bg-elevated', colors.bgElevated)
  el.style.setProperty('--color-border-default', colors.borderDefault)
  el.style.setProperty('--color-border-strong', colors.borderStrong)
  el.style.setProperty('--color-text-primary', colors.textPrimary)
  el.style.setProperty('--color-text-secondary', colors.textSecondary)
  el.style.setProperty('--color-text-muted', colors.textMuted)
  el.style.setProperty('--color-brand-500', colors.brandPrimary)
  el.style.setProperty('color-scheme', colors.colorScheme)
}

function clearCustomTokens() {
  const el = document.documentElement
  const props = [
    '--color-bg-base', '--color-bg-subtle', '--color-bg-muted', '--color-bg-elevated',
    '--color-border-default', '--color-border-strong', '--color-text-primary',
    '--color-text-secondary', '--color-text-muted', '--color-brand-500', 'color-scheme',
  ]
  props.forEach(p => el.style.removeProperty(p))
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemeId>(() => {
    try {
      return (localStorage.getItem(THEME_STORAGE_KEY) as ThemeId) || 'dark'
    } catch {
      return 'dark'
    }
  })

  const [customTheme, setCustomThemeState] = useState<CustomTheme | null>(() => {
    try {
      const raw = localStorage.getItem(CUSTOM_THEME_STORAGE_KEY)
      return raw ? (JSON.parse(raw) as CustomTheme) : null
    } catch {
      return null
    }
  })

  // isFirstRender: on mount the inline script in index.html already set data-theme.
  // Skip the initial DOM write to avoid a redundant attribute mutation.
  const isFirstRender = useRef(true)

  useEffect(() => {
    if (isFirstRender.current) {
      isFirstRender.current = false
      return
    }
    const resolved = resolveDataTheme(theme)
    document.documentElement.setAttribute('data-theme', resolved)

    if (resolved === 'custom' && customTheme) {
      applyCustomTokens(customTheme.colors)
    } else {
      clearCustomTokens()
    }

    try {
      localStorage.setItem(THEME_STORAGE_KEY, theme)
    } catch {
      // localStorage unavailable (private browsing) — silently ignore
    }
  }, [theme, customTheme])

  // Listen for system preference changes when in 'system' mode
  useEffect(() => {
    if (theme !== 'system') return
    const mq = window.matchMedia('(prefers-color-scheme: light)')
    const handler = () => {
      document.documentElement.setAttribute('data-theme', resolveSystemTheme())
    }
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [theme])

  const setTheme = useCallback((newTheme: ThemeId) => {
    setThemeState(newTheme)
  }, [])

  const setCustomTheme = useCallback((colors: CustomThemeColors, name = 'Custom') => {
    const ct: CustomTheme = { name, colors }
    setCustomThemeState(ct)
    try {
      localStorage.setItem(CUSTOM_THEME_STORAGE_KEY, JSON.stringify(ct))
    } catch {
      // silently ignore
    }
    setThemeState('custom')
  }, [])

  const exportTheme = useCallback((): ThemeExport => ({
    version: 1,
    exportedAt: new Date().toISOString(),
    theme,
    customTheme: theme === 'custom' && customTheme ? customTheme : undefined,
  }), [theme, customTheme])

  const importTheme = useCallback((data: ThemeExport) => {
    if (data.customTheme) {
      setCustomThemeState(data.customTheme)
      try {
        localStorage.setItem(CUSTOM_THEME_STORAGE_KEY, JSON.stringify(data.customTheme))
      } catch {
        // silently ignore
      }
    }
    setThemeState(data.theme)
  }, [])

  const resolvedTheme = resolveDataTheme(theme)

  return (
    <ThemeContext.Provider value={{
      theme,
      resolvedTheme,
      setTheme,
      customTheme,
      setCustomTheme,
      exportTheme,
      importTheme,
    }}>
      {children}
    </ThemeContext.Provider>
  )
}
```

### 3.3 Updated `useTheme` Hook

**File: `frontend/src/hooks/useTheme.ts` (replace entirely)**

```typescript
import { useContext } from 'react'

import { ThemeContext, type ThemeContextType } from '../context/ThemeContextValue'

export function useTheme(): ThemeContextType {
  const context = useContext(ThemeContext)
  if (context === undefined) {
    throw new Error('useTheme must be used within a ThemeProvider')
  }
  return context
}
```

### 3.4 Updated `index.html` — Inline Script and `data-theme`

**File: `frontend/index.html` (full replacement)**

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/png" href="/favicon.png" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Charon</title>
    <!-- Vite injects <link rel="stylesheet"> above this script at build time.
         The script must remain LAST in <head> so CSS is declared before it runs. -->
    <script>!function(){var k='charon-theme',c='charon-custom-theme';try{var t=localStorage.getItem(k)||localStorage.getItem('theme')||'dark';var r=t==='system'?(window.matchMedia('(prefers-color-scheme: light)').matches?'light':'dark'):t==='custom'?'custom':t;document.documentElement.setAttribute('data-theme',r);if(r==='custom'){try{var ct=JSON.parse(localStorage.getItem(c)||'null');if(ct&&ct.colors){var s=document.documentElement.style;s.setProperty('--color-bg-base',ct.colors.bgBase);s.setProperty('--color-bg-subtle',ct.colors.bgSubtle);s.setProperty('--color-bg-muted',ct.colors.bgMuted);s.setProperty('--color-bg-elevated',ct.colors.bgElevated);s.setProperty('--color-border-default',ct.colors.borderDefault);s.setProperty('--color-border-strong',ct.colors.borderStrong);s.setProperty('--color-text-primary',ct.colors.textPrimary);s.setProperty('--color-text-secondary',ct.colors.textSecondary);s.setProperty('--color-text-muted',ct.colors.textMuted);s.setProperty('--color-brand-500',ct.colors.brandPrimary);s.setProperty('color-scheme',ct.colors.colorScheme)}}catch(e2){}}}catch(e){document.documentElement.setAttribute('data-theme','dark')}}();</script>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

Key changes from current:
- `<script>` moved from line 5 to last element in `<head>`.
- Uses `setAttribute('data-theme', ...)` instead of `classList.add(...)`.
- Storage key is `'charon-theme'`. **Fallback migration:** The script reads `localStorage.getItem('charon-theme') || localStorage.getItem('theme') || 'dark'`. This preserves the theme preference of users upgrading from the old system (whose preference is stored under `'theme'`). This is the canonical approach — see AC-12.
- Handles `'system'`, `'custom'`, and all built-in theme names.
- Custom theme colors applied inline at startup (zero FOUC even for custom themes).

### 3.5 Updated CSS in `index.css`

**File: `frontend/src/index.css`**

The existing `:root` block becomes the base (dark defaults). The `.light` class block is replaced by `[data-theme="light"]`. New blocks are added for each theme and for `[data-theme="custom"]`.

**Structure:**

```css
/* ========================================
 * BASE TOKENS (dark defaults — applied to all themes as fallback)
 * ======================================== */
:root {
  /* All existing --color-brand-*, typography, spacing, effects tokens remain */
  /* Semantic surface tokens default to dark values (existing values) */
  --color-bg-base: 15 23 42;
  /* ... all existing :root values ... */
  color-scheme: dark;
}

/* ========================================
 * BUILT-IN THEMES
 * ======================================== */
[data-theme="dark"] {
  /* Same as :root defaults — explicit for clarity */
  --color-bg-base: 15 23 42;
  --color-bg-subtle: 30 41 59;
  --color-bg-muted: 51 65 85;
  --color-bg-elevated: 30 41 59;
  --color-bg-overlay: 2 6 23;
  --color-border-default: 51 65 85;
  --color-border-muted: 30 41 59;
  --color-border-strong: 71 85 105;
  --color-text-primary: 248 250 252;
  --color-text-secondary: 203 213 225;
  --color-text-muted: 148 163 184;
  --color-text-inverted: 15 23 42;
  --color-success-muted: 20 83 45;
  --color-warning-muted: 113 63 18;
  --color-error-muted: 127 29 29;
  --color-info-muted: 30 58 138;
  color-scheme: dark;
  color: rgb(var(--color-text-primary));
  background-color: rgb(var(--color-bg-base));
}

[data-theme="light"] {
  --color-bg-base: 248 250 252;
  --color-bg-subtle: 241 245 249;
  --color-bg-muted: 226 232 240;
  --color-bg-elevated: 255 255 255;
  --color-bg-overlay: 15 23 42;
  --color-border-default: 226 232 240;
  --color-border-muted: 241 245 249;
  --color-border-strong: 203 213 225;
  --color-text-primary: 15 23 42;
  --color-text-secondary: 71 85 105;
  --color-text-muted: 148 163 184;
  --color-text-inverted: 255 255 255;
  --color-success-muted: 220 252 231;
  --color-warning-muted: 254 249 195;
  --color-error-muted: 254 226 226;
  --color-info-muted: 219 234 254;
  color-scheme: light;
  color: rgb(var(--color-text-primary));
  background-color: rgb(var(--color-bg-base));
}

[data-theme="high-contrast-dark"] {
  --color-bg-base: 0 0 0;
  --color-bg-subtle: 17 17 17;
  --color-bg-muted: 34 34 34;
  --color-bg-elevated: 25 25 25;
  --color-bg-overlay: 0 0 0;
  --color-border-default: 85 85 85;
  --color-border-muted: 51 51 51;
  --color-border-strong: 128 128 128;
  --color-text-primary: 255 255 255;
  --color-text-secondary: 220 220 220;
  --color-text-muted: 170 170 170;
  --color-text-inverted: 0 0 0;
  --color-brand-500: 0 217 255;
  --color-success-muted: 0 40 0;
  --color-warning-muted: 50 40 0;
  --color-error-muted: 50 0 0;
  --color-info-muted: 0 30 60;
  color-scheme: dark;
  color: rgb(var(--color-text-primary));
  background-color: rgb(var(--color-bg-base));
}

[data-theme="high-contrast-light"] {
  --color-bg-base: 255 255 255;
  --color-bg-subtle: 245 245 245;
  --color-bg-muted: 230 230 230;
  --color-bg-elevated: 255 255 255;
  --color-bg-overlay: 0 0 0;
  --color-border-default: 170 170 170;
  --color-border-muted: 200 200 200;
  --color-border-strong: 128 128 128;
  --color-text-primary: 0 0 0;
  --color-text-secondary: 50 50 50;
  --color-text-muted: 100 100 100;
  --color-text-inverted: 255 255 255;
  --color-brand-500: 0 86 179;
  --color-success-muted: 200 240 200;
  --color-warning-muted: 255 240 180;
  --color-error-muted: 255 200 200;
  --color-info-muted: 200 220 255;
  color-scheme: light;
  color: rgb(var(--color-text-primary));
  background-color: rgb(var(--color-bg-base));
}

[data-theme="solarized"] {
  /* Solarized intentionally uses tight contrast; these values are based on the Solarized Dark palette */
  --color-bg-base: 0 43 54;
  --color-bg-subtle: 7 54 66;
  --color-bg-muted: 14 63 75;   /* distinct from bg-base; slightly lighter surface */
  --color-bg-elevated: 7 54 66;
  --color-bg-overlay: 0 43 54;
  --color-border-default: 42 75 86;  /* distinct from bg-subtle to ensure visible separation */
  --color-border-muted: 0 43 54;
  --color-border-strong: 88 110 117;
  --color-text-primary: 131 148 150;
  --color-text-secondary: 101 123 131;
  --color-text-muted: 88 110 117;
  --color-text-inverted: 0 43 54;
  --color-brand-500: 38 139 210;
  --color-success: 133 153 0;
  --color-warning: 181 137 0;
  --color-error: 220 50 47;
  --color-success-muted: 10 30 0;
  --color-warning-muted: 40 30 0;
  --color-error-muted: 50 5 5;
  --color-info-muted: 0 30 50;
  color-scheme: dark;
  color: rgb(var(--color-text-primary));
  background-color: rgb(var(--color-bg-base));
}

/* [data-theme="custom"] uses inline styles set by ThemeProvider.
   The base :root values act as fallback. No static block needed. */

/* Remove the .light class block (replaced by [data-theme="light"]) */
/* Remove the .dark scrollbar rule (replaced by [data-theme="dark"]) */
```

**Scrollbar CSS migration:** Replace `.dark .overflow-y-auto::-webkit-scrollbar-thumb` with `[data-theme="dark"] .overflow-y-auto::-webkit-scrollbar-thumb` and similarly for `scrollbar-color`.

### 3.6 Tailwind Config Changes

**File: `frontend/tailwind.config.js`**

Change `darkMode: 'class'` to:

```javascript
darkMode: ['selector', '[data-theme="dark"]'],
```

This tells Tailwind that `dark:` prefixed utilities apply when the `[data-theme="dark"]` selector is present on an ancestor. The `high-contrast-dark` and `solarized` themes are both dark-scheme and will also benefit from this. For now, `dark:` utility classes inside Layout.tsx and other components will still work correctly for the two dark themes; high-contrast and solarized themes will need their own CSS variable overrides (which are provided in the `index.css` blocks above) rather than `dark:` Tailwind utilities.

**Note on Layout.tsx hardcoded dark classes:** `Layout.tsx` contains many hardcoded Tailwind classes like `dark:bg-dark-sidebar`, `dark:border-gray-800`, `dark:text-gray-400`, etc. These must be migrated as part of Phase 1 (Foundation/CSS) to use semantic CSS custom property classes (`bg-surface-elevated`, `border-border`, `text-content-secondary`) so all themes work correctly. This is a significant but mechanical refactor.

### 3.7 New Components

#### 3.7.1 `ThemeGallery`

**File:** `frontend/src/components/theme/ThemeGallery.tsx`

**Props:**
```typescript
interface ThemeGalleryProps {
  value: ThemeId        // currently selected
  previewTheme: ThemeId | null  // theme being hovered/previewed
  onChange: (theme: ThemeId) => void
  onPreview: (theme: ThemeId | null) => void
}
```

Renders a grid of `ThemeCard` components. Each card shows a mini preview of the theme's color palette and a label. A selected card shows a checkmark badge.

#### 3.7.2 `ThemeCard`

**File:** `frontend/src/components/theme/ThemeCard.tsx`

**Props:**
```typescript
interface ThemeCardProps {
  themeId: ThemeId
  label: string
  selected: boolean
  onSelect: () => void
  onPreview: () => void
  onPreviewEnd: () => void
}
```

A card with:
- Mini color swatch row (bg-base, brand color, text color)
- Theme name label
- Selected indicator
- `onMouseEnter` / `onFocus` → `onPreview()`
- `onMouseLeave` / `onBlur` → `onPreviewEnd()`
- `onClick` → `onSelect()`

**Accessibility:** Use `role="radio"` on each `ThemeCard` and `role="radiogroup"` on the `ThemeGallery` container. Set `aria-checked={selected}` on each card. This matches WCAG 2.1 SC 4.1.2 for custom radio-select controls. The selected card must have `aria-checked="true"`; all others must have `aria-checked="false"`.

#### 3.7.3 `CustomColorPicker`

**File:** `frontend/src/components/theme/CustomColorPicker.tsx`

**Props:**
```typescript
interface CustomColorPickerProps {
  value: CustomThemeColors
  onChange: (colors: CustomThemeColors) => void
}
```

Renders a form with one `<input type="color">` per token. Converts hex color picker output to RGB space-separated format for the custom property system.

Helper utilities:
- `hexToRgb(hex: string): string` — `"#3b82f6"` → `"59 130 246"`
- `rgbToHex(rgb: string): string` — `"59 130 246"` → `"#3b82f6"`

Fields exposed in the picker:
- Background Base / Subtle / Muted / Elevated
- Border Default / Strong
- Text Primary / Secondary / Muted
- Brand Primary (brand-500)
- Color Scheme toggle (dark/light)

**Default value:** When no existing custom theme exists (`customTheme === null` in the context), the parent `AppearanceSettings` must initialize `CustomColorPicker`'s `value` prop with `colorScheme: 'dark'` as the default, plus the dark theme's color values as sensible starting points. `CustomColorPicker` itself is a controlled component — it does not own its initial state.

#### 3.7.4 `ThemeImportExport`

**File:** `frontend/src/components/theme/ThemeImportExport.tsx`

Provides two buttons:
1. **Export:** Calls `useTheme().exportTheme()`, serializes to JSON, triggers file download as `charon-theme.json`.
2. **Import:** File input (`<input type="file" accept=".json">`), reads file, parses, validates schema, calls `useTheme().importTheme(data)` on success. Shows toast on error.

**Validation function:**
```typescript
// Valid ThemeId values — kept in sync with ThemeContextValue.ts
const VALID_THEME_IDS = ['dark', 'light', 'high-contrast-dark', 'high-contrast-light', 'solarized', 'system', 'custom'] as const

// RGB color value pattern: three integers 0–255 separated by spaces
const RGB_PATTERN = /^\d{1,3} \d{1,3} \d{1,3}$/

function isValidThemeExport(data: unknown): data is ThemeExport {
  if (typeof data !== 'object' || data === null) return false
  const d = data as Record<string, unknown>

  // version must be the literal 1
  if (d.version !== 1) return false
  if (typeof d.exportedAt !== 'string') return false
  // theme must be a known ThemeId — prevents unknown theme injection
  if (!VALID_THEME_IDS.includes(d.theme as typeof VALID_THEME_IDS[number])) return false

  // If a custom theme is present, validate all color fields to prevent CSS injection
  if (d.customTheme !== undefined && d.customTheme !== null) {
    const ct = d.customTheme as Record<string, unknown>
    if (typeof ct !== 'object') return false
    if (ct.colors !== undefined) {
      const colors = ct.colors as Record<string, unknown>
      const colorFields = [
        'bgBase', 'bgSubtle', 'bgMuted', 'bgElevated',
        'borderDefault', 'borderStrong',
        'textPrimary', 'textSecondary', 'textMuted',
        'brandPrimary',
      ]
      for (const field of colorFields) {
        if (typeof colors[field] !== 'string') return false
        if (!RGB_PATTERN.test(colors[field] as string)) return false
      }
      if (colors.colorScheme !== 'dark' && colors.colorScheme !== 'light') return false
    }
  }

  return true
}
```

#### 3.7.5 `LogoCustomizer`

**File:** `frontend/src/components/theme/LogoCustomizer.tsx`

**Props:**
```typescript
interface LogoCustomizerProps {
  currentLogoUrl: string | null
  onSave: (value: string, type: 'url' | 'upload') => void
  isSaving: boolean
}
```

**Admin access control:** `LogoCustomizer` must check the current user's role via `useAuth()`. If the user is not an admin, render a read-only notice — "Logo customization requires admin access" — and hide both the upload form and URL form fields entirely. This prevents non-admin users from encountering an unexplained HTTP 403 when the backend route rejects their request.

Two tabs (admin only):
1. **Upload:** `<input type="file" accept="image/png,image/jpeg,image/webp">`, max size 2 MB, triggers `POST /api/v1/settings/logo` multipart upload. SVG is not accepted for upload — too many XSS vectors to sanitize safely server-side. The hint text must read: "PNG, JPG or WebP — max 2 MB".
2. **URL:** Text input for an external URL (any format including SVG — user's responsibility), triggers `POST /settings` with key `ui.logo_url`.

Shows current logo preview. "Reset to Default" button clears the setting.

#### 3.7.6 `ThemePreviewOverlay`

**File:** `frontend/src/components/theme/ThemePreviewOverlay.tsx`

When `previewTheme` is non-null, this component temporarily applies the preview theme's `data-theme` attribute to `<html>` without committing to state. Uses `useEffect` to set and restore the attribute.

```typescript
interface ThemePreviewOverlayProps {
  previewTheme: ThemeId | null
  resolvedCurrentTheme: DataThemeValue
}
```

On `previewTheme` change:
- Not null: `document.documentElement.setAttribute('data-theme', resolveDataTheme(previewTheme))`
- Null: `document.documentElement.setAttribute('data-theme', resolvedCurrentTheme)` (restore)

**Race condition prevention:** `AppearanceSettings` must call `setPreviewTheme(null)` immediately before calling `setTheme(selectedTheme)` in the `onChange` handler of `ThemeGallery`. This ensures the preview restoration effect runs synchronously before the `ThemeProvider`'s effect picks up the new committed theme, preventing the two effects from racing to write `data-theme`.

```typescript
// In AppearanceSettings onChange handler:
const handleThemeChange = (newTheme: ThemeId) => {
  setPreviewTheme(null)          // clear preview first
  setTheme(newTheme)             // then commit
}
```

#### 3.7.7 Updated `ThemeToggle`

**File:** `frontend/src/components/ThemeToggle.tsx` (replace existing binary toggle)

Becomes a dropdown or a button that opens the Appearance settings panel directly:

```typescript
export function ThemeToggle() {
  const { resolvedTheme } = useTheme()
  const navigate = useNavigate()
  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={() => navigate('/settings/appearance')}
      title="Theme settings"
      aria-label={`Current theme: ${resolvedTheme}. Open appearance settings.`}
    >
      {/* Icon that reflects the current resolved theme */}
      {resolvedTheme === 'light' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
    </Button>
  )
}
```

### 3.8 New Page: `AppearanceSettings`

**File:** `frontend/src/pages/AppearanceSettings.tsx`

Mounted at `/settings/appearance`. Added to the `Settings.tsx` tab navigation alongside System, Notifications, Email, Users.

Sections:
1. **Theme Gallery** — `<ThemeGallery>` with `<ThemePreviewOverlay>`
2. **Custom Theme** (shown when theme = 'custom') — `<CustomColorPicker>`
3. **Import / Export** — `<ThemeImportExport>`
4. **Logo Customization** — `<LogoCustomizer>`
5. **Follow System** — Toggle within `ThemeGallery` (selecting 'system' mode)

State management:
- `previewTheme: ThemeId | null` — local `useState`, drives `ThemePreviewOverlay`
- Reads and writes via `useTheme()`

### 3.9 Backend API for Logo Upload

**New file:** `backend/internal/api/handlers/logo_handler.go`

```go
// UploadLogo handles POST /api/v1/settings/logo
// Accepts multipart form with field "logo" (image/png, image/jpeg, image/webp)
// SVG file uploads are NOT accepted — SVG is too complex to sanitize safely inline.
// SVG logos can still be configured via the URL option (user's responsibility).
// Validates MIME type via server-side byte sniffing, max size 2MB, sanitizes filename
// Writes to dataRoot/uploads/logo.<ext>
// Saves setting ui.logo_url = "/uploads/logo.<ext>", ui.logo_type = "upload"
// Returns 200 { url: "/uploads/logo.<ext>" }
```

Security constraints:
- **Accept only `image/png`, `image/jpeg`, `image/webp` for file uploads.** SVG uploads are explicitly disallowed — the SVG format supports too many XSS vectors (`javascript:` href links, `on*` event handler attributes, `<foreignObject>`, `<use>` with external references, `<animate>` manipulating `href`/`src`, `<?xml-stylesheet?>` processing instructions, CSS `url()` within SVG `<style>` blocks) to be safely sanitized inline. SVG logos can be set via the URL option (user assumes responsibility).
- **Server-side MIME detection is mandatory.** Read the first 512 bytes of the uploaded file and pass them to `net/http.DetectContentType()`. Do NOT trust the multipart `Content-Type` header — it can be spoofed by the client. If the detected MIME type does not match one of the three accepted types, reject the request with HTTP 400 even if the extension appears valid.
- Max size: 2 MB (enforced with `http.MaxBytesReader` applied before reading any bytes).
- Filename: always normalized to `logo.<ext>` regardless of user input (`filepath.Clean` is not sufficient alone — the file is stored with a fixed name to prevent path traversal). Extension is derived from the server-detected MIME type, not from the user-supplied filename.
- Write to `dataRoot/uploads/logo.<ext>` using `os.WriteFile` with permissions `0644`.

**New route registration in `backend/internal/api/routes/routes.go`:**

```go
// Authenticated admin-only routes
authed.POST("/settings/logo", logoHandler.UploadLogo)
authed.DELETE("/settings/logo", logoHandler.DeleteLogo)
```

**New static file route in `backend/internal/server/server.go`:**

```go
// Serve uploaded logo from data directory
if dataDir != "" {
    router.Static("/uploads", dataDir+"/uploads")
}
```

**`NewRouter` signature change** — needs `dataDir string` parameter so it can serve `/uploads`. Current signature is `NewRouter(frontendDir string)`. New signature: `NewRouter(frontendDir, dataDir string)`.

**`dataDir` derivation (NB-2):** In `backend/cmd/api/main.go`, pass `filepath.Dir(cfg.DatabasePath)` as the `dataDir` argument to `server.NewRouter`. Do NOT add a new `DataDir` field to the config struct — derive it from `cfg.DatabasePath` at the call site.

**`backend/internal/api/routes/routes.go` AutoMigrate** — no schema change needed (uses existing `Setting` model).

**Updated `frontend/src/api/settings.ts`:**

```typescript
// New function
export const uploadLogo = async (file: File): Promise<{ url: string }> => {
  const form = new FormData()
  form.append('logo', file)
  const response = await client.post('/settings/logo', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return response.data
}

export const deleteLogo = async (): Promise<void> => {
  await client.delete('/settings/logo')
}
```

### 3.10 Logo Source of Truth in `Layout.tsx`

`Layout.tsx` must read `ui.logo_url` from the settings API and use it in place of the hardcoded `/logo.png` and `/banner.png`:

```typescript
// In Layout.tsx
const { data: settings } = useQuery({ queryKey: ['settings'], queryFn: getSettings })
const customLogoUrl = settings?.['ui.logo_url'] || null
const logoSrc = customLogoUrl || '/logo.png'
const bannerSrc = customLogoUrl || undefined  // custom logo used for both positions if set
```

When a custom logo URL is set, both the collapsed icon and expanded banner use it. Default behavior (no setting) remains exactly as before.

### 3.11 Settings.tsx — Add Appearance Tab

**File: `frontend/src/pages/Settings.tsx`**

```typescript
// Add to navItems:
{ path: '/settings/appearance', label: t('settings.appearance'), icon: Palette }
```

**File: `frontend/src/App.tsx`**

Add the following lazy import alongside the other page lazy imports:

```typescript
const AppearanceSettings = lazy(() => import('./pages/AppearanceSettings'))
```

Then add the route inside the Settings nested routes:

```typescript
<Route path="appearance" element={<AppearanceSettings />} />
```

### 3.12 i18n Keys Required

New keys to add to `frontend/src/locales/en/translation.json` (and all other locale files):

```json
{
  "settings": {
    "appearance": "Appearance"
  },
  "appearance": {
    "title": "Appearance",
    "description": "Customize the look and feel of Charon",
    "themeGallery": "Theme Gallery",
    "themeGalleryDescription": "Choose a built-in theme or create your own",
    "followSystem": "Follow System",
    "followSystemDescription": "Automatically match your OS light/dark preference",
    "customTheme": "Custom Theme",
    "customThemeDescription": "Fine-tune colors to create your own theme",
    "colorPickerBgBase": "Background",
    "colorPickerBgSubtle": "Subtle Background",
    "colorPickerBgMuted": "Muted Background",
    "colorPickerBgElevated": "Elevated Surface",
    "colorPickerBorderDefault": "Border",
    "colorPickerBorderStrong": "Strong Border",
    "colorPickerTextPrimary": "Primary Text",
    "colorPickerTextSecondary": "Secondary Text",
    "colorPickerTextMuted": "Muted Text",
    "colorPickerBrandPrimary": "Accent Color",
    "colorPickerColorScheme": "Base Scheme",
    "importExport": "Import / Export",
    "importExportDescription": "Share themes between Charon instances",
    "exportButton": "Export Theme",
    "importButton": "Import Theme",
    "importError": "Invalid theme file",
    "logoCustomization": "Logo Customization",
    "logoCustomizationDescription": "Replace the Charon logo with your own",
    "logoUploadTab": "Upload File",
    "logoUrlTab": "Enter URL",
    "logoResetButton": "Reset to Default",
    "logoSaveButton": "Save Logo",
    "logoPreview": "Logo Preview",
    "logoUploadHint": "PNG, JPG or WebP — max 2 MB",
    "logoUrlPlaceholder": "https://example.com/logo.png"
  }
}
```

### 3.13 File Inventory

| File | Change Type | Description |
|---|---|---|
| `frontend/index.html` | Modify | Move `<script>` to last in `<head>`; switch to `data-theme` attribute and `charon-theme` key |
| `frontend/src/context/ThemeContextValue.ts` | Replace | Expanded types: `ThemeId`, `CustomTheme`, `ThemeExport`, context shape |
| `frontend/src/context/ThemeContext.tsx` | Replace | Full rewrite: `useEffect` + `useRef` skip, system mode, custom tokens |
| `frontend/src/context/__tests__/ThemeContext.test.tsx` | New | Unit tests for `ThemeProvider` |
| `frontend/src/hooks/useTheme.ts` | Modify | Return full `ThemeContextType` |
| `frontend/src/hooks/__tests__/useTheme.test.tsx` | Extend | Happy-path and new API tests |
| `frontend/src/index.css` | Modify | Replace `.light` class with `[data-theme]` selectors; add 5 theme blocks |
| `frontend/tailwind.config.js` | Modify | `darkMode` changed to selector mode |
| `frontend/src/components/Layout.tsx` | Modify | Migrate dark: Tailwind classes to semantic; read logo from settings |
| `frontend/src/components/ThemeToggle.tsx` | Replace | Navigate to `/settings/appearance`; icon reflects resolved theme |
| `frontend/src/components/theme/ThemeGallery.tsx` | New | Theme grid component |
| `frontend/src/components/theme/ThemeCard.tsx` | New | Individual theme preview card |
| `frontend/src/components/theme/CustomColorPicker.tsx` | New | Color input form |
| `frontend/src/components/theme/ThemeImportExport.tsx` | New | Export/import buttons |
| `frontend/src/components/theme/LogoCustomizer.tsx` | New | Logo upload/URL component |
| `frontend/src/components/theme/ThemePreviewOverlay.tsx` | New | Transient preview controller |
| `frontend/src/components/theme/__tests__/ThemeGallery.test.tsx` | New | Unit tests |
| `frontend/src/components/theme/__tests__/ThemeCard.test.tsx` | New | Unit tests |
| `frontend/src/components/theme/__tests__/CustomColorPicker.test.tsx` | New | Unit tests (hex↔RGB conversion + rendering) |
| `frontend/src/components/theme/__tests__/ThemeImportExport.test.tsx` | New | Unit tests (export download, import parsing, validation) |
| `frontend/src/components/theme/__tests__/LogoCustomizer.test.tsx` | New | Unit tests (file input, URL input, reset) |
| `frontend/src/components/theme/__tests__/ThemePreviewOverlay.test.tsx` | New | Unit tests |
| `frontend/src/pages/AppearanceSettings.tsx` | New | `/settings/appearance` page |
| `frontend/src/pages/Settings.tsx` | Modify | Add Appearance tab to nav |
| `frontend/src/pages/__tests__/AppearanceSettings.test.tsx` | New | Unit tests |
| `frontend/src/App.tsx` | Modify | Add `<Route path="appearance" element={<AppearanceSettings />} />` |
| `frontend/src/api/settings.ts` | Modify | Add `uploadLogo`, `deleteLogo` |
| `frontend/src/api/__tests__/settings.test.ts` | Extend | Tests for new functions |
| `frontend/src/locales/en/translation.json` | Modify | Add i18n keys |
| `frontend/src/locales/*/translation.json` | Modify | Add i18n keys (5 locale files) |
| `backend/internal/api/handlers/logo_handler.go` | New | `UploadLogo`, `DeleteLogo` |
| `backend/internal/api/handlers/logo_handler_test.go` | New | Unit tests |
| `backend/internal/api/routes/routes.go` | Modify | Register logo routes |
| `backend/internal/server/server.go` | Modify | Add `/uploads` static route; update `NewRouter` signature |
| `backend/internal/server/server_test.go` | Modify | Update `NewRouter` call with data dir arg |
| `docs/features.md` | Modify | Expand "Dark Mode & Modern UI" section |
| `ARCHITECTURE.md` | Modify | Note theme system in Frontend section |

---

## 4. Implementation Plan

### Phase 1 — Foundation: CSS, Types, and Tailwind Migration

**Goal:** Establish the `data-theme` CSS foundation and migrate `Layout.tsx` from hardcoded dark: classes to semantic tokens. No visible UI change for existing dark/light users. This is the riskiest refactor because Layout.tsx is the outermost shell.

**Files changed:**
- `frontend/src/index.css` — Replace `.light` block with `[data-theme="light"]`, add `[data-theme="dark"]`, `[data-theme="high-contrast-dark"]`, `[data-theme="high-contrast-light"]`, `[data-theme="solarized"]`. Update scrollbar CSS to `[data-theme]` selectors.
- `frontend/tailwind.config.js` — Change `darkMode` to selector mode.
- `frontend/src/components/Layout.tsx` — Migrate all `dark:bg-*`, `dark:border-*`, `dark:text-*` hardcoded classes to semantic classes (`bg-surface-elevated`, `border-border`, `text-content-secondary`, etc.) that already work via CSS custom properties. This is a mechanical find-replace of approximately 30 class-name occurrences.

**Known limitation of Phase 1 (scope boundary):** The frontend codebase contains 214+ `dark:` class usages outside of `Layout.tsx` (e.g., `CSPBuilder.tsx`, `NotificationCenter.tsx`, `PasswordStrengthMeter.tsx`, `SetupGuard.tsx`, `PermissionsPolicyBuilder.tsx`, and others). With Tailwind `['selector', '[data-theme="dark"]']`, these `dark:` utilities only activate for the `dark` theme — they do NOT activate for `high-contrast-dark` or `solarized`. As a result, under non-dark themes, those components will render with their light-mode fallback styles rather than a fully themed appearance. This is an accepted limitation of this PR. A separate follow-up PR will audit and migrate all remaining `dark:` usages to semantic CSS custom property classes. See AC-18.

**Validation:** Run `npm run build` and visually confirm dark and light modes still work.

### Phase 2 — Core Theme System with FOUC Fix

**Goal:** Replace the binary theme system with the full `ThemeId` system, fix the FOUC, and wire up `ThemeProvider` with the new context shape.

**Files changed:**
- `frontend/index.html` — Move `<script>` to last in `<head>`; update script to use `data-theme` and `charon-theme` key.
- `frontend/src/context/ThemeContextValue.ts` — Full type expansion.
- `frontend/src/context/ThemeContext.tsx` — Full rewrite.
- `frontend/src/hooks/useTheme.ts` — Return full context type.
- `frontend/src/components/ThemeToggle.tsx` — Navigate to `/settings/appearance`.
- `frontend/src/pages/Settings.tsx` — Add Appearance tab.
- `frontend/src/App.tsx` — Add appearance route.
- `frontend/src/context/__tests__/ThemeContext.test.tsx` — New unit tests.
- `frontend/src/hooks/__tests__/useTheme.test.tsx` — Extended tests.

**Key test scenarios for `ThemeContext.test.tsx`:**

| Test ID | Scenario | Assertion |
|---|---|---|
| TC-01 | Default theme is `dark` (no localStorage) | `theme === 'dark'` |
| TC-02 | Reads saved `light` from storage | `theme === 'light'` |
| TC-03 | Reads saved `system` from storage | `theme === 'system'` |
| TC-04 | On mount, does NOT set `data-theme` (inline script owns it) | `setAttribute` spy not called on first render |
| TC-05 | `setTheme('light')` updates state and sets `data-theme="light"` | After call, attribute set |
| TC-06 | `setTheme('high-contrast-dark')` sets `data-theme="high-contrast-dark"` | Attribute correct |
| TC-07 | `setTheme('system')` resolves to OS preference | Mocked `matchMedia` returns `dark` → attribute `dark` |
| TC-08 | `setTheme('custom')` with `customTheme` applies inline styles | `style.setProperty` called for `--color-bg-base` |
| TC-09 | `setCustomTheme(colors)` sets theme to 'custom' | `theme === 'custom'` |
| TC-10 | `exportTheme()` returns valid `ThemeExport` | `version === 1`, correct theme |
| TC-11 | `importTheme(export)` restores theme | `theme` matches exported value |
| TC-12 | Invalid localStorage value falls back to `dark` | `theme === 'dark'` |
| TC-13 | `setTheme` persists to localStorage | `localStorage.getItem('charon-theme') === 'light'` |

### Phase 3 — Theme Gallery UI

**Goal:** Ship the built-in theme gallery in `/settings/appearance` so users can select any of the five pre-built themes plus system mode.

**Files changed:**
- `frontend/src/components/theme/ThemeGallery.tsx` — New
- `frontend/src/components/theme/ThemeCard.tsx` — New
- `frontend/src/components/theme/ThemePreviewOverlay.tsx` — New
- `frontend/src/pages/AppearanceSettings.tsx` — New (initial version with gallery + system toggle only)
- `frontend/src/components/theme/__tests__/ThemeGallery.test.tsx` — New
- `frontend/src/components/theme/__tests__/ThemeCard.test.tsx` — New
- `frontend/src/components/theme/__tests__/ThemePreviewOverlay.test.tsx` — New
- `frontend/src/pages/__tests__/AppearanceSettings.test.tsx` — New
- `frontend/src/locales/en/translation.json` + other locales — Add i18n keys

**ThemeGallery test scenarios:**

| Test ID | Scenario | Assertion |
|---|---|---|
| TG-01 | Renders all 5 built-in themes + system | 6 cards visible |
| TG-02 | Selected card has `aria-checked="true"`; others have `aria-checked="false"` | Correct a11y attribute on each card; `role="radiogroup"` on container |
| TG-03 | Hovering a card calls `onPreview` | Spy called with correct ThemeId |
| TG-04 | Clicking a card calls `onChange` | Spy called with correct ThemeId |
| TG-05 | Preview overlay restores original after hover end | `data-theme` reverts |
| TG-06 | Rapid hover-then-click: `setPreviewTheme(null)` is called before `setTheme` in the `onChange` handler | Call order verified via mock — preview cleared first, theme set second |

### Phase 4 — Custom Color Picker

**Goal:** Add the custom color picker section to `AppearanceSettings`. Selecting "Custom" in the gallery reveals the picker.

**Files changed:**
- `frontend/src/components/theme/CustomColorPicker.tsx` — New
- `frontend/src/components/theme/__tests__/CustomColorPicker.test.tsx` — New
- `frontend/src/pages/AppearanceSettings.tsx` — Add custom picker section

**CustomColorPicker test scenarios:**

| Test ID | Scenario | Assertion |
|---|---|---|
| CP-01 | `hexToRgb('#3b82f6')` returns `'59 130 246'` | Exact string match |
| CP-02 | `rgbToHex('59 130 246')` returns `'#3b82f6'` | Exact string match |
| CP-03 | Changing bgBase color input triggers `onChange` | Spy called with updated colors |
| CP-04 | Renders all 10 color inputs | 10 `<input type="color">` elements |
| CP-05 | Color scheme toggle switches between dark/light | `colorScheme` in output changes |

### Phase 5 — Theme Import/Export

**Goal:** Add import/export functionality.

**Files changed:**
- `frontend/src/components/theme/ThemeImportExport.tsx` — New
- `frontend/src/components/theme/__tests__/ThemeImportExport.test.tsx` — New
- `frontend/src/pages/AppearanceSettings.tsx` — Add import/export section

**ThemeImportExport test scenarios:**

| Test ID | Scenario | Assertion |
|---|---|---|
| IE-01 | Export button creates downloadable JSON | `URL.createObjectURL` called; filename `charon-theme.json` |
| IE-02 | Import with valid JSON calls `importTheme` | Context `importTheme` spy called |
| IE-03 | Import with invalid JSON shows toast error | `toast.error` called |
| IE-04 | Import with missing `version` field shows error | Validation rejects |
| IE-05 | Import with wrong `version` shows error | Validation rejects |
| IE-06 | Import with malformed color field (e.g., `bgBase: 'red; --injected: val'`) is rejected with validation error | `toast.error` called; `importTheme` not called |

### Phase 6 — Logo Customization

**Goal:** Allow admins to upload a custom logo or point to an external URL. Requires a backend endpoint.

**Files changed:**
- `backend/internal/api/handlers/logo_handler.go` — New
- `backend/internal/api/handlers/logo_handler_test.go` — New
- `backend/internal/api/routes/routes.go` — Register routes
- `backend/internal/server/server.go` — Add `/uploads` static route, update `NewRouter` signature
- `backend/internal/server/server_test.go` — Update test
- `frontend/src/api/settings.ts` — Add `uploadLogo`, `deleteLogo`
- `frontend/src/api/__tests__/settings.test.ts` — New tests
- `frontend/src/components/theme/LogoCustomizer.tsx` — New
- `frontend/src/components/theme/__tests__/LogoCustomizer.test.tsx` — New
- `frontend/src/components/Layout.tsx` — Read `ui.logo_url` setting
- `frontend/src/pages/AppearanceSettings.tsx` — Add logo section

**LogoCustomizer test scenarios:**

| Test ID | Scenario | Assertion |
|---|---|---|
| LC-01 | Upload tab renders file input | `<input type="file">` present |
| LC-02 | File too large (>2MB) shows error before upload | Error message visible |
| LC-03 | Non-image file rejected | Error message visible |
| LC-04 | Valid file triggers `onSave('upload')` | Prop called |
| LC-05 | URL tab renders text input | `<input type="url">` present |
| LC-06 | Valid URL triggers `onSave('url')` | Prop called |
| LC-07 | Reset button calls `onSave('')` | Prop called with empty string |
| LC-08 | Non-admin user sees 'admin access required' notice and no upload/URL form | Notice rendered; no file input or URL input present |

**Backend `logo_handler.go` test scenarios:**

| Test ID | Scenario | Assertion |
|---|---|---|
| BL-01 | Valid PNG upload writes file and returns URL | HTTP 200, `url` field present |
| BL-02 | File exceeds 2MB | HTTP 413 |
| BL-03 | Non-image MIME type (e.g., text/html) — detected via `DetectContentType` | HTTP 400 |
| BL-04 | SVG file upload (any extension) — detected via `DetectContentType` | HTTP 400 (SVG uploads not accepted) |
| BL-04b | Spoofed Content-Type: file has `.svg` extension but multipart header declares `image/png` — server detects SVG bytes and rejects | HTTP 400 |
| BL-04c | Content-Type header omitted in multipart — server detects bytes and accepts valid PNG | HTTP 200 |
| BL-05 | DELETE logo clears setting and removes file | HTTP 200, file removed |
| BL-06 | Unauthenticated upload | HTTP 401 |
| BL-07 | Non-admin upload | HTTP 403 |

### Phase 7 — E2E Playwright Tests and DoD

**Goal:** Write Playwright E2E tests that cover the new theme system user flows.

**File:** `tests/theme.spec.ts` (new)

**E2E Test Scenarios:**

| Test | Description | Assertion |
|---|---|---|
| `no-fouc-warning` | Page load produces no "Layout was forced" console warning | Console listener not triggered |
| `dark-theme-on-fresh-load` | No stored theme → `<html>` has `data-theme="dark"` | Attribute present |
| `light-theme-from-storage` | `localStorage['charon-theme'] = 'light'` → `data-theme="light"` | Attribute present |
| `theme-persists-after-reload` | Select `solarized`, reload → `data-theme="solarized"` | Persisted |
| `system-mode-respects-os` | Set `'system'`, mocked dark OS → `data-theme="dark"` | Attribute correct |
| `theme-gallery-visible` | Navigate to `/settings/appearance` → gallery visible | Cards rendered |
| `select-theme-from-gallery` | Click `high-contrast-dark` card → UI updates | `data-theme` changes |
| `preview-on-hover` | Hover over theme card → transient preview applies | `data-theme` changes |
| `preview-reverts-on-leave` | Leave hover → `data-theme` reverts | Original restored |
| `custom-color-picker-visible` | Select custom → picker section appears | Section visible |
| `export-downloads-json` | Click export → file download triggered | Download event |
| `logo-upload-applies` | Upload image → logo in sidebar updates | `src` attribute changes |

**DoD Checklist execution order:**
1. `npx playwright test --project=firefox tests/theme.spec.ts`
2. GORM Security Scan: `./scripts/scan-gorm-security.sh --check` (logo handler touches DB via settings)
3. `bash scripts/local-patch-report.sh`
4. `lefthook run pre-commit`
5. `make lint-fast` (backend)
6. `cd frontend && npm run type-check`
7. `scripts/go-test-coverage.sh` (≥85%)
8. `scripts/frontend-test-coverage.sh` (≥85%)
9. `cd backend && go build ./...`
10. `cd frontend && npm run build`
11. Inspect `frontend/dist/index.html` — stylesheet `<link>` must precede the inline `<script>`.

---

## 5. Acceptance Criteria

### AC-01 — FOUC Eliminated
The browser console must not emit "Layout was forced before the page was fully loaded" on any page load in Firefox. The warning must be absent on both initial load and navigation.

### AC-02 — Inline Script Positioned After Stylesheet
In the built `frontend/dist/index.html`, all `<link rel="stylesheet">` tags appear before the inline `<script>`. Verified by `grep -n 'stylesheet\|!function' frontend/dist/index.html`.

### AC-03 — `data-theme` Attribute (Not Class)
`<html>` receives `data-theme="dark"` (or other theme value) instead of `class="dark"`. The `.dark` and `.light` CSS class selectors are removed from `index.css`. Verified by `grep -n 'classList\|\.dark\|\.light' frontend/src/context/ThemeContext.tsx` returning no theme-manipulation matches.

### AC-04 — No `useLayoutEffect` in ThemeContext
`frontend/src/context/ThemeContext.tsx` does not import or use `useLayoutEffect`. Verified by grep.

### AC-05 — Five Built-In Themes Work
The following `data-theme` values produce distinct visible styling changes across the full UI:
- `dark` (default)
- `light`
- `high-contrast-dark`
- `high-contrast-light`
- `solarized`

### AC-06 — Follow System Mode
Setting theme to `system` makes the UI match the OS preference. Changing the OS preference updates the UI without a page reload.

### AC-07 — Theme Gallery UI
`/settings/appearance` renders a gallery of theme cards. Hovering previews the theme transiently. Clicking selects and persists it.

### AC-08 — Custom Color Picker
Selecting "Custom" in the gallery shows a color picker with at least 10 inputs (bg-base, bg-subtle, bg-muted, bg-elevated, border-default, border-strong, text-primary, text-secondary, text-muted, brand-primary). Changes apply to the UI in real time.

### AC-09 — Theme Import/Export
Export produces a valid `charon-theme.json` file. Importing that file on another Charon instance (or after clearing storage) restores the same theme and custom colors.

### AC-10 — Logo Customization
Admin can upload an image file (PNG, JPG, or WebP — max 2 MB; SVG is not accepted for upload) or provide an external URL (any format, including SVG, at the user's responsibility). The custom logo replaces the default Charon logo in the sidebar (both collapsed and expanded states) and mobile header. Reset to default clears the customization. Non-admin users see a read-only notice and cannot access the upload or URL form.

### AC-11 — Settings Tab Added
`/settings/appearance` is accessible as a new tab in the Settings page, visible to all authenticated non-passthrough users.

### AC-12 — Existing Themes Persist on Upgrade
Users with `localStorage['theme'] = 'dark'` or `'light'` from the old system have their preference preserved on upgrade. The inline script (canonical text in section 3.4) reads `localStorage.getItem('charon-theme') || localStorage.getItem('theme') || 'dark'`. This means:
- If `charon-theme` is set (returning user after upgrade), it takes precedence.
- If only the old `theme` key is set (first load after upgrade), it is used as the fallback so light-mode users are not reset to dark.
- If neither key is set (brand-new user), `dark` is the default.

The old `theme` key is left in storage (not deleted). `ThemeProvider` writes only to `charon-theme`. No data loss occurs.

### AC-13 — Type Safety
`cd frontend && npm run type-check` exits zero with no TypeScript errors.

### AC-14 — Test Coverage ≥ 85%
`scripts/go-test-coverage.sh` and `scripts/frontend-test-coverage.sh` both report ≥ 85%. All new files have accompanying unit tests.

### AC-15 — Build Succeeds
`cd backend && go build ./...` and `cd frontend && npm run build` both succeed with no errors.

### AC-16 — GORM Security Scan Clean
`./scripts/scan-gorm-security.sh --check` reports zero CRITICAL/HIGH findings for the new `logo_handler.go`.

### AC-17 — Logo Upload Security
The server detects the MIME type of uploaded files using `net/http.DetectContentType()` on the actual file bytes — the multipart `Content-Type` header is not trusted. Only `image/png`, `image/jpeg`, and `image/webp` are accepted for file uploads; SVG uploads are rejected with HTTP 400. Files exceeding 2 MB are rejected with HTTP 413. Files with a spoofed Content-Type (e.g., an SVG file claiming to be `image/png`) are detected and rejected based on byte content.

### AC-18 — Follow-Up Issue for Remaining `dark:` Class Migration
A GitHub issue is created to track the migration of all remaining 214+ `dark:` class usages (outside `Layout.tsx`) to semantic CSS custom property classes, enabling full theme support for `high-contrast-dark`, `solarized`, and future themes in every component. The issue is referenced in the PR description.

---

## 6. Commit Slicing Strategy

**Decision:** Single PR with six ordered logical commits. The PR is frontend-primary with one backend addition (logo upload). Commits are sized to be independently reviewable and reverted without cascading failures.

---

### Commit 1: `fix(theme): migrate CSS to data-theme selectors and update Tailwind config`

**Scope:** Pure CSS/config refactor — no behavior change for existing dark/light users. Foundation for all subsequent work.

**Files:**
- `frontend/src/index.css` — Replace `.light` with `[data-theme="light"]`; add `[data-theme="dark"]`, `[data-theme="high-contrast-dark"]`, `[data-theme="high-contrast-light"]`, `[data-theme="solarized"]`; update scrollbar CSS
- `frontend/tailwind.config.js` — `darkMode` → selector mode
- `frontend/src/components/Layout.tsx` — Migrate hardcoded dark: classes to semantic token classes

**Dependencies:** None.

**Validation gates:**
```bash
cd frontend && npm run build          # must succeed
cd frontend && npm run type-check     # zero errors
cd frontend && npm run test           # no regressions
```

**Rollback:** `git revert HEAD`. CSS reverts to class-based system. No functional impact on users still seeing `class="dark"`.

---

### Commit 2: `fix(theme): replace useLayoutEffect with useEffect + data-theme attribute; fix FOUC`

**Scope:** Core FOUC fix and theme system rewire. Replaces binary toggle with full ThemeId system.

**Files:**
- `frontend/index.html` — Move `<script>` to last in `<head>`; update to `data-theme` and `charon-theme` key
- `frontend/src/context/ThemeContextValue.ts` — Expanded types
- `frontend/src/context/ThemeContext.tsx` — Full rewrite
- `frontend/src/hooks/useTheme.ts` — Updated return type
- `frontend/src/components/ThemeToggle.tsx` — Navigate to appearance settings
- `frontend/src/context/__tests__/ThemeContext.test.tsx` — New unit tests (TC-01 through TC-13)
- `frontend/src/hooks/__tests__/useTheme.test.tsx` — Extended tests

**Dependencies:** Commit 1 (CSS selectors must exist before `data-theme` is applied).

**Validation gates:**
```bash
cd frontend && npm run build
grep -n 'stylesheet\|!function' frontend/dist/index.html   # stylesheet must appear first
cd frontend && npm run test        # all ThemeProvider tests pass
cd frontend && npm run type-check
lefthook run pre-commit
```

**Note:** Between Commit 2 and Commit 3, the `ThemeToggle` navigates to `/settings/appearance`, but the route has no component yet. This means clicking the theme toggle renders a blank `<Outlet>` — the Settings shell renders correctly, but the appearance tab content is empty. This is acceptable for the rollout window since these commits land in the same PR. Commit 3 must follow immediately.

**Rollback:** `git revert HEAD`. FOUC returns; binary toggle restores. Theme gallery not yet added so no orphaned routes.

---

### Commit 3: `feat(theme): add Settings Appearance tab, theme gallery, and preview`

**Scope:** Theme gallery UI — five built-in themes + system mode selectable from `/settings/appearance`.

**Files:**
- `frontend/src/components/theme/ThemeGallery.tsx`
- `frontend/src/components/theme/ThemeCard.tsx`
- `frontend/src/components/theme/ThemePreviewOverlay.tsx`
- `frontend/src/pages/AppearanceSettings.tsx` (gallery + system sections only)
- `frontend/src/pages/Settings.tsx` — Add Appearance tab
- `frontend/src/App.tsx` — Add appearance route
- `frontend/src/locales/*/translation.json` — New i18n keys
- `frontend/src/components/theme/__tests__/ThemeGallery.test.tsx`
- `frontend/src/components/theme/__tests__/ThemeCard.test.tsx`
- `frontend/src/components/theme/__tests__/ThemePreviewOverlay.test.tsx`
- `frontend/src/pages/__tests__/AppearanceSettings.test.tsx`
- `frontend/src/pages/__tests__/Settings.test.tsx` — Extend to cover new Appearance tab

**Dependencies:** Commit 2 (ThemeProvider must expose `setTheme` and `resolvedTheme`).

**Validation gates:**
```bash
cd frontend && npm run test
cd frontend && npm run type-check
cd frontend && npm run build
```

---

### Commit 4: `feat(theme): add custom color picker and theme import/export`

**Scope:** Custom theme creation and portability.

**Files:**
- `frontend/src/components/theme/CustomColorPicker.tsx`
- `frontend/src/components/theme/ThemeImportExport.tsx`
- `frontend/src/pages/AppearanceSettings.tsx` — Add custom + import/export sections
- `frontend/src/components/theme/__tests__/CustomColorPicker.test.tsx`
- `frontend/src/components/theme/__tests__/ThemeImportExport.test.tsx`
- `frontend/src/pages/__tests__/AppearanceSettings.test.tsx` — Extend

**Dependencies:** Commit 3 (AppearanceSettings page must exist).

**Validation gates:**
```bash
cd frontend && npm run test
cd frontend && npm run type-check
cd frontend && npm run build
scripts/frontend-test-coverage.sh      # ≥85%
```

---

### Commit 5: `feat(theme): add logo customization — backend upload endpoint and frontend UI`

**Scope:** Logo upload/URL feature. Only commit that touches the backend.

**Files:**
- `backend/internal/api/handlers/logo_handler.go`
- `backend/internal/api/handlers/logo_handler_test.go`
- `backend/internal/api/routes/routes.go` — Register `/settings/logo` routes
- `backend/internal/server/server.go` — Add `/uploads` static route; update `NewRouter`
- `backend/internal/server/server_test.go` — Update call site
- `frontend/src/api/settings.ts` — `uploadLogo`, `deleteLogo`
- `frontend/src/api/__tests__/settings.test.ts` — Extended
- `frontend/src/components/theme/LogoCustomizer.tsx`
- `frontend/src/components/theme/__tests__/LogoCustomizer.test.tsx`
- `frontend/src/pages/AppearanceSettings.tsx` — Add logo section
- `frontend/src/components/Layout.tsx` — Read `ui.logo_url`

**Dependencies:** Commit 3 (AppearanceSettings exists), Commit 1 (Layout.tsx already cleaned up).

**Validation gates:**
```bash
cd backend && go build ./...
cd backend && go test ./...
./scripts/scan-gorm-security.sh --check    # zero CRITICAL/HIGH
cd frontend && npm run test
cd frontend && npm run type-check
cd frontend && npm run build
scripts/go-test-coverage.sh               # ≥85%
lefthook run pre-commit
```

**Rollback:** `git revert HEAD`. Logo upload endpoint disappears; Layout falls back to `/logo.png` default. No data loss (uploaded file stays in `data/uploads/` but setting is cleared).

---

### Commit 6: `test(e2e): add Playwright theme system E2E tests + docs updates`

**Scope:** E2E test suite and documentation.

**Files:**
- `tests/theme.spec.ts` — New E2E tests
- `docs/features.md` — Expand "Dark Mode & Modern UI" section
- `ARCHITECTURE.md` — Note theme system in Frontend / State Management section

**Dependencies:** Commits 1–5 (tests exercise the full system).

**Validation gates:**
```bash
npx playwright test --project=firefox tests/theme.spec.ts
bash scripts/local-patch-report.sh
```

---

### PR-Level Contingency Notes

- **Rollback sequence:** Commits are ordered so each builds on the previous. To roll back to a partially shipped state, revert from the most recent commit backward. Commit 1 (CSS) can be kept while reverting 2–6 if needed.
- **Known limitation — `dark:` class migration scope:** This PR migrates only `Layout.tsx` away from hardcoded Tailwind `dark:` classes. The 214+ `dark:` usages in other components (e.g., `CSPBuilder.tsx`, `NotificationCenter.tsx`, etc.) are out of scope. `high-contrast-dark` and `solarized` themes will have mixed styling in those components — they will render with their light-mode fallback styles. A separate PR will complete the migration. See AC-18.
- **React 18 Strict Mode:** The `useRef(true)` first-render skip will fire once in dev (double-invoke), then `false`. Tests must wrap `ThemeProvider` without `React.StrictMode` or account for the double-invoke when asserting `setAttribute` call counts.
- **localStorage migration:** The canonical inline script is specified in section 3.4 and reads `charon-theme` first, falls back to the old `'theme'` key, then defaults to `'dark'`. Section 3.4 is the single source of truth for the script — do not diverge from it. See AC-12 for the behavior contract.
- **Layout.tsx class migration risk:** Layout.tsx is the highest-risk file in Commit 1 — it has ~30 hardcoded dark: class occurrences. The implementation must verify each migrated class has a corresponding CSS custom property mapping in Tailwind config before committing. A visual regression test (screenshot comparison) is strongly recommended.
- **Backend `NewRouter` signature change:** `server.go` `NewRouter` gains a `dataDir string` parameter. Pass `filepath.Dir(cfg.DatabasePath)` at the call site in `backend/cmd/api/main.go` — do NOT add a new `DataDir` config field. See section 3.9 for the exact derivation. The implementation agent must grep for all `NewRouter` call sites.
- **No new npm packages** are introduced. All components use existing React, Lucide icons, and the existing `@testing-library/react` + Vitest stack.
- **GORM Security Scan** is required for Commit 5 because `logo_handler.go` calls GORM to save settings.
