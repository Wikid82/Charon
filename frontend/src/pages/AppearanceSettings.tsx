import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowUpDown, Bell, ImageIcon, Palette, Sliders } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { deleteBanner, deleteLogo, getSettings, updateSetting, uploadBanner, uploadLogo } from '../api/settings'
import WhatsNewModal from '../components/dialogs/WhatsNewModal'
import { BannerCustomizer } from '../components/theme/BannerCustomizer'
import { CustomColorPicker } from '../components/theme/CustomColorPicker'
import { LogoCustomizer } from '../components/theme/LogoCustomizer'
import { ThemeGallery } from '../components/theme/ThemeGallery'
import { ThemeImportExport } from '../components/theme/ThemeImportExport'
import { ThemePreviewOverlay } from '../components/theme/ThemePreviewOverlay'
import { UserThemeManager } from '../components/theme/UserThemeManager'
import { Button } from '../components/ui/Button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card'
import { Checkbox } from '../components/ui/Checkbox'
import { Label } from '../components/ui/Label'
import { useAuth } from '../hooks/useAuth'
import { useAckChangelog, useOptInChangelog } from '../hooks/useChangelog'
import { useTheme } from '../hooks/useTheme'

import type { CustomThemeColors, ThemeId } from '../context/ThemeContextValue'

// Default colors based on the dark theme values from Section 2.7 of the plan
const DARK_THEME_DEFAULTS: CustomThemeColors = {
  bgBase: '15 23 42',
  bgSubtle: '30 41 59',
  bgMuted: '51 65 85',
  bgElevated: '30 41 59',
  borderDefault: '51 65 85',
  borderStrong: '71 85 105',
  textPrimary: '248 250 252',
  textSecondary: '203 213 225',
  textMuted: '148 163 184',
  brandPrimary: '59 130 246',
  colorScheme: 'dark',
}

export default function AppearanceSettings() {
  const { t } = useTranslation()
  const { theme, resolvedTheme, setTheme, customTheme, setCustomTheme, setUserTheme } = useTheme()
  const [previewTheme, setPreviewTheme] = useState<ThemeId | null>(null)
  const queryClient = useQueryClient()
  const { user, refetchUser } = useAuth()
  const [browseChangelogOpen, setBrowseChangelogOpen] = useState(false)

  const ackChangelogMutation = useAckChangelog()
  const optInChangelogMutation = useOptInChangelog()

  const handleWhatsNewToggle = (enabled: boolean) => {
    if (enabled) {
      optInChangelogMutation.mutate(undefined, { onSuccess: () => { void refetchUser() } })
    } else {
      ackChangelogMutation.mutate(
        { action: 'dismiss_temporary', opt_out: true },
        { onSuccess: () => { void refetchUser() } }
      )
    }
  }

  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
    staleTime: 1000 * 60 * 5,
  })

  const currentLogoUrl = settings?.['ui.logo_url'] ?? null
  const currentBannerUrl = settings?.['ui.banner_url'] ?? null

  const uploadLogoMutation = useMutation({
    mutationFn: uploadLogo,
    onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['settings'] }) },
  })

  const saveUrlMutation = useMutation({
    mutationFn: async (url: string) => {
      await updateSetting('ui.logo_url', url, 'ui', 'string')
      await updateSetting('ui.logo_type', 'url', 'ui', 'string')
    },
    onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['settings'] }) },
  })

  const deleteLogoMutation = useMutation({
    mutationFn: deleteLogo,
    onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['settings'] }) },
  })

  const isSavingLogo =
    uploadLogoMutation.isPending ||
    saveUrlMutation.isPending ||
    deleteLogoMutation.isPending

  const uploadBannerMutation = useMutation({
    mutationFn: uploadBanner,
    onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['settings'] }) },
  })

  // NOTE: saveBannerUrlMutation stores the URL as a plain setting value.
  // It does NOT perform a server-side fetch of the URL, so it bypasses MIME/size
  // enforcement. Client-side https:// validation in BannerCustomizer.tsx is the
  // security boundary for this code path. Server-side url-scheme validation is a
  // future enhancement.
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

  const handleThemeChange = (newTheme: ThemeId) => {
    setPreviewTheme(null)
    setTheme(newTheme)
  }

  const customPickerValue: CustomThemeColors = customTheme?.colors ?? DARK_THEME_DEFAULTS

  const handleCustomColorsChange = (colors: CustomThemeColors) => {
    setCustomTheme(colors)
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

      {/* Custom Theme Section — only shown when custom theme is selected */}
      {theme === 'custom' && (
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <Sliders className="h-5 w-5 text-content-secondary" />
              <CardTitle>{t('appearance.customTheme')}</CardTitle>
            </div>
            <CardDescription>{t('appearance.customThemeDescription')}</CardDescription>
          </CardHeader>
          <CardContent>
            <CustomColorPicker
              value={customPickerValue}
              onChange={handleCustomColorsChange}
            />
          </CardContent>
        </Card>
      )}

      {/* Import / Export Section — always shown */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <ArrowUpDown className="h-5 w-5 text-content-secondary" />
            <CardTitle>{t('appearance.importExport')}</CardTitle>
          </div>
          <CardDescription>{t('appearance.importExportDescription')}</CardDescription>
        </CardHeader>
        <CardContent>
          <ThemeImportExport />
        </CardContent>
      </Card>

      {/* Logo Customization Section */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <ImageIcon className="h-5 w-5 text-content-secondary" />
            <CardTitle>{t('appearance.logoCustomization')}</CardTitle>
          </div>
          <CardDescription>{t('appearance.logoCustomizationDescription')}</CardDescription>
        </CardHeader>
        <CardContent>
          <LogoCustomizer
            currentLogoUrl={currentLogoUrl}
            onUpload={(file) => uploadLogoMutation.mutate(file)}
            onUrlSave={(url) => saveUrlMutation.mutate(url)}
            onReset={() => deleteLogoMutation.mutate()}
            isSaving={isSavingLogo}
          />
        </CardContent>
      </Card>

      {/* Banner Customization Section */}
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

      {/* What's New Notifications Section */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Bell className="h-5 w-5 text-content-secondary" />
            <CardTitle>{t('appearance.whatsNew')}</CardTitle>
          </div>
          <CardDescription>{t('appearance.whatsNewDescription')}</CardDescription>
        </CardHeader>
        <CardContent className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Checkbox
              id="whats-new-toggle"
              checked={!user?.changelog_opt_out}
              onCheckedChange={(checked) => handleWhatsNewToggle(checked === true)}
            />
            <Label htmlFor="whats-new-toggle">{t('appearance.showWhatsNewToggle')}</Label>
          </div>
          <Button variant="secondary" onClick={() => setBrowseChangelogOpen(true)}>
            {t('appearance.whatsNewRevisit')}
          </Button>
        </CardContent>
      </Card>

      <WhatsNewModal
        mode="browse"
        open={browseChangelogOpen}
        onClose={() => setBrowseChangelogOpen(false)}
      />
    </div>
  )
}
