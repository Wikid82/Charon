import { useEffect } from 'react'

import { isUserThemeId } from '../../context/ThemeContextValue'
import type { DataThemeValue, ThemeId } from '../../context/ThemeContextValue'

export interface ThemePreviewOverlayProps {
  previewTheme: ThemeId | null
  resolvedCurrentTheme: DataThemeValue
}

// Resolves a ThemeId to a DataThemeValue for DOM attribute use.
// Mirrors the logic in ThemeContext — kept local to avoid import coupling.
function resolveDataTheme(theme: ThemeId): DataThemeValue {
  if (theme === 'system') {
    if (typeof window === 'undefined') return 'dark'
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
  }
  if (theme === 'custom') return 'custom'
  if (isUserThemeId(theme)) return 'custom'
  return theme
}

/**
 * ThemePreviewOverlay — renders no UI.
 * Temporarily applies a preview data-theme to <html> when previewTheme is non-null.
 * Restores resolvedCurrentTheme when previewTheme becomes null.
 */
export function ThemePreviewOverlay({
  previewTheme,
  resolvedCurrentTheme,
}: ThemePreviewOverlayProps) {
  useEffect(() => {
    if (previewTheme !== null) {
      document.documentElement.setAttribute('data-theme', resolveDataTheme(previewTheme))
    } else {
      document.documentElement.setAttribute('data-theme', resolvedCurrentTheme)
    }
  }, [previewTheme, resolvedCurrentTheme])

  return null
}
