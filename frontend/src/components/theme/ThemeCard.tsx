import { Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { ThemeId } from '../../context/ThemeContextValue'
import { cn } from '../../utils/cn'

// Mini swatch colors per theme: [bg, brand, text]
const THEME_SWATCHES: Record<ThemeId, [string, string, string]> = {
  dark: ['rgb(15 23 42)', 'rgb(59 130 246)', 'rgb(248 250 252)'],
  light: ['rgb(248 250 252)', 'rgb(59 130 246)', 'rgb(15 23 42)'],
  'high-contrast-dark': ['rgb(0 0 0)', 'rgb(0 217 255)', 'rgb(255 255 255)'],
  'high-contrast-light': ['rgb(255 255 255)', 'rgb(0 86 179)', 'rgb(0 0 0)'],
  solarized: ['rgb(0 43 54)', 'rgb(38 139 210)', 'rgb(131 148 150)'],
  system: ['rgb(15 23 42)', 'rgb(59 130 246)', 'rgb(248 250 252)'],
  custom: ['rgb(15 23 42)', 'rgb(59 130 246)', 'rgb(248 250 252)'],
}

export interface ThemeCardProps {
  themeId: ThemeId
  label: string
  selected: boolean
  onSelect: () => void
  onPreview: () => void
  onPreviewEnd: () => void
}

export function ThemeCard({
  themeId,
  label,
  selected,
  onSelect,
  onPreview,
  onPreviewEnd,
}: ThemeCardProps) {
  const { t } = useTranslation()
  const [bgColor, brandColor, textColor] = THEME_SWATCHES[themeId] ?? THEME_SWATCHES.dark

  return (
    <div
      role="radio"
      aria-checked={selected}
      tabIndex={0}
      onClick={onSelect}
      onMouseEnter={onPreview}
      onMouseLeave={onPreviewEnd}
      onFocus={onPreview}
      onBlur={onPreviewEnd}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onSelect()
        }
      }}
      className={cn(
        'relative cursor-pointer rounded-lg border-2 p-3 transition-all duration-fast',
        'focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2 focus-visible:ring-offset-surface-base',
        selected
          ? 'border-brand-500 shadow-md'
          : 'border-border hover:border-border-strong hover:shadow-sm'
      )}
      aria-label={selected ? t('appearance.themeGallery') + ': ' + label + ' (' + t('common.enabled') + ')' : label}
    >
      {/* Mini color swatches */}
      <div className="mb-2 flex h-10 overflow-hidden rounded-md border border-border">
        <div
          className="flex-1"
          style={{ backgroundColor: bgColor }}
          aria-hidden="true"
        />
        <div
          className="w-4"
          style={{ backgroundColor: brandColor }}
          aria-hidden="true"
        />
        <div
          className="w-6 flex items-center justify-center"
          style={{ backgroundColor: bgColor }}
          aria-hidden="true"
        >
          <div
            className="h-1.5 w-4 rounded-full opacity-80"
            style={{ backgroundColor: textColor }}
          />
        </div>
      </div>

      {/* Label */}
      <p className="text-xs font-medium text-content-primary leading-tight">{label}</p>

      {/* Selected indicator */}
      {selected && (
        <div
          className="absolute top-1.5 right-1.5 flex h-4 w-4 items-center justify-center rounded-full bg-brand-500"
          aria-hidden="true"
        >
          <Check className="h-2.5 w-2.5 text-white" />
        </div>
      )}
    </div>
  )
}
