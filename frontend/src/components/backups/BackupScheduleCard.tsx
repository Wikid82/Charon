import { useTranslation } from 'react-i18next'

import type { UseBackupSettingsFormResult } from '../../hooks/useBackups'
import { Button, Card, CardHeader, CardTitle, CardContent, Input, Label, Switch } from '../ui'
import { NativeSelect } from '../ui/NativeSelect'
import { toast } from '../../utils/toast'

const DAYS_OF_WEEK = [
  { value: '0', labelKey: 'sunday' },
  { value: '1', labelKey: 'monday' },
  { value: '2', labelKey: 'tuesday' },
  { value: '3', labelKey: 'wednesday' },
  { value: '4', labelKey: 'thursday' },
  { value: '5', labelKey: 'friday' },
  { value: '6', labelKey: 'saturday' },
]

export interface BackupScheduleCardProps {
  form: UseBackupSettingsFormResult
}

/**
 * Replaces the dead-knob "Configuration" card (spec §3.8): schedule enable
 * toggle, Daily/Weekly presets + a raw-cron escape hatch, and local/remote
 * retention counts. Owns the single "Save Schedule" action for the combined
 * `BackupSettings` resource — `BackupEncryptionCard` shares the same `form`
 * so encryption changes are saved by this same button (spec §3.4.4: one
 * settings resource, one PUT endpoint).
 */
export function BackupScheduleCard({ form }: BackupScheduleCardProps) {
  const { t } = useTranslation()

  const handleCronInputChange = (value: string) => {
    form.setCronExpression(value)
  }

  const handleSave = () => {
    form.save({
      onSuccess: () => toast.success(t('backups.schedule.saveSuccess')),
      onError: (error) => toast.error(error.message),
    })
  }

  const showCronError = form.frequency === 'custom' && form.cronExpression.length > 0 && !form.cronValid

  return (
    <Card data-testid="backup-schedule-card">
      <CardHeader>
        <CardTitle>{t('backups.schedule.title')}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="flex items-center gap-3">
          <Switch
            id="backup-schedule-enabled"
            data-testid="backup-schedule-enabled-toggle"
            checked={form.enabled}
            onCheckedChange={form.setEnabled}
          />
          <Label htmlFor="backup-schedule-enabled">{t('backups.schedule.enabled')}</Label>
        </div>

        <fieldset className="space-y-2">
          <legend className="text-sm font-medium text-content-secondary mb-1.5">
            {t('backups.schedule.frequency')}
          </legend>
          <div className="flex flex-wrap gap-4">
            {(['daily', 'weekly', 'custom'] as const).map((option) => (
              <label key={option} className="flex items-center gap-2 text-sm text-content-primary cursor-pointer">
                <input
                  type="radio"
                  name="backup-schedule-frequency"
                  value={option}
                  checked={form.frequency === option}
                  onChange={() => form.setFrequency(option)}
                />
                {t(`backups.schedule.${option}`)}
              </label>
            ))}
          </div>
        </fieldset>

        {form.frequency !== 'custom' && (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Input
              id="backup-schedule-time"
              label={t('backups.schedule.time')}
              type="time"
              value={form.time}
              onChange={(e) => form.setTime(e.target.value)}
            />
            {form.frequency === 'weekly' && (
              <div>
                <Label htmlFor="backup-schedule-day-of-week">{t('backups.schedule.dayOfWeek')}</Label>
                <NativeSelect
                  id="backup-schedule-day-of-week"
                  className="mt-1.5"
                  value={form.dayOfWeek}
                  onChange={(e) => form.setDayOfWeek(e.target.value)}
                >
                  {DAYS_OF_WEEK.map((day) => (
                    <option key={day.value} value={day.value}>
                      {t(`backups.schedule.days.${day.labelKey}`)}
                    </option>
                  ))}
                </NativeSelect>
              </div>
            )}
          </div>
        )}

        <Input
          id="backup-schedule-cron"
          data-testid="backup-schedule-cron-input"
          label={t('backups.schedule.cronExpression')}
          value={form.cronExpression}
          onChange={(e) => handleCronInputChange(e.target.value)}
          disabled={form.frequency !== 'custom'}
          error={showCronError ? t('backups.schedule.cronInvalid') : undefined}
          errorTestId="backup-schedule-cron-error"
          helperText={form.frequency === 'custom' ? t('backups.schedule.cronHelp') : undefined}
        />

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Input
            id="backup-schedule-retention"
            data-testid="backup-schedule-retention-input"
            label={t('backups.schedule.retentionCount')}
            type="number"
            min="1"
            value={form.retentionCount}
            onChange={(e) => form.setRetentionCount(e.target.value)}
          />
          <Input
            id="backup-schedule-remote-retention"
            data-testid="backup-schedule-remote-retention-input"
            label={t('backups.schedule.remoteRetentionCount')}
            type="number"
            min="1"
            value={form.remoteRetentionCount}
            onChange={(e) => form.setRemoteRetentionCount(e.target.value)}
          />
        </div>

        <div className="flex justify-end">
          <Button onClick={handleSave} disabled={form.saveDisabled} isLoading={form.isSaving}>
            {t('backups.schedule.save')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
