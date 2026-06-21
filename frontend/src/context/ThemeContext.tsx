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
