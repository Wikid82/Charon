import { useTranslation } from 'react-i18next'

import { Label } from '../ui/Label'

import type { CustomThemeColors } from '../../context/ThemeContextValue'

export interface CustomColorPickerProps {
  value: CustomThemeColors
  onChange: (colors: CustomThemeColors) => void
}

/**
 * Converts a hex color string to RGB space-separated format.
 * hexToRgb('#3b82f6') → '59 130 246'
 */
// eslint-disable-next-line react-refresh/only-export-components
export function hexToRgb(hex: string): string {
  const cleaned = hex.replace('#', '')
  const r = parseInt(cleaned.substring(0, 2), 16)
  const g = parseInt(cleaned.substring(2, 4), 16)
  const b = parseInt(cleaned.substring(4, 6), 16)
  return `${r} ${g} ${b}`
}

/**
 * Converts RGB space-separated format to a hex color string.
 * rgbToHex('59 130 246') → '#3b82f6'
 */
// eslint-disable-next-line react-refresh/only-export-components
export function rgbToHex(rgb: string): string {
  const parts = rgb.trim().split(/\s+/)
  if (parts.length !== 3) return '#000000'
  const [r, g, b] = parts.map(Number)
  const toHex = (n: number) => Math.max(0, Math.min(255, n)).toString(16).padStart(2, '0')
  return `#${toHex(r)}${toHex(g)}${toHex(b)}`
}

type ColorField = keyof Omit<CustomThemeColors, 'colorScheme'>

const COLOR_FIELDS: { field: ColorField; labelKey: string }[] = [
  { field: 'bgBase', labelKey: 'appearance.colorPickerBgBase' },
  { field: 'bgSubtle', labelKey: 'appearance.colorPickerBgSubtle' },
  { field: 'bgMuted', labelKey: 'appearance.colorPickerBgMuted' },
  { field: 'bgElevated', labelKey: 'appearance.colorPickerBgElevated' },
  { field: 'borderDefault', labelKey: 'appearance.colorPickerBorderDefault' },
  { field: 'borderStrong', labelKey: 'appearance.colorPickerBorderStrong' },
  { field: 'textPrimary', labelKey: 'appearance.colorPickerTextPrimary' },
  { field: 'textSecondary', labelKey: 'appearance.colorPickerTextSecondary' },
  { field: 'textMuted', labelKey: 'appearance.colorPickerTextMuted' },
  { field: 'brandPrimary', labelKey: 'appearance.colorPickerBrandPrimary' },
]

export function CustomColorPicker({ value, onChange }: CustomColorPickerProps) {
  const { t } = useTranslation()

  const handleColorChange = (field: ColorField, hex: string) => {
    onChange({ ...value, [field]: hexToRgb(hex) })
  }

  const handleSchemeChange = (scheme: 'dark' | 'light') => {
    onChange({ ...value, colorScheme: scheme })
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
        {COLOR_FIELDS.map(({ field, labelKey }) => (
          <div key={field} className="flex flex-col gap-1">
            <Label htmlFor={`color-${field}`}>{t(labelKey)}</Label>
            <input
              id={`color-${field}`}
              type="color"
              value={rgbToHex(value[field])}
              onChange={(e) => handleColorChange(field, e.target.value)}
              className="h-10 w-full cursor-pointer rounded border border-border bg-surface-base p-1"
              aria-label={t(labelKey)}
            />
          </div>
        ))}
      </div>

      <div className="flex flex-col gap-1">
        <Label htmlFor="color-scheme">{t('appearance.colorPickerColorScheme')}</Label>
        <select
          id="color-scheme"
          value={value.colorScheme}
          onChange={(e) => handleSchemeChange(e.target.value as 'dark' | 'light')}
          className="rounded border border-border bg-surface-base px-3 py-2 text-sm text-content-primary"
          aria-label={t('appearance.colorPickerColorScheme')}
        >
          <option value="dark">Dark</option>
          <option value="light">Light</option>
        </select>
      </div>
    </div>
  )
}
