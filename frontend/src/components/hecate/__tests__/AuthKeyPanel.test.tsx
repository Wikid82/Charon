import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { AuthKeyPanel } from '../AuthKeyPanel'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

describe('AuthKeyPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
    })
  })

  it('shows the key in a read-only field', () => {
    render(<AuthKeyPanel authKey="ch_orthrus_abc" />)

    const field = screen.getByLabelText('hecate.wizard.authKeyLabel')
    expect(field).toHaveValue('ch_orthrus_abc')
    expect(field).toHaveAttribute('readonly')
  })

  it('shows the one-time warning text', () => {
    render(<AuthKeyPanel authKey="ch_orthrus_abc" />)

    expect(screen.getByText('hecate.wizard.authKeyWarning')).toBeInTheDocument()
  })

  it('copies the key and shows copied feedback', async () => {
    const user = userEvent.setup()
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
    render(<AuthKeyPanel authKey="ch_orthrus_abc" />)

    await user.click(screen.getByRole('button', { name: 'hecate.wizard.copyAuthKey' }))

    expect(writeText).toHaveBeenCalledWith('ch_orthrus_abc')
    expect(await screen.findByText('hecate.wizard.copied')).toBeInTheDocument()
  })
})
