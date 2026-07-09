import { useEffect, useState } from 'react'

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  getBackups,
  createBackup,
  restoreBackup,
  deleteBackup,
  uploadBackup,
  validateBackup,
  getBackupSettings,
  updateBackupSettings,
  type BackupFile,
  type BackupRemoteCopyEntry,
  type CreateBackupOptions,
  type CreateBackupResponse,
  type RestoreResult,
  type UploadBackupResponse,
  type ValidateBackupResponse,
  type BackupSettings,
  type UpdateBackupSettingsPayload,
} from '../api/backups'
import { isValidCronExpression, parseCronPreset, type ScheduleFrequency } from '../utils/cron'

/** Query key for the backup list (spec §3.8). */
export const BACKUPS_QUERY_KEY = ['backups']
/** Query key for the typed backup settings facade (spec §3.8). */
export const BACKUP_SETTINGS_QUERY_KEY = ['backup-settings']

/** Fetches the backup history list. */
export function useBackups() {
  return useQuery({
    queryKey: BACKUPS_QUERY_KEY,
    queryFn: getBackups,
  })
}

/** Creates a new backup (optionally encrypted); invalidates the backup list on success. */
export function useCreateBackup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (options?: CreateBackupOptions) => createBackup(options),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: BACKUPS_QUERY_KEY })
    },
  })
}

/** Restores a backup (with an optional passphrase for `.age` archives). */
export function useRestoreBackup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ filename, passphrase }: { filename: string; passphrase?: string }) =>
      restoreBackup(filename, passphrase),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: BACKUPS_QUERY_KEY })
    },
  })
}

/** Deletes a backup; invalidates the backup list on success. */
export function useDeleteBackup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (filename: string) => deleteBackup(filename),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: BACKUPS_QUERY_KEY })
    },
  })
}

/** Uploads a backup archive for validation and storage; invalidates the backup list on success. */
export function useUploadBackup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ file, passphrase }: { file: File; passphrase?: string }) => uploadBackup(file, passphrase),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: BACKUPS_QUERY_KEY })
    },
  })
}

/** Dry-run validates a backup archive (no invalidation — nothing mutates). */
export function useValidateBackup() {
  return useMutation({
    mutationFn: ({ filename, passphrase }: { filename: string; passphrase?: string }) =>
      validateBackup(filename, passphrase),
  })
}

/** Fetches the current backup schedule/retention/encryption settings. */
export function useBackupSettings() {
  return useQuery({
    queryKey: BACKUP_SETTINGS_QUERY_KEY,
    queryFn: getBackupSettings,
  })
}

/** Updates the backup settings (partial update); invalidates the settings query on success. */
export function useUpdateBackupSettings() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: UpdateBackupSettingsPayload) => updateBackupSettings(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: BACKUP_SETTINGS_QUERY_KEY })
    },
  })
}

/**
 * Combined backup-settings form state (schedule + retention + encryption),
 * saved together through a single `PUT /api/v1/backups/settings` call —
 * mirrors the single BackupSettings resource on the backend (spec §3.3.2,
 * §3.4.4), so the schedule and encryption cards share one save action rather
 * than racing two independent partial-update requests.
 */
export interface UseBackupSettingsFormResult {
  settings?: BackupSettings
  isLoading: boolean

  enabled: boolean
  setEnabled: (value: boolean) => void

  frequency: ScheduleFrequency
  setFrequency: (value: ScheduleFrequency) => void
  time: string
  setTime: (value: string) => void
  dayOfWeek: string
  setDayOfWeek: (value: string) => void
  cronExpression: string
  setCronExpression: (value: string) => void
  cronValid: boolean

  retentionCount: string
  setRetentionCount: (value: string) => void
  remoteRetentionCount: string
  setRemoteRetentionCount: (value: string) => void

  encryptionEnabled: boolean
  setEncryptionEnabled: (value: boolean) => void
  encryptionPassphrase: string
  setEncryptionPassphrase: (value: string) => void
  encryptionPassphraseSet: boolean

  isSaving: boolean
  saveDisabled: boolean
  save: (callbacks?: { onSuccess?: () => void; onError?: (error: Error) => void }) => void
}

