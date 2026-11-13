import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { AccessListForm } from '../AccessListForm'

vi.mock('../../api/system', () => ({
  getMyIP: vi.fn(),
}))

globalThis.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}))

describe('AccessListForm save error', () => {
  it('shows the save refusal above the actions', () => {
    render(
      <AccessListForm
        onSubmit={vi.fn()}
        onCancel={vi.fn()}
        saveErrorMessage="An enabled allow-list needs at least one rule."
      />,
    )
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('An enabled allow-list needs at least one rule.')
    expect(screen.getByTestId('access-list-save-error')).toBe(alert)
  })

  it('shows no alert when there is no save refusal', () => {
    render(<AccessListForm onSubmit={vi.fn()} onCancel={vi.fn()} saveErrorMessage={null} />)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows no alert when the message is omitted', () => {
    render(<AccessListForm onSubmit={vi.fn()} onCancel={vi.fn()} />)
    expect(screen.queryByTestId('access-list-save-error')).not.toBeInTheDocument()
  })
})
