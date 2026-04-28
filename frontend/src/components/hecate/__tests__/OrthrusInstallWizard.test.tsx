import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { OrthrusInstallWizard } from '../OrthrusInstallWizard'
import type { InstallSnippets } from '../../../api/orthrus'

const mockSnippets: InstallSnippets = {
  docker_compose: 'version: "3"\nservices:\n  orthrus:\n    image: orthrus\n    env:\n      AUTH_KEY: <AUTH_KEY>',
  systemd: '[Unit]\nDescription=Orthrus\n[Service]\nEnvironment=AUTH_KEY=<AUTH_KEY>',
  tarball: './orthrus --auth-key <AUTH_KEY>',
  homebrew: 'brew install orthrus && orthrus start --auth-key <AUTH_KEY>',
  kubernetes_daemonset: 'apiVersion: apps/v1\nkind: DaemonSet\n  - name: AUTH_KEY\n    value: "<AUTH_KEY>"',
}

const defaultProps = {
  agentName: 'My Agent',
  agentUUID: 'agent-uuid',
  authKey: 'my-secret-auth-key',
  snippets: mockSnippets,
  open: true,
  onClose: vi.fn(),
}

describe('OrthrusInstallWizard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
    })
  })

  it('renders the dialog when open', () => {
    render(<OrthrusInstallWizard {...defaultProps} />)

    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('does not render dialog when closed', () => {
    render(<OrthrusInstallWizard {...defaultProps} open={false} />)

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows the dialog title', () => {
    render(<OrthrusInstallWizard {...defaultProps} />)

    // wizard.title = "Install Orthrus Agent" (no name interpolation in translation)
    expect(screen.getByRole('heading', { name: /install orthrus agent/i })).toBeInTheDocument()
  })

  it('displays the auth key in a readonly input', () => {
    render(<OrthrusInstallWizard {...defaultProps} />)

    const input = screen.getByDisplayValue('my-secret-auth-key') as HTMLInputElement
    expect(input).toBeInTheDocument()
    expect(input.readOnly).toBe(true)
  })

  it('auth key input has correct aria attributes', () => {
    render(<OrthrusInstallWizard {...defaultProps} />)

    // Input uses aria-label="Authentication Key" (not associated via <label for="...">)
    const input = screen.getByLabelText('Authentication Key')
    expect(input).toHaveAttribute('aria-describedby', 'auth-key-warning')
  })

  it('copy auth key button writes authKey to clipboard', async () => {
    render(<OrthrusInstallWizard {...defaultProps} />)

    // aria-label = "Copy authentication key"
    const copyBtn = screen.getByRole('button', { name: 'Copy authentication key' })
    await userEvent.click(copyBtn)

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('my-secret-auth-key')
  })

  it('shows success feedback after copying auth key', async () => {
    render(<OrthrusInstallWizard {...defaultProps} />)

    await userEvent.click(screen.getByRole('button', { name: 'Copy authentication key' }))

    // aria-live paragraph shows "Copied!" when copiedKey=true
    await waitFor(() => {
      expect(screen.getByText('Copied!')).toBeInTheDocument()
    })
  })

  it('renders platform tabs', () => {
    render(<OrthrusInstallWizard {...defaultProps} />)

    expect(screen.getByRole('tab', { name: /docker/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /systemd/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /tarball/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /homebrew/i })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /kubernetes/i })).toBeInTheDocument()
  })

  it('shows docker_compose snippet in the first tab by default', async () => {
    render(<OrthrusInstallWizard {...defaultProps} />)

    await waitFor(() => {
      expect(screen.getByText(/version: "3"/)).toBeInTheDocument()
    })
  })

  it('switches tabs and shows different snippet', async () => {
    render(<OrthrusInstallWizard {...defaultProps} />)

    await userEvent.click(screen.getByRole('tab', { name: /systemd/i }))

    await waitFor(() => {
      expect(screen.getByText(/\[Unit\]/)).toBeInTheDocument()
    })
  })

  it('copy snippet replaces <AUTH_KEY> placeholder with actual authKey', async () => {
    render(<OrthrusInstallWizard {...defaultProps} />)

    // aria-label = "Copy snippet"
    await userEvent.click(screen.getByRole('button', { name: 'Copy snippet' }))

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      expect.stringContaining('my-secret-auth-key')
    )
    expect(navigator.clipboard.writeText).not.toHaveBeenCalledWith(
      expect.stringContaining('<AUTH_KEY>')
    )
  })

  it('shows snippet copy feedback', async () => {
    render(<OrthrusInstallWizard {...defaultProps} />)

    await userEvent.click(screen.getByRole('button', { name: 'Copy snippet' }))

    await waitFor(() => {
      expect(screen.getByText('Copied!')).toBeInTheDocument()
    })
  })

  it('resets copy state when dialog closes and reopens', async () => {
    const { rerender } = render(<OrthrusInstallWizard {...defaultProps} />)

    await userEvent.click(screen.getByRole('button', { name: 'Copy authentication key' }))
    // Copied! should show
    await waitFor(() => expect(screen.getByText('Copied!')).toBeInTheDocument())

    rerender(<OrthrusInstallWizard {...defaultProps} open={false} />)
    rerender(<OrthrusInstallWizard {...defaultProps} open={true} />)

    // After reopening, copy state resets — "Copied!" is no longer shown
    expect(screen.queryByText('Copied!')).not.toBeInTheDocument()
  })
})
