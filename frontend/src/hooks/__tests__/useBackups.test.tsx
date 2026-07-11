import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor, act } from '@testing-library/react'
import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as api from '../../api/backups'
import {
  useBackups,
  useCreateBackup,
  useRestoreBackup,
  useDeleteBackup,
  useUploadBackup,
  useValidateBackup,
  useBackupSettings,
  useUpdateBackupSettings,
  useBackupSettingsForm,
  BACKUPS_QUERY_KEY,
  BACKUP_SETTINGS_QUERY_KEY,
} from '../useBackups'

vi.mock('../../api/backups')

const mockBackup: api.BackupFile = {
  filename: 'backup_2026-07-07_03-00-00.zip',
  size: 1024,
  time: '2026-07-07T03:00:00Z',
}

const mockSettings: api.BackupSettings = {
  schedule_enabled: true,
  schedule_cron: '0 3 * * *',
  retention_count: 7,
  remote_retention_count: 7,
  encryption_enabled: false,
  encryption_passphrase_set: false,
}

const createWrapper = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

describe('useBackups', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches the backup list', async () => {
    vi.mocked(api.getBackups).mockResolvedValue([mockBackup])
    const { result } = renderHook(() => useBackups(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.data).toEqual([mockBackup])
  })
})

describe('useCreateBackup', () => {
  beforeEach(() => vi.clearAllMocks())

  it('creates a backup and invalidates the backup list', async () => {
    vi.mocked(api.createBackup).mockResolvedValue({ filename: 'b.zip', uuid: 'u1' })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )

    const { result } = renderHook(() => useCreateBackup(), { wrapper })
    result.current.mutate({ encrypt: true, passphrase: 'x' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.createBackup).toHaveBeenCalledWith({ encrypt: true, passphrase: 'x' })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: BACKUPS_QUERY_KEY })
  })

  it('surfaces creation errors', async () => {
    vi.mocked(api.createBackup).mockRejectedValue(new Error('insufficient space'))
    const { result } = renderHook(() => useCreateBackup(), { wrapper: createWrapper() })

    result.current.mutate(undefined)

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.error).toEqual(new Error('insufficient space'))
  })
})

describe('useRestoreBackup', () => {
  beforeEach(() => vi.clearAllMocks())

  it('restores a backup with the given filename and passphrase', async () => {
    vi.mocked(api.restoreBackup).mockResolvedValue({
      message: 'ok',
      restart_required: false,
      database_swap_pending: false,
      live_rehydrate_applied: true,
      caddy_reloaded: true,
      legacy_format: false,
    })
    const { result } = renderHook(() => useRestoreBackup(), { wrapper: createWrapper() })

    result.current.mutate({ filename: 'b.zip.age', passphrase: 'hunter2' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.restoreBackup).toHaveBeenCalledWith('b.zip.age', 'hunter2')
  })

  it('surfaces restore errors (e.g. wrong passphrase) without throwing', async () => {
    vi.mocked(api.restoreBackup).mockRejectedValue(new Error('wrong passphrase'))
    const { result } = renderHook(() => useRestoreBackup(), { wrapper: createWrapper() })

    result.current.mutate({ filename: 'b.zip.age', passphrase: 'wrong' })

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.error).toEqual(new Error('wrong passphrase'))
  })
})

describe('useValidateBackup', () => {
  beforeEach(() => vi.clearAllMocks())

  it('dry-run validates a backup with filename and passphrase, without invalidating the backup list', async () => {
    const validationResult: api.ValidateBackupResponse = {
      valid: true,
      format_version: 2,
      legacy_format: false,
      database_integrity: 'ok',
      encryption_key_required: true,
    }
    vi.mocked(api.validateBackup).mockResolvedValue(validationResult)
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )

    const { result } = renderHook(() => useValidateBackup(), { wrapper })
    result.current.mutate({ filename: 'b.zip.age', passphrase: 'hunter2' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.validateBackup).toHaveBeenCalledWith('b.zip.age', 'hunter2')
    expect(result.current.data).toEqual(validationResult)
    expect(invalidateSpy).not.toHaveBeenCalled()
  })

  it('surfaces validation errors (e.g. wrong passphrase) without throwing', async () => {
    vi.mocked(api.validateBackup).mockRejectedValue(new Error('wrong passphrase'))
    const { result } = renderHook(() => useValidateBackup(), { wrapper: createWrapper() })

    result.current.mutate({ filename: 'b.zip.age', passphrase: 'wrong' })

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.error).toEqual(new Error('wrong passphrase'))
  })
})

describe('useDeleteBackup', () => {
  it('deletes a backup and invalidates the list', async () => {
    vi.mocked(api.deleteBackup).mockResolvedValue(undefined)
    const { result } = renderHook(() => useDeleteBackup(), { wrapper: createWrapper() })

    result.current.mutate('b.zip')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.deleteBackup).toHaveBeenCalledWith('b.zip')
  })
})

