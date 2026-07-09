import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'

import { BackupScheduleCard } from '../BackupScheduleCard'
import type { UseBackupSettingsFormResult } from '../../../hooks/useBackups'

vi.mock('../../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

function makeForm(overrides: Partial<UseBackupSettingsFormResult> = {}): UseBackupSettingsFormResult {
  return {
    settings: undefined,
    isLoading: false,
    enabled: true,
    setEnabled: vi.fn(),
    frequency: 'daily',
    setFrequency: vi.fn(),
    time: '03:00',
    setTime: vi.fn(),
    dayOfWeek: '0',
    setDayOfWeek: vi.fn(),
    cronExpression: '0 3 * * *',
    setCronExpression: vi.fn(),
    cronValid: true,
    retentionCount: '7',
    setRetentionCount: vi.fn(),
    remoteRetentionCount: '7',
    setRemoteRetentionCount: vi.fn(),
    encryptionEnabled: false,
    setEncryptionEnabled: vi.fn(),
    encryptionPassphrase: '',
    setEncryptionPassphrase: vi.fn(),
    encryptionPassphraseSet: false,
    isSaving: false,
    saveDisabled: false,
    save: vi.fn(),
    ...overrides,
  }
}

describe('BackupScheduleCard', () => {
  it('renders the enabled toggle, cron string, and retention counts', () => {
    render(<BackupScheduleCard form={makeForm()} />)

    expect(screen.getByTestId('backup-schedule-card')).toBeInTheDocument()
    expect(screen.getByTestId('backup-schedule-enabled-toggle')).toBeChecked()
    expect(screen.getByTestId('backup-schedule-cron-input')).toHaveValue('0 3 * * *')
    expect(screen.getByTestId('backup-schedule-retention-input')).toHaveValue(7)
    expect(screen.getByTestId('backup-schedule-remote-retention-input')).toHaveValue(7)
  })

  it('reflects a disabled schedule', () => {
    render(<BackupScheduleCard form={makeForm({ enabled: false })} />)
    expect(screen.getByTestId('backup-schedule-enabled-toggle')).not.toBeChecked()
  })

  it('shows the day-of-week select only in weekly mode', () => {
    const { rerender } = render(<BackupScheduleCard form={makeForm({ frequency: 'daily' })} />)
    expect(screen.queryByLabelText(/day of week/i)).not.toBeInTheDocument()

    rerender(<BackupScheduleCard form={makeForm({ frequency: 'weekly' })} />)
    expect(screen.getByLabelText(/day of week/i)).toBeInTheDocument()
  })

  it('disables the cron input outside custom mode and enables it in custom mode', () => {
    const { rerender } = render(<BackupScheduleCard form={makeForm({ frequency: 'daily' })} />)
    expect(screen.getByTestId('backup-schedule-cron-input')).toBeDisabled()

    rerender(<BackupScheduleCard form={makeForm({ frequency: 'custom' })} />)
    expect(screen.getByTestId('backup-schedule-cron-input')).toBeEnabled()
  })

  it('calls setFrequency when a preset radio is selected', async () => {
    const user = userEvent.setup()
    const form = makeForm()
    render(<BackupScheduleCard form={form} />)

    await user.click(screen.getByRole('radio', { name: /custom/i }))
    expect(form.setFrequency).toHaveBeenCalledWith('custom')
  })

  it('shows inline cron validation error text in custom mode when cronValid is false', () => {
    render(
      <BackupScheduleCard
        form={makeForm({ frequency: 'custom', cronExpression: 'not a cron', cronValid: false })}
      />
    )

    expect(screen.getByTestId('backup-schedule-cron-error')).toBeVisible()
    expect(screen.getByTestId('backup-schedule-cron-error')).toHaveTextContent(/invalid/i)
  })

  it('disables the Save Schedule button when the form reports saveDisabled', () => {
    render(<BackupScheduleCard form={makeForm({ saveDisabled: true })} />)
    expect(screen.getByRole('button', { name: /save schedule/i })).toBeDisabled()
  })

  it('calls form.save with success/error callbacks when Save Schedule is clicked', async () => {
    const user = userEvent.setup()
    const form = makeForm()
    render(<BackupScheduleCard form={form} />)

    await user.click(screen.getByRole('button', { name: /save schedule/i }))

    expect(form.save).toHaveBeenCalledWith(
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) })
    )
  })

  it('updates retention inputs via the form setters', async () => {
    const user = userEvent.setup()
    const form = makeForm()
    render(<BackupScheduleCard form={form} />)

    await user.clear(screen.getByTestId('backup-schedule-retention-input'))
    await user.type(screen.getByTestId('backup-schedule-retention-input'), '14')
    await user.clear(screen.getByTestId('backup-schedule-remote-retention-input'))
    await user.type(screen.getByTestId('backup-schedule-remote-retention-input'), '30')

    expect(form.setRetentionCount).toHaveBeenCalled()
    expect(form.setRemoteRetentionCount).toHaveBeenCalled()
  })

  it('updates the time and day-of-week fields in weekly mode', async () => {
    const user = userEvent.setup()
    const form = makeForm({ frequency: 'weekly' })
    render(<BackupScheduleCard form={form} />)

    await user.clear(screen.getByLabelText(/^time/i))
    await user.type(screen.getByLabelText(/^time/i), '04:30')
    expect(form.setTime).toHaveBeenCalled()

    await userEvent.selectOptions(screen.getByLabelText(/day of week/i), '2')
    expect(form.setDayOfWeek).toHaveBeenCalledWith('2')
  })

  it('calls setCronExpression when typing a custom cron expression', async () => {
    const user = userEvent.setup()
    const form = makeForm({ frequency: 'custom', cronExpression: '' })
    render(<BackupScheduleCard form={form} />)

    await user.type(screen.getByTestId('backup-schedule-cron-input'), '0')
    expect(form.setCronExpression).toHaveBeenCalled()
  })

  it('invokes the success/error toast callbacks passed to form.save', async () => {
    const { toast } = await import('../../../utils/toast')
    const user = userEvent.setup()
    const form = makeForm({
      save: vi.fn((callbacks) => {
        callbacks?.onSuccess?.()
        callbacks?.onError?.(new Error('boom'))
      }),
    })
    render(<BackupScheduleCard form={form} />)

    await user.click(screen.getByRole('button', { name: /save schedule/i }))

    expect(toast.success).toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalledWith('boom')
  })
})
