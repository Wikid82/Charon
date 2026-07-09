import { ShieldAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { UseBackupSettingsFormResult } from '../../hooks/useBackups'
import { Alert, Card, CardHeader, CardTitle, CardContent, Input, Label, Switch } from '../ui'

export interface BackupEncryptionCardProps {
  form: UseBackupSettingsFormResult
}

/**
 * Enable toggle + write-only passphrase set/change flow (spec §3.6, §3.8).
 * Encryption is off by default. Shares `form` (and its single "Save
 * Schedule" save action, see `BackupScheduleCard`) with the schedule card
 * since both map to the same `BackupSettings` resource — this card has no
 * save button of its own.
 */
export function BackupEncryptionCard({ form }: BackupEncryptionCardProps) {
  const { t } = useTranslation()

  return (
    <Card data-testid="backup-encryption-card">
      <CardHeader>
        <CardTitle>{t('backups.encryption.title')}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center gap-3">
          <Switch
            id="backup-encryption-enabled"
            data-testid="backup-encryption-toggle"
            checked={form.encryptionEnabled}
            onCheckedChange={form.setEncryptionEnabled}
          />
          <Label htmlFor="backup-encryption-enabled">{t('backups.encryption.enabled')}</Label>
        </div>

        {form.encryptionEnabled && (
          <>
            <Alert
              variant="warning"
              icon={ShieldAlert}
              data-testid="backup-encryption-warning"
              title={t('backups.encryption.warningTitle')}
            >
              {t('backups.encryption.warningBody')}
            </Alert>

            {form.encryptionPassphraseSet && (
              <p
                data-testid="backup-encryption-passphrase-set-indicator"
                className="text-sm text-success flex items-center gap-1.5"
              >
                {t('backups.encryption.passphraseSet')}
              </p>
            )}

            <Input
              id="backup-encryption-passphrase"
              data-testid="backup-encryption-passphrase-input"
              type="password"
              label={
                form.encryptionPassphraseSet
                  ? t('backups.encryption.passphraseChangeLabel')
                  : t('backups.encryption.passphraseLabel')
              }
              placeholder={
                form.encryptionPassphraseSet ? t('backups.encryption.passphraseKeepCurrent') : undefined
              }
              value={form.encryptionPassphrase}
              onChange={(e) => form.setEncryptionPassphrase(e.target.value)}
              autoComplete="new-password"
            />
          </>
        )}
      </CardContent>
    </Card>
  )
}