describe('useUploadBackup', () => {
  it('uploads a file and invalidates the list on success', async () => {
    vi.mocked(api.uploadBackup).mockResolvedValue({
      filename: 'uploaded.zip',
      uuid: 'u1',
      legacy_format: false,
      message: 'ok',
    })
    const file = new File(['x'], 'a.zip')
    const { result } = renderHook(() => useUploadBackup(), { wrapper: createWrapper() })

    result.current.mutate({ file, passphrase: 'p' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.uploadBackup).toHaveBeenCalledWith(file, 'p')
  })
})

describe('useBackupSettings / useUpdateBackupSettings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('fetches settings', async () => {
    vi.mocked(api.getBackupSettings).mockResolvedValue(mockSettings)
    const { result } = renderHook(() => useBackupSettings(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.data).toEqual(mockSettings)
  })

  it('updates settings and invalidates the settings query', async () => {
    vi.mocked(api.updateBackupSettings).mockResolvedValue({ ...mockSettings, retention_count: 14 })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )

    const { result } = renderHook(() => useUpdateBackupSettings(), { wrapper })
    result.current.mutate({ retention_count: 14 })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: BACKUP_SETTINGS_QUERY_KEY })
  })
})

describe('useBackupSettingsForm', () => {
  beforeEach(() => vi.clearAllMocks())

  it('hydrates local state from the fetched settings', async () => {
    vi.mocked(api.getBackupSettings).mockResolvedValue(mockSettings)
    const { result } = renderHook(() => useBackupSettingsForm(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.frequency).toBe('daily')
    expect(result.current.cronExpression).toBe('0 3 * * *')
    expect(result.current.retentionCount).toBe('7')
  })

  it('derives a daily cron string from time changes', async () => {
    vi.mocked(api.getBackupSettings).mockResolvedValue(mockSettings)
    const { result } = renderHook(() => useBackupSettingsForm(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    act(() => result.current.setTime('04:00'))
    await waitFor(() => expect(result.current.cronExpression).toBe('0 4 * * *'))
  })

  it('derives a weekly cron string from day-of-week + time changes', async () => {
    vi.mocked(api.getBackupSettings).mockResolvedValue(mockSettings)
    const { result } = renderHook(() => useBackupSettingsForm(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    act(() => result.current.setFrequency('weekly'))
    act(() => result.current.setDayOfWeek('2'))
    act(() => result.current.setTime('06:00'))
    await waitFor(() => expect(result.current.cronExpression).toBe('0 6 * * 2'))
  })

  it('flags an invalid custom cron expression via cronValid', async () => {
    vi.mocked(api.getBackupSettings).mockResolvedValue(mockSettings)
    const { result } = renderHook(() => useBackupSettingsForm(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    act(() => result.current.setFrequency('custom'))
    act(() => result.current.setCronExpression('not a cron'))
    await waitFor(() => expect(result.current.cronValid).toBe(false))
    expect(result.current.saveDisabled).toBe(true)
  })

  it('disables save when encryption is enabled but no passphrase is set or entered', async () => {
    vi.mocked(api.getBackupSettings).mockResolvedValue(mockSettings)
    const { result } = renderHook(() => useBackupSettingsForm(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    act(() => result.current.setEncryptionEnabled(true))
    await waitFor(() => expect(result.current.saveDisabled).toBe(true))

    act(() => result.current.setEncryptionPassphrase('correct-horse-battery-staple'))
    await waitFor(() => expect(result.current.saveDisabled).toBe(false))
  })

  it('does not require a passphrase when one is already set server-side', async () => {
    vi.mocked(api.getBackupSettings).mockResolvedValue({
      ...mockSettings,
      encryption_enabled: true,
      encryption_passphrase_set: true,
    })
    const { result } = renderHook(() => useBackupSettingsForm(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(result.current.encryptionPassphraseSet).toBe(true)
    expect(result.current.saveDisabled).toBe(false)
  })

  it('save() sends the combined payload and omits the passphrase field when blank', async () => {
    vi.mocked(api.getBackupSettings).mockResolvedValue(mockSettings)
    vi.mocked(api.updateBackupSettings).mockResolvedValue(mockSettings)
    const { result } = renderHook(() => useBackupSettingsForm(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    const onSuccess = vi.fn()
    act(() => result.current.save({ onSuccess }))

    await waitFor(() => expect(onSuccess).toHaveBeenCalled())
    expect(api.updateBackupSettings).toHaveBeenCalledWith({
      schedule_enabled: true,
      schedule_cron: '0 3 * * *',
      retention_count: 7,
      remote_retention_count: 7,
      encryption_enabled: false,
    })
  })

  it('save() is a no-op while saveDisabled is true', async () => {
    vi.mocked(api.getBackupSettings).mockResolvedValue(mockSettings)
    const { result } = renderHook(() => useBackupSettingsForm(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    act(() => result.current.setFrequency('custom'))
    act(() => result.current.setCronExpression('invalid'))
    await waitFor(() => expect(result.current.saveDisabled).toBe(true))

    const onSuccess = vi.fn()
    act(() => result.current.save({ onSuccess }))
    expect(api.updateBackupSettings).not.toHaveBeenCalled()
    expect(onSuccess).not.toHaveBeenCalled()
  })

  it('surfaces save errors via the onError callback', async () => {
    vi.mocked(api.getBackupSettings).mockResolvedValue(mockSettings)
    vi.mocked(api.updateBackupSettings).mockRejectedValue(new Error('invalid cron expression'))
    const { result } = renderHook(() => useBackupSettingsForm(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    const onError = vi.fn()
    act(() => result.current.save({ onError }))

    await waitFor(() => expect(onError).toHaveBeenCalledWith(new Error('invalid cron expression')))
  })
})
