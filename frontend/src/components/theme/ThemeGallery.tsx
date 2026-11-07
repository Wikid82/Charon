import { useTranslation } from 'react-i18next'

import {
  BUILT_IN_THEMES,
  BUILT_IN_THEME_LABELS,
  type ThemeId,
} from '../../context/ThemeContextValue'
import { ThemeCard } from './ThemeCard'

export interface ThemeGalleryProps {
  value: ThemeId
  previewTheme: ThemeId | null
  onChange: (theme: ThemeId) => void
  onPreview: (theme: ThemeId | null) => void
}

export function ThemeGallery({
  value,
  previewTheme,
  onChange,
  onPreview,
}: ThemeGalleryProps) {
  const { t } = useTranslation()

  // All selectable themes: 5 built-ins + system
  const themes: ThemeId[] = [...BUILT_IN_THEMES, 'system']

  const themeLabels: Record<ThemeId, string> = {
    ...BUILT_IN_THEME_LABELS,
    system: t('appearance.followSystem'),
    custom: t('appearance.customTheme'),
  }

  return (
    <div
      role="radiogroup"
      aria-label={t('appearance.themeGallery')}
      className="grid grid-cols-2 gap-3 sm:grid-cols-3"
    >
      {themes.map((themeId) => (
        <ThemeCard
          key={themeId}
          themeId={themeId}
          label={themeLabels[themeId] ?? themeId}
          selected={value === themeId}
          onSelect={() => onChange(themeId)}
          onPreview={() => onPreview(themeId)}
          onPreviewEnd={() => onPreview(null)}
        />
      ))}
      {/* Show preview indicator when hovering */}
      {previewTheme !== null && previewTheme !== value && (
        <span className="sr-only" aria-live="polite">
          {t('appearance.themeGallery')}: {themeLabels[previewTheme] ?? previewTheme}
        </span>
      )}
    </div>
  )
}
