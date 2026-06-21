import { Download, Upload } from 'lucide-react'
import { useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { useTheme } from '../../hooks/useTheme'
import { toast } from '../../utils/toast'
import { Button } from '../ui/Button'

import type { ThemeExport } from '../../context/ThemeContextValue'

// Valid ThemeId values — kept in sync with ThemeContextValue.ts
const VALID_THEME_IDS = [
  'dark',
  'light',
  'high-contrast-dark',
  'high-contrast-light',
  'solarized',
  'system',
  'custom',
] as const

// RGB color value pattern: three integers 0–255 separated by spaces
const RGB_PATTERN = /^\d{1,3} \d{1,3} \d{1,3}$/

function isValidThemeExport(data: unknown): data is ThemeExport {
  if (typeof data !== 'object' || data === null) return false
  const d = data as Record<string, unknown>

  // version must be the literal 1
  if (d.version !== 1) return false
  if (typeof d.exportedAt !== 'string') return false
  // theme must be a known ThemeId — prevents unknown theme injection
  if (!VALID_THEME_IDS.includes(d.theme as (typeof VALID_THEME_IDS)[number])) return false

  // If a custom theme is present, validate all color fields to prevent CSS injection
  if (d.customTheme !== undefined && d.customTheme !== null) {
    const ct = d.customTheme as Record<string, unknown>
    if (typeof ct !== 'object') return false
    if (ct.colors !== undefined) {
      const colors = ct.colors as Record<string, unknown>
      const colorFields = [
        'bgBase',
        'bgSubtle',
        'bgMuted',
        'bgElevated',
        'borderDefault',
        'borderStrong',
        'textPrimary',
        'textSecondary',
        'textMuted',
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

export function ThemeImportExport() {
  const { t } = useTranslation()
  const { exportTheme, importTheme } = useTheme()
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleExport = () => {
    const data = exportTheme()
    const json = JSON.stringify(data, null, 2)
    const blob = new Blob([json], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'charon-theme.json'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  const handleImportClick = () => {
    fileInputRef.current?.click()
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    const reader = new FileReader()
    reader.onload = (event) => {
      try {
        const text = event.target?.result
        if (typeof text !== 'string') {
          toast.error(t('appearance.importError'))
          return
        }
        const parsed: unknown = JSON.parse(text)
        if (!isValidThemeExport(parsed)) {
          toast.error(t('appearance.importError'))
          return
        }
        importTheme(parsed)
      } catch {
        toast.error(t('appearance.importError'))
      }
    }
    reader.readAsText(file)

    // Reset so the same file can be re-imported
    e.target.value = ''
  }

  return (
    <div className="flex flex-wrap gap-3">
      <Button
        variant="outline"
        onClick={handleExport}
        aria-label={t('appearance.exportButton')}
      >
        <Download className="mr-2 h-4 w-4" />
        {t('appearance.exportButton')}
      </Button>

      <Button
        variant="outline"
        onClick={handleImportClick}
        aria-label={t('appearance.importButton')}
      >
        <Upload className="mr-2 h-4 w-4" />
        {t('appearance.importButton')}
      </Button>

      <input
        ref={fileInputRef}
        type="file"
        accept=".json"
        onChange={handleFileChange}
        className="sr-only"
        aria-hidden="true"
        tabIndex={-1}
        data-testid="theme-file-input"
      />
    </div>
  )
}
