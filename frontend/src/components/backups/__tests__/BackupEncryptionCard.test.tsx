import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'

import { BackupEncryptionCard } from '../BackupEncryptionCard'
import type { UseBackupSettingsFormResult } from '../../../hooks/useBackups'

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

describe('BackupEncryptionCard', () => {
  it('renders the enabled toggle unchecked and hides the passphrase field by default', () => {
    render(<BackupEncryptionCard form={makeForm()} />)

    expect(screen.getByTestId('backup-encryption-toggle')).not.toBeChecked()
    expect(screen.queryByTestId('backup-encryption-passphrase-input')).not.toBeInTheDocument()
    expect(screen.queryByTestId('backup-encryption-warning')).not.toBeInTheDocument()
  })

  it('reveals the passphrase input and the "cannot be recovered" warning once enabled', () => {
    render(<BackupEncryptionCard form={makeForm({ encryptionEnabled: true })} />)

    expect(screen.getByTestId('backup-encryption-passphrase-input')).toBeInTheDocument()
    expect(screen.getByTestId('backup-encryption-warning')).toBeVisible()
    expect(screen.getByTestId('backup-encryption-warning')).toHaveTextContent(/cannot be recovered|lost/i)
  })

  it('shows the "passphrase is set" indicator when the server reports one is stored, with an empty (write-only) input', () => {
    render(
      <BackupEncryptionCard
        form={makeForm({ encryptionEnabled: true, encryptionPassphraseSet: true })}
      />
    )

    const indicator = screen.getByTestId('backup-encryption-passphrase-set-indicator')
    expect(indicator).toBeVisible()
    expect(indicator).toHaveTextContent(/passphrase is set/i)
    expect(screen.getByTestId('backup-encryption-passphrase-input')).toHaveValue('')
  })

  it('never pre-populates the passphrase input even if local state briefly holds a value from a prior open/close', () => {
    render(
      <BackupEncryptionCard
        form={makeForm({ encryptionEnabled: true, encryptionPassphraseSet: true, encryptionPassphrase: '' })}
      />
    )
    expect(screen.getByTestId('backup-encryption-passphrase-input')).toHaveValue('')
  })

  it('calls setEncryptionEnabled when the toggle is clicked', async () => {
    const user = userEvent.setup()
    const form = makeForm()
    render(<BackupEncryptionCard form={form} />)

    await user.click(screen.getByTestId('backup-encryption-toggle'))
    expect(form.setEncryptionEnabled).toHaveBeenCalledWith(true)
  })

  it('calls setEncryptionPassphrase as the user types', async () => {
    const user = userEvent.setup()
    const form = makeForm({ encryptionEnabled: true })
    render(<BackupEncryptionCard form={form} />)

    await user.type(screen.getByTestId('backup-encryption-passphrase-input'), 'x')
    expect(form.setEncryptionPassphrase).toHaveBeenCalled()
  })
})
