import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useRestoreBackup, useValidateBackup, type BackupFile } from '../../hooks/useBackups'
import { toast } from '../../utils/toast'
import { Alert, Button, Dialog, DialogContent, DialogHeader, DialogFooter, DialogTitle, Input } from '../ui'

export interface RestoreDialogProps {
  backup: BackupFile | null
  onClose: () => void
}

/**
 * Restore confirmation dialog, extracted from the previous inline dialog in
 * Backups.tsx (spec §3.8). Adds a passphrase input shown only when the
 * filename ends in `.age`, calls validate first and surfaces
 * `legacy_format` / `encryption_key_required` warnings, and reports the
 * `restart_required` / `database_swap_pending` outcome once restore completes.
 */
export function RestoreDialog(props: RestoreDialogProps) {
  // Keyed by target so the passphrase and mutation state reset whenever a different
  // backup is targeted (replaces a reset-in-effect). The key is the filename, so
  // re-renders of the same backup keep their state.
  return <RestoreDialogBody key={props.backup?.filename ?? ''} {...props} />
}

function RestoreDialogBody({ backup, onClose }: RestoreDialogProps) {
  const { t } = useTranslation()
  const [passphrase, setPassphrase] = useState('')
  const validateMutation = useValidateBackup()
  const restoreMutation = useRestoreBackup()

  const isEncrypted = backup?.filename.endsWith('.age') ?? false

  const handleValidate = () => {
    if (!backup) return
    validateMutation.mutate(
      { filename: backup.filename, passphrase: isEncrypted ? passphrase : undefined },
      { onError: (error: Error) => toast.error(error.message) }
    )
  }

  const handleRestore = () => {
    if (!backup) return
    restoreMutation.mutate(
      { filename: backup.filename, passphrase: isEncrypted ? passphrase : undefined },
      {
        onSuccess: (result) => {
          toast.success(
            result.restart_required
              ? t('backups.restoreWarnings.restartRequired')
              : t('backups.restoreSuccess')
          )
          onClose()
        },
        onError: (error: Error) => toast.error(t('backups.restoreFailed', { error: error.message })),
      }
    )
  }

  const actionsDisabled = isEncrypted && passphrase.length === 0

  return (
    <Dialog open={backup !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('backups.restoreBackup')}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 px-6 pb-2">
          <p className="text-content-secondary">{t('backups.restoreConfirmMessage')}</p>

          {isEncrypted && (
            <Input
              id="backup-restore-passphrase"
              data-testid="backup-restore-passphrase-input"
              type="password"
              label={t('backups.encryption.passphraseLabel')}
              value={passphrase}
              onChange={(e) => setPassphrase(e.target.value)}
              autoComplete="off"
            />
          )}

          {restoreMutation.isPending && restoreMutation.job?.stage && (
            <p className="text-sm text-content-secondary" data-testid="backup-restore-stage">
              {t(`backups.stage.${restoreMutation.job.stage}`)}
            </p>
          )}

          {validateMutation.data && (
            <Alert
              variant={validateMutation.data.valid ? 'info' : 'error'}
              data-testid="backup-restore-validate-results"
            >
              <p>
                {t('backups.validate.databaseIntegrity', {
                  status: validateMutation.data.database_integrity,
                })}
              </p>
              {validateMutation.data.legacy_format && <p>{t('backups.restoreWarnings.legacyFormat')}</p>}
              {validateMutation.data.encryption_key_required && (
                <p>{t('backups.restoreWarnings.encryptionKeyRequired')}</p>
              )}
              {validateMutation.data.warnings?.map((warning) => (
                <p key={warning}>{warning}</p>
              ))}
            </Alert>
          )}
        </div>
        <DialogFooter>
          <Button variant="secondary" onClick={onClose} disabled={restoreMutation.isPending}>
            {t('common.cancel')}
          </Button>
          <Button
            variant="secondary"
            onClick={handleValidate}
            isLoading={validateMutation.isPending}
            disabled={actionsDisabled}
          >
            {t('backups.validate.action')}
          </Button>
          <Button
            variant="primary"
            onClick={handleRestore}
            isLoading={restoreMutation.isPending}
            disabled={actionsDisabled}
          >
            {t('backups.restore')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
