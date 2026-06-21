import { Palette } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ThemeGallery } from '../components/theme/ThemeGallery'
import { ThemePreviewOverlay } from '../components/theme/ThemePreviewOverlay'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card'
import type { ThemeId } from '../context/ThemeContextValue'
import { useTheme } from '../hooks/useTheme'

export default function AppearanceSettings() {
  const { t } = useTranslation()
  const { theme, resolvedTheme, setTheme } = useTheme()
  const [previewTheme, setPreviewTheme] = useState<ThemeId | null>(null)

  const handleThemeChange = (newTheme: ThemeId) => {
    setPreviewTheme(null)
    setTheme(newTheme)
  }

  return (
    <div className="space-y-6">
      <ThemePreviewOverlay
        previewTheme={previewTheme}
        resolvedCurrentTheme={resolvedTheme}
      />

      {/* Theme Gallery Section */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Palette className="h-5 w-5 text-content-secondary" />
            <CardTitle>{t('appearance.themeGallery')}</CardTitle>
          </div>
          <CardDescription>{t('appearance.themeGalleryDescription')}</CardDescription>
        </CardHeader>
        <CardContent>
          <ThemeGallery
            value={theme}
            previewTheme={previewTheme}
            onChange={handleThemeChange}
            onPreview={setPreviewTheme}
          />

          {/* Follow System explanation shown when system is selected */}
          {theme === 'system' && (
            <p className="mt-4 text-sm text-content-secondary">
              {t('appearance.followSystemDescription')}
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
