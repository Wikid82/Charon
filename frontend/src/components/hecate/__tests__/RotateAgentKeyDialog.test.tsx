import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { type OrthrusAgent } from '../../../api/orthrus'
import { RotateAgentKeyDialog } from '../RotateAgentKeyDialog'

const mockRotate = vi.fn()

vi.mock('../../../hooks/useOrthrus', () => ({
  useRotateAgentKey: () => ({ mutateAsync: mockRotate, isPending: false }),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

const agent: OrthrusAgent = {
  uuid: 'agent-1',
  name: 'Prod Agent',
  status: 'online',
  capabilities: '[]',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 0,
  write_enabled: false,
}

describe('RotateAgentKeyDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the confirmation step first without the key', () => {
    render(<RotateAgentKeyDialog agent={agent} open onClose={vi.fn()} />)

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText('hecate.agentManager.rotateDescription')).toBeInTheDocument()
    expect(screen.queryByLabelText('hecate.wizard.authKeyLabel')).not.toBeInTheDocument()
    expect(mockRotate).not.toHaveBeenCalled()
  })

  it('cancel closes without rotating', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<RotateAgentKeyDialog agent={agent} open onClose={onClose} />)

    await user.click(screen.getByRole('button', { name: 'common.cancel' }))

    expect(onClose).toHaveBeenCalledTimes(1)
    expect(mockRotate).not.toHaveBeenCalled()
  })

  it('confirm rotates the key and shows it once', async () => {
    const user = userEvent.setup()
    mockRotate.mockResolvedValue({ auth_key: 'ch_orthrus_new' })
    render(<RotateAgentKeyDialog agent={agent} open onClose={vi.fn()} />)

    await user.click(screen.getByRole('button', { name: 'hecate.agentManager.rotateConfirm' }))

    expect(mockRotate).toHaveBeenCalledWith('agent-1')
    const field = await screen.findByLabelText('hecate.wizard.authKeyLabel')
    expect(field).toHaveValue('ch_orthrus_new')
    expect(screen.getByText('hecate.agentManager.rotateSuccessDescription')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'hecate.agentManager.rotateConfirm' })).not.toBeInTheDocument()
  })

  it('done closes the dialog after the key was shown', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    mockRotate.mockResolvedValue({ auth_key: 'ch_orthrus_new' })
    render(<RotateAgentKeyDialog agent={agent} open onClose={onClose} />)

    await user.click(screen.getByRole('button', { name: 'hecate.agentManager.rotateConfirm' }))
    await user.click(await screen.findByRole('button', { name: 'hecate.agentManager.rotateDone' }))

    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('shows an error and stays on the confirmation step when rotation fails', async () => {
    const user = userEvent.setup()
    mockRotate.mockRejectedValue(new Error('boom'))
    render(<RotateAgentKeyDialog agent={agent} open onClose={vi.fn()} />)

    await user.click(screen.getByRole('button', { name: 'hecate.agentManager.rotateConfirm' }))

    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent('hecate.agentManager.rotateError'),
    )
    expect(screen.queryByLabelText('hecate.wizard.authKeyLabel')).not.toBeInTheDocument()
  })

  it('never uses the native confirm dialog', async () => {
    const user = userEvent.setup()
    const confirmSpy = vi.spyOn(window, 'confirm')
    mockRotate.mockResolvedValue({ auth_key: 'ch_orthrus_new' })
    render(<RotateAgentKeyDialog agent={agent} open onClose={vi.fn()} />)

    await user.click(screen.getByRole('button', { name: 'hecate.agentManager.rotateConfirm' }))
    await screen.findByLabelText('hecate.wizard.authKeyLabel')

    expect(confirmSpy).not.toHaveBeenCalled()
    confirmSpy.mockRestore()
  })
})
