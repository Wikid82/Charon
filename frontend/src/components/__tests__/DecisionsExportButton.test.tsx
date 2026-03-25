import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import { DecisionsExportButton } from '../crowdsec/DecisionsExportButton'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback: string) => fallback ?? _key,
  }),
}))

const mockExportDecisions = vi.fn()

vi.mock('../../api/crowdsecDashboard', () => ({
  exportDecisions: (...args: unknown[]) => mockExportDecisions(...args),
}))

describe('DecisionsExportButton', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockExportDecisions.mockResolvedValue(new Blob(['test'], { type: 'text/csv' }))
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:test')
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
  })

  it('renders the export button', () => {
    renderWithQueryClient(<DecisionsExportButton range="24h" />)

    expect(screen.getByRole('button', { name: /export decisions/i })).toBeInTheDocument()
  })

  it('opens dropdown menu on click', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<DecisionsExportButton range="24h" />)

    await user.click(screen.getByRole('button', { name: /export decisions/i }))

    expect(screen.getByRole('menu')).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: /csv/i })).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: /json/i })).toBeInTheDocument()
  })

  it('triggers CSV export when CSV option is clicked', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<DecisionsExportButton range="24h" />)

    await user.click(screen.getByRole('button', { name: /export decisions/i }))
    await user.click(screen.getByRole('menuitem', { name: /csv/i }))

    await waitFor(() => {
      expect(mockExportDecisions).toHaveBeenCalledWith('csv', '24h')
    })
  })

  it('triggers JSON export when JSON option is clicked', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<DecisionsExportButton range="7d" />)

    await user.click(screen.getByRole('button', { name: /export decisions/i }))
    await user.click(screen.getByRole('menuitem', { name: /json/i }))

    await waitFor(() => {
      expect(mockExportDecisions).toHaveBeenCalledWith('json', '7d')
    })
  })

  it('shows error message when export fails', async () => {
    mockExportDecisions.mockRejectedValue(new Error('Server error'))

    const user = userEvent.setup()
    renderWithQueryClient(<DecisionsExportButton range="24h" />)

    await user.click(screen.getByRole('button', { name: /export decisions/i }))
    await user.click(screen.getByRole('menuitem', { name: /csv/i }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Export failed')
    })
  })

  it('closes dropdown on Escape key', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<DecisionsExportButton range="24h" />)

    await user.click(screen.getByRole('button', { name: /export decisions/i }))
    expect(screen.getByRole('menu')).toBeInTheDocument()

    await user.keyboard('{Escape}')

    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('sets aria-expanded attribute correctly', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<DecisionsExportButton range="24h" />)

    const btn = screen.getByRole('button', { name: /export decisions/i })
    expect(btn).toHaveAttribute('aria-expanded', 'false')

    await user.click(btn)
    expect(btn).toHaveAttribute('aria-expanded', 'true')
  })
})
