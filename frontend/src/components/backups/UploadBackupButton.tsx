import { Upload } from 'lucide-react'
import { useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { useUploadBackup } from '../../hooks/useBackups'
import { Button } from '../ui'
import { toast } from '../../utils/toast'

/**
 * File picker accepting `.zip,.age,.db` (spec §3.3.2, §3.8). Uploads
 * immediately on selection, shows validate feedback (format version, legacy
 * format, encryption-key-required), then the uploaded backup appears in the
 * backup table via the `useBackups()` query invalidation on success.
 */
export function UploadBackupButton() {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement>(null)
  const uploadMutation = useUploadBackup()

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    // Allow re-selecting the same filename later regardless of outcome.
    e.target.value = ''
    if (!file) return

    uploadMutation.mutate(
      { file },
      {
        onError: (error: Error) => toast.error(error.message),
      }
    )
  }

  const result = uploadMutation.data

  return (
    <div className="flex flex-col gap-2">
      <Button variant="secondary" onClick={() => inputRef.current?.click()} isLoading={uploadMutation.isPending}>
        <Upload className="w-4 h-4 mr-2" />
        {t('backups.upload.button')}
      </Button>
      <input
        ref={inputRef}
        type="file"
        accept=".zip,.age,.db"
        data-testid="backup-upload-input"
        className="sr-only"
        onChange={handleChange}
      />

      {result && (
        <p data-testid="backup-upload-feedback" className="text-sm text-content-secondary">
          {result.legacy_format ? t('backups.upload.legacyFormatWarning') : t('backups.upload.formatVersion2')}
          {result.encryption_key_required && <> {t('backups.upload.encryptionKeyRequiredWarning')}</>}
        </p>
      )}
    </div>
  )
}
