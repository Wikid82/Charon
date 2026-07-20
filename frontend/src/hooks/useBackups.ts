import { useEffect, useRef, useState } from 'react'

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  getBackups,
  createBackup,
  restoreBackup,
  getBackupJob,
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
  type BackupJob,
  type BackupJobStartResponse,
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

/**
 * Poll interval for `GET /backups/jobs/:job_id` — matches `useImport.ts`'s
 * `statusQuery.refetchInterval` precedent exactly (`useImport.ts:30-37`
 * returns `3000`, not `2000` — plan §3.4.2/ASSUMPTION-002).
 */
const BACKUP_JOB_POLL_INTERVAL_MS = 3000

/**
 * Polls `GET /backups/jobs/:job_id` every `BACKUP_JOB_POLL_INTERVAL_MS`
 * while `status` is `"pending"`/`"running"`; stops (`refetchInterval:
 * false`) once terminal. Mirrors `useImport.ts`'s established
 * `statusQuery` polling pattern (refetchInterval as a function of the
 * latest query data) for consistency (plan §3.4.2). Internal — not exported.
 */
function useBackupJobPolling<TResult>(jobId: string | null) {
  return useQuery<BackupJob<TResult>>({
    queryKey: ['backup-job', jobId],
    queryFn: () => getBackupJob(jobId as string) as unknown as Promise<BackupJob<TResult>>,
    enabled: jobId !== null,
    refetchInterval: (query) => {
      const status = query.state.data?.status
      return status === 'pending' || status === 'running' ? BACKUP_JOB_POLL_INTERVAL_MS : false
    },
  })
}

/** Callbacks accepted by a `useBackupJob(...)`-instantiated hook's `mutate()` (plan §3.4.2). */
export interface UseBackupJobCallbacks<TResult> {
  onSuccess?: (result: TResult) => void
  onError?: (error: Error) => void
}

/** Shape shared by `useCreateBackup()`/`useRestoreBackup()` (plan §3.4.2). */
export interface UseBackupJobResult<TArg, TResult> {
  mutate: (arg: TArg, callbacks?: UseBackupJobCallbacks<TResult>) => void
  isPending: boolean
  job: BackupJob<TResult> | undefined
  reset: () => void
}

/**
 * Shared start+poll implementation behind `useCreateBackup()` and
 * `useRestoreBackup()` (plan §3.4.2 — DRY, both need identical start+poll
 * logic). `start` is either `createBackup` or `restoreBackup` — both now
 * return a `BackupJobStartResponse` (202) instead of blocking for the full
 * pipeline (plan §2/§3.2).
 *
 * **Public surface preserved**: callers keep using `.mutate(arg,
 * {onSuccess, onError})` and `.isPending` exactly as before the async-job
 * conversion — `onSuccess`/`onError` now fire when the polled *job* reaches
 * `completed`/`failed` (not when the initial POST resolves), via an effect
 * keyed on `job?.status`. A ref holds the latest callbacks so a poll
 * landing after a re-render never invokes a stale closure. `isPending`
 * stays `true` from the initial `mutate()` call until the job reaches a
 * terminal status, so existing `isLoading={mutation.isPending}` wiring in
 * `Backups.tsx`/`RestoreDialog.tsx` keeps working unchanged.
 */
function useBackupJob<TArg, TResult>(start: (arg: TArg) => Promise<BackupJobStartResponse>): UseBackupJobResult<
  TArg,
  TResult
> {
  const queryClient = useQueryClient()
  const [jobId, setJobId] = useState<string | null>(null)
  const [isPending, setIsPending] = useState(false)
  const callbacksRef = useRef<UseBackupJobCallbacks<TResult>>({})
  const settledJobIdRef = useRef<string | null>(null)

  const startMutation = useMutation({
    mutationFn: start,
    onSuccess: (response) => {
      setJobId(response.job_id)
    },
    onError: (error) => {
      setIsPending(false)
      callbacksRef.current.onError?.(error as Error)
    },
  })

  const jobQuery = useBackupJobPolling<TResult>(jobId)
  const job = jobQuery.data

  useEffect(() => {
    if (!job || !jobId || settledJobIdRef.current === jobId) return
    if (job.status === 'completed') {
      settledJobIdRef.current = jobId
      setIsPending(false)
      queryClient.invalidateQueries({ queryKey: BACKUPS_QUERY_KEY })
      callbacksRef.current.onSuccess?.(job.result as TResult)
    } else if (job.status === 'failed') {
      settledJobIdRef.current = jobId
      setIsPending(false)
      callbacksRef.current.onError?.(new Error(job.error?.message ?? 'Job failed'))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [job?.status, jobId])

  const mutate: UseBackupJobResult<TArg, TResult>['mutate'] = (arg, callbacks) => {
    callbacksRef.current = callbacks ?? {}
    settledJobIdRef.current = null
    setJobId(null)
    setIsPending(true)
    startMutation.mutate(arg)
  }

  const reset = () => {
    callbacksRef.current = {}
    settledJobIdRef.current = null
    setJobId(null)
    setIsPending(false)
    startMutation.reset()
  }

  return { mutate, isPending, job, reset }
}

/** Creates a new backup (optionally encrypted); invalidates the backup list once the job completes. */
export function useCreateBackup(): UseBackupJobResult<CreateBackupOptions | undefined, CreateBackupResponse> {
  return useBackupJob<CreateBackupOptions | undefined, CreateBackupResponse>((options) => createBackup(options))
}

/** Restores a backup (with an optional passphrase for `.age` archives). */
export function useRestoreBackup(): UseBackupJobResult<{ filename: string; passphrase?: string }, RestoreResult> {
  return useBackupJob<{ filename: string; passphrase?: string }, RestoreResult>(({ filename, passphrase }) =>
    restoreBackup(filename, passphrase)
  )
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
  BackupJob,
  BackupJobStartResponse,
  UploadBackupResponse,
  ValidateBackupResponse,
  BackupSettings,
  UpdateBackupSettingsPayload,
}