/**
 * Owns the combined schedule/retention/encryption settings form used by
 * `BackupScheduleCard` and `BackupEncryptionCard`. Centralizing this in one
 * hook (rather than each card independently fetching/saving its own slice)
 * means there is exactly one save action for the whole `BackupSettings`
 * resource, matching the single PUT endpoint it maps to.
 */
export function useBackupSettingsForm(): UseBackupSettingsFormResult {
  const { data: settings, isLoading } = useBackupSettings()
  const updateMutation = useUpdateBackupSettings()

  const [hydrated, setHydrated] = useState(false)
  const [enabled, setEnabled] = useState(true)
  const [frequency, setFrequency] = useState<ScheduleFrequency>('daily')
  const [time, setTime] = useState('03:00')
  const [dayOfWeek, setDayOfWeek] = useState('0')
  const [cronExpression, setCronExpression] = useState('0 3 * * *')
  const [retentionCount, setRetentionCount] = useState('7')
  const [remoteRetentionCount, setRemoteRetentionCount] = useState('7')
  const [encryptionEnabled, setEncryptionEnabled] = useState(false)
  const [encryptionPassphrase, setEncryptionPassphrase] = useState('')

  useEffect(() => {
    if (!settings || hydrated) return
    const preset = parseCronPreset(settings.schedule_cron)
    setEnabled(settings.schedule_enabled)
    setFrequency(preset.frequency)
    setTime(preset.time)
    setDayOfWeek(preset.dayOfWeek)
    setCronExpression(settings.schedule_cron)
    setRetentionCount(String(settings.retention_count))
    setRemoteRetentionCount(String(settings.remote_retention_count))
    setEncryptionEnabled(settings.encryption_enabled)
    setHydrated(true)
  }, [settings, hydrated])

  // Recompute the derived cron string whenever a preset (non-custom) input changes.
  useEffect(() => {
    if (frequency === 'custom') return
    const hour = parseInt(time.split(':')[0] ?? '0', 10) || 0
    setCronExpression(frequency === 'daily' ? `0 ${hour} * * *` : `0 ${hour} * * ${dayOfWeek}`)
  }, [frequency, time, dayOfWeek])

  const cronValid = isValidCronExpression(cronExpression)
  const encryptionPassphraseSet = settings?.encryption_passphrase_set ?? false
  const needsPassphrase = encryptionEnabled && !encryptionPassphrase && !encryptionPassphraseSet
  const saveDisabled = updateMutation.isPending || (frequency === 'custom' && !cronValid) || needsPassphrase

  const save: UseBackupSettingsFormResult['save'] = (callbacks) => {
    if (saveDisabled) return

    const retention = parseInt(retentionCount, 10)
    const remoteRetention = parseInt(remoteRetentionCount, 10)

    updateMutation.mutate(
      {
        schedule_enabled: enabled,
        schedule_cron: cronExpression,
        retention_count: Number.isNaN(retention) ? 1 : retention,
        remote_retention_count: Number.isNaN(remoteRetention) ? 1 : remoteRetention,
        encryption_enabled: encryptionEnabled,
        ...(encryptionPassphrase ? { encryption_passphrase: encryptionPassphrase } : {}),
      },
      {
        onSuccess: () => {
          setEncryptionPassphrase('')
          callbacks?.onSuccess?.()
        },
        onError: (error) => callbacks?.onError?.(error as Error),
      }
    )
  }

  return {
    settings,
    isLoading,
    enabled,
    setEnabled,
    frequency,
    setFrequency,
    time,
    setTime,
    dayOfWeek,
    setDayOfWeek,
    cronExpression,
    setCronExpression,
    cronValid,
    retentionCount,
    setRetentionCount,
    remoteRetentionCount,
    setRemoteRetentionCount,
    encryptionEnabled,
    setEncryptionEnabled,
    encryptionPassphrase,
    setEncryptionPassphrase,
    encryptionPassphraseSet,
    isSaving: updateMutation.isPending,
    saveDisabled,
    save,
  }
}

export type { ScheduleFrequency }

export type {
  BackupFile,
  BackupRemoteCopyEntry,
  CreateBackupOptions,
  CreateBackupResponse,
  RestoreResult,
  UploadBackupResponse,
  ValidateBackupResponse,
  BackupSettings,
  UpdateBackupSettingsPayload,
}
