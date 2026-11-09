import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import CertificateCleanupDialog from '../CertificateCleanupDialog'

import type { ComponentProps } from 'react'

const certificates = [{ uuid: 'cert-1', name: 'Wildcard', domain: '*.example.com' }]

const renderDialog = (overrides: Partial<ComponentProps<typeof CertificateCleanupDialog>> = {}) => {
  const props = {
    onConfirm: vi.fn(),
    onCancel: vi.fn(),
    certificates,
    hostNames: ['app.example.com'],
    isBulk: false,
    ...overrides,
  }
  const utils = render(<CertificateCleanupDialog {...props} />)
  return { ...utils, props }
}

describe('CertificateCleanupDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('is exposed as a modal dialog named by its heading', () => {
    renderDialog()

    const dialog = screen.getByRole('dialog', { name: 'Delete Proxy Host?' })
    expect(dialog).toHaveAttribute('aria-modal', 'true')
  })

  it('uses the bulk heading when deleting several hosts', () => {
    renderDialog({ isBulk: true, hostNames: ['a.example.com', 'b.example.com'] })

    expect(screen.getByRole('dialog', { name: 'Delete 2 Proxy Hosts?' })).toBeInTheDocument()
  })

  it('moves focus to the Cancel button on open', () => {
    renderDialog()

    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus()
  })

  it('closes on Escape', async () => {
    const { props } = renderDialog()

    await userEvent.keyboard('{Escape}')

    expect(props.onCancel).toHaveBeenCalledTimes(1)
    expect(props.onConfirm).not.toHaveBeenCalled()
  })

  it.each(['{Enter}', ' '])('activates Cancel with the %j key', async (key) => {
    const { props } = renderDialog()

    await userEvent.keyboard(key)

    expect(props.onCancel).toHaveBeenCalledTimes(1)
  })

  it('closes when the backdrop is clicked', async () => {
    const { props } = renderDialog()

    await userEvent.click(screen.getByRole('presentation'))

    expect(props.onCancel).toHaveBeenCalledTimes(1)
  })

  it('stays open when the dialog body is clicked', async () => {
    const { props } = renderDialog()

    await userEvent.click(screen.getByRole('dialog'))

    expect(props.onCancel).not.toHaveBeenCalled()
  })

  it('confirms with deleteCerts=false when the certificate option is left unchecked', async () => {
    const { props } = renderDialog()

    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    expect(props.onConfirm).toHaveBeenCalledWith(false)
  })

  it('confirms with deleteCerts=true when the certificate option is checked', async () => {
    const { props } = renderDialog()

    await userEvent.click(screen.getByRole('checkbox'))
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

    expect(props.onConfirm).toHaveBeenCalledWith(true)
  })
})
