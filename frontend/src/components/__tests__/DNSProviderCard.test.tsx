import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'

import DNSProviderCard from '../DNSProviderCard'

import type { DNSProvider } from '../../api/dnsProviders'

const provider: DNSProvider = {
  uuid: 'provider-uuid',
  name: 'Seeded Provider',
  provider_type: 'manual',
  enabled: true,
  is_default: false,
  has_credentials: true,
  propagation_timeout: 60,
  polling_interval: 5,
  success_count: 0,
  failure_count: 0,
  created_at: '2026-02-15T00:00:00Z',
  updated_at: '2026-02-15T00:00:00Z',
}

const renderCard = (canManage: boolean) =>
  render(
    <DNSProviderCard
      provider={provider}
      canManage={canManage}
      onEdit={vi.fn()}
      onDelete={vi.fn()}
      onTest={vi.fn()}
    />
  )

describe('DNSProviderCard', () => {
  it('shows edit, test and delete controls when the caller can manage providers', () => {
    renderCard(true)

    expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /test/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument()
  })

  it('wires the administrator controls to their callbacks', async () => {
    const onEdit = vi.fn()
    const onTest = vi.fn()
    const onDelete = vi.fn()
    render(
      <DNSProviderCard provider={provider} canManage onEdit={onEdit} onDelete={onDelete} onTest={onTest} />
    )

    await userEvent.click(screen.getByRole('button', { name: /edit/i }))
    expect(onEdit).toHaveBeenCalledWith(provider)

    await userEvent.click(screen.getByRole('button', { name: /test/i }))
    expect(onTest).toHaveBeenCalledWith('provider-uuid')

    await userEvent.click(screen.getByRole('button', { name: /delete/i }))
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(within(dialog).getByRole('button', { name: /delete/i }))
    expect(onDelete).toHaveBeenCalled()
  })

  it('hides edit, test and delete controls from a standard user but keeps the provider name', () => {
    renderCard(false)

    expect(screen.queryByRole('button', { name: /edit/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /test/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument()
    expect(screen.getByText('Seeded Provider')).toBeInTheDocument()
  })
})
