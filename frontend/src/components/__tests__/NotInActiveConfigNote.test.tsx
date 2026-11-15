import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect } from 'vitest'

import { NotInActiveConfigNote } from '../NotInActiveConfigNote'

describe('NotInActiveConfigNote', () => {
  it('renders the passive text in a keyboard-reachable trigger', () => {
    render(<NotInActiveConfigNote />)

    const note = screen.getByRole('button', { name: 'Not in the active configuration' })
    expect(note).toHaveAttribute('data-testid', 'proxy-host-not-in-active-config')
  })

  it('shows the explanation when the note receives keyboard focus', async () => {
    const user = userEvent.setup()
    render(<NotInActiveConfigNote />)

    await user.tab()

    expect(await screen.findByRole('tooltip')).toHaveTextContent(
      'Its forward target is not allowed and needs another address.',
    )
  })
})
