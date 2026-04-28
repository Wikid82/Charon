import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'

import { ConnectionTypeSelector } from '../ConnectionTypeSelector'

describe('ConnectionTypeSelector', () => {
  it('renders all three connection options', () => {
    render(<ConnectionTypeSelector value="direct" onChange={vi.fn()} />)

    expect(screen.getByRole('combobox')).toBeInTheDocument()
    expect(screen.getByText('Direct')).toBeInTheDocument()
    expect(screen.getByText('Orthrus Agent')).toBeInTheDocument()
    expect(screen.getByText('Cloudflare Tunnel')).toBeInTheDocument()
  })

  it('shows the current value as selected', () => {
    render(<ConnectionTypeSelector value="orthrus" onChange={vi.fn()} />)

    const select = screen.getByRole('combobox') as HTMLSelectElement
    expect(select.value).toBe('orthrus')
  })

  it('calls onChange with correct value when selection changes', async () => {
    const onChange = vi.fn()
    render(<ConnectionTypeSelector value="direct" onChange={onChange} />)

    await userEvent.selectOptions(screen.getByRole('combobox'), 'Cloudflare Tunnel')

    expect(onChange).toHaveBeenCalledWith('cloudflare')
  })

  it('calls onChange with orthrus when orthrus is selected', async () => {
    const onChange = vi.fn()
    render(<ConnectionTypeSelector value="direct" onChange={onChange} />)

    await userEvent.selectOptions(screen.getByRole('combobox'), 'Orthrus Agent')

    expect(onChange).toHaveBeenCalledWith('orthrus')
  })

  it('is disabled when disabled prop is true', () => {
    render(<ConnectionTypeSelector value="direct" onChange={vi.fn()} disabled />)

    expect(screen.getByRole('combobox')).toBeDisabled()
  })

  it('is not disabled by default', () => {
    render(<ConnectionTypeSelector value="direct" onChange={vi.fn()} />)

    expect(screen.getByRole('combobox')).not.toBeDisabled()
  })

  it('uses custom id when provided', () => {
    render(<ConnectionTypeSelector value="direct" onChange={vi.fn()} id="my-selector" />)

    expect(screen.getByRole('combobox')).toHaveAttribute('id', 'my-selector')
  })

  it('has default id when not provided', () => {
    render(<ConnectionTypeSelector value="direct" onChange={vi.fn()} />)

    expect(screen.getByRole('combobox')).toHaveAttribute('id', 'connection-type')
  })

  it('has accessible aria-label', () => {
    render(<ConnectionTypeSelector value="direct" onChange={vi.fn()} />)

    expect(screen.getByRole('combobox')).toHaveAttribute('aria-label')
  })
})
