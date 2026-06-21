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
