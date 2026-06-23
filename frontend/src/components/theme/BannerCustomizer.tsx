import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useAuth } from '../../hooks/useAuth'
import { Button } from '../ui/Button'
import { Input } from '../ui/Input'
import { Label } from '../ui/Label'

const MAX_BANNER_SIZE_BYTES = 2 * 1024 * 1024 // 2 MB
const ACCEPTED_MIME_TYPES = 'image/png,image/jpeg,image/webp'

function isValidBannerUrl(url: string): boolean {
  return url.startsWith('https://')
}

export interface BannerCustomizerProps {
  currentBannerUrl: string | null
  onUpload: (file: File) => void
  onUrlSave: (url: string) => void
  onReset: () => void
  isSaving: boolean
}

type ActiveTab = 'upload' | 'url'

export function BannerCustomizer({
  currentBannerUrl,
  onUpload,
  onUrlSave,
  onReset,
  isSaving,
}: BannerCustomizerProps) {
  const { t } = useTranslation()
  const { user } = useAuth()
  const [activeTab, setActiveTab] = useState<ActiveTab>('upload')
  const [urlInput, setUrlInput] = useState('')
  const [fileError, setFileError] = useState<string | null>(null)
  const [urlError, setUrlError] = useState<string | null>(null)

  const isAdmin = user?.role === 'admin'

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFileError(null)
    const file = e.target.files?.[0]
    if (!file) return

    if (file.size > MAX_BANNER_SIZE_BYTES) {
      setFileError(t('appearance.bannerUploadHint'))
      e.target.value = ''
      return
    }

    // Client-side MIME type check (server also validates with byte sniffing)
    const accepted = ['image/png', 'image/jpeg', 'image/webp']
    if (!accepted.includes(file.type)) {
      setFileError(t('appearance.bannerUploadHint'))
      e.target.value = ''
      return
    }

    onUpload(file)
    e.target.value = ''
  }

  const handleUrlSave = () => {
    setUrlError(null)
    const trimmed = urlInput.trim()
    if (!trimmed) return
    if (!isValidBannerUrl(trimmed)) {
      setUrlError(t('appearance.bannerUrlHttpsRequired'))
      return
    }
    onUrlSave(trimmed)
  }

  if (!isAdmin) {
    return (
      <div className="space-y-4">
        {/* Banner preview - always visible */}
        {currentBannerUrl && (
          <div>
            <p className="text-sm font-medium text-content-secondary mb-2">
              {t('appearance.bannerPreview')}
            </p>
            <img
              src={currentBannerUrl}
              alt="Banner preview"
              className="max-w-full h-16 object-contain rounded border border-border p-2"
            />
          </div>
        )}
        <p className="text-sm text-content-muted">
          Banner customization requires admin access
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Banner preview */}
      {currentBannerUrl && (
        <div>
          <p className="text-sm font-medium text-content-secondary mb-2">
            {t('appearance.bannerPreview')}
          </p>
          <img
            src={currentBannerUrl}
            alt="Banner preview"
            className="max-w-full h-16 object-contain rounded border border-border p-2"
          />
        </div>
      )}

      {/* Tab switcher */}
      <div className="flex gap-2 border-b border-border">
        <button
          type="button"
          onClick={() => setActiveTab('upload')}
          className={`pb-2 px-1 text-sm font-medium transition-colors border-b-2 -mb-px ${
            activeTab === 'upload'
              ? 'border-brand-500 text-brand-500'
              : 'border-transparent text-content-secondary hover:text-content-primary'
          }`}
        >
          {t('appearance.bannerUploadTab')}
        </button>
        <button
          type="button"
          onClick={() => setActiveTab('url')}
          className={`pb-2 px-1 text-sm font-medium transition-colors border-b-2 -mb-px ${
            activeTab === 'url'
              ? 'border-brand-500 text-brand-500'
              : 'border-transparent text-content-secondary hover:text-content-primary'
          }`}
        >
          {t('appearance.bannerUrlTab')}
        </button>
      </div>

      {/* Upload tab */}
      {activeTab === 'upload' && (
        <div className="space-y-2">
          <Label htmlFor="banner-file-input">
            {t('appearance.bannerUploadTab')}
          </Label>
          <input
            id="banner-file-input"
            type="file"
            accept={ACCEPTED_MIME_TYPES}
            onChange={handleFileChange}
            disabled={isSaving}
            className="block w-full text-sm text-content-secondary
              file:mr-4 file:py-2 file:px-4
              file:rounded file:border-0
              file:text-sm file:font-medium
              file:bg-brand-500/10 file:text-brand-500
              hover:file:bg-brand-500/20"
          />
          {fileError ? (
            <p className="text-xs text-error">{fileError}</p>
          ) : (
            <p className="text-xs text-content-muted">{t('appearance.bannerUploadHint')}</p>
          )}
        </div>
      )}

      {/* URL tab */}
      {activeTab === 'url' && (
        <div className="space-y-2">
          <Label htmlFor="banner-url-input">
            {t('appearance.bannerUrlTab')}
          </Label>
          <div className="flex gap-2">
            <Input
              id="banner-url-input"
              type="url"
              pattern="https://.*"
              value={urlInput}
              onChange={(e) => { setUrlInput(e.target.value); setUrlError(null) }}
              placeholder={t('appearance.bannerUrlPlaceholder')}
              disabled={isSaving}
              className="flex-1"
            />
            <Button
              variant="primary"
              size="sm"
              onClick={handleUrlSave}
              disabled={isSaving || !urlInput.trim()}
            >
              {t('appearance.bannerSaveButton')}
            </Button>
          </div>
          {urlError && (
            <p className="text-xs text-error">{urlError}</p>
          )}
        </div>
      )}

      {/* Reset to default */}
      <div className="pt-2 border-t border-border">
        <Button
          variant="ghost"
          size="sm"
          onClick={onReset}
          disabled={isSaving}
          className="text-content-muted hover:text-content-primary"
        >
          {t('appearance.bannerResetButton')}
        </Button>
      </div>
    </div>
  )
}
