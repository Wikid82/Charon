import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect } from 'vitest'

import FeedbackWidget from '../FeedbackWidget'

const GITHUB_BUG_URL =
  'https://github.com/Wikid82/Charon/issues/new?template=bug_report.md'
const GITHUB_FEATURE_URL =
  'https://github.com/Wikid82/Charon/issues/new?template=feature_request.md'
const DOCS_URL = 'https://wikid82.github.io/Charon/'

describe('FeedbackWidget', () => {
  // 1. Renders trigger button with correct aria-label
  it('renders trigger button with correct aria-label', () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    expect(trigger).toBeInTheDocument()
  })

  // 2. Trigger button has aria-expanded="false" by default
  it('trigger button has aria-expanded="false" by default', () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
  })

  // 3. Panel is not in the DOM initially (conditional render keeps links out of tab order)
  it('panel is not in the DOM initially', () => {
    const { container } = render(<FeedbackWidget />)
    // With conditional rendering the nav is unmounted when closed
    const panel = container.querySelector('#feedback-panel')
    expect(panel).toBeNull()
  })

  // 4. Clicking trigger opens the panel
  it('clicking trigger opens the panel', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
  })

  // 5. Trigger has aria-expanded="true" when open
  it('trigger has aria-expanded="true" when open', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
  })

  // 6. Panel contains a nav element with aria-label
  it('panel contains a nav element with aria-label', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    const nav = screen.getByRole('navigation', { name: 'Feedback options' })
    expect(nav).toBeInTheDocument()
  })

  // 7. Panel contains bug report link pointing to correct URL
  it('panel contains bug report link pointing to correct URL', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    const bugLink = screen.getByRole('link', { name: /report a bug/i })
    expect(bugLink).toHaveAttribute('href', GITHUB_BUG_URL)
  })

  // 8. Panel contains feature request link pointing to correct URL
  it('panel contains feature request link pointing to correct URL', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    const featureLink = screen.getByRole('link', { name: /request a feature/i })
    expect(featureLink).toHaveAttribute('href', GITHUB_FEATURE_URL)
  })

  // 9. Both links have target="_blank"
  it('both links have target="_blank"', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    const bugLink = screen.getByRole('link', { name: /report a bug/i })
    const featureLink = screen.getByRole('link', { name: /request a feature/i })
    expect(bugLink).toHaveAttribute('target', '_blank')
    expect(featureLink).toHaveAttribute('target', '_blank')
  })

  // 10. Both links have rel="noopener noreferrer"
  it('both links have rel="noopener noreferrer"', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    const bugLink = screen.getByRole('link', { name: /report a bug/i })
    const featureLink = screen.getByRole('link', { name: /request a feature/i })
    expect(bugLink).toHaveAttribute('rel', 'noopener noreferrer')
    expect(featureLink).toHaveAttribute('rel', 'noopener noreferrer')
  })

  // 11. Pressing Escape closes the panel
  it('pressing Escape closes the panel', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')

    // Find the nav and fire Escape
    const nav = screen.getByRole('navigation', { name: 'Feedback options' })
    fireEvent.keyDown(nav, { key: 'Escape', code: 'Escape' })

    expect(screen.getByRole('button', { name: 'Open feedback menu' })).toHaveAttribute(
      'aria-expanded',
      'false'
    )
  })

  // 12. Pressing Escape returns focus to trigger
  it('pressing Escape returns focus to trigger', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)

    const nav = screen.getByRole('navigation', { name: 'Feedback options' })
    fireEvent.keyDown(nav, { key: 'Escape', code: 'Escape' })

    await waitFor(() => {
      expect(document.activeElement).toBe(
        screen.getByRole('button', { name: 'Open feedback menu' })
      )
    })
  })

  // 13. Clicking backdrop closes panel
  it('clicking backdrop closes panel', async () => {
    const { container } = render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')

    // The backdrop is the fixed inset-0 z-40 div with aria-hidden="true"
    const backdrop = container.querySelector('[aria-hidden="true"]') as HTMLElement
    expect(backdrop).not.toBeNull()
    fireEvent.click(backdrop)

    expect(screen.getByRole('button', { name: 'Open feedback menu' })).toHaveAttribute(
      'aria-expanded',
      'false'
    )
  })

  // 13a. Clicking backdrop returns focus to trigger (uses close() path)
  it('clicking backdrop returns focus to trigger', async () => {
    const { container } = render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)

    const backdrop = container.querySelector('[aria-hidden="true"]') as HTMLElement
    fireEvent.click(backdrop)

    await waitFor(() => {
      expect(document.activeElement).toBe(
        screen.getByRole('button', { name: 'Open feedback menu' })
      )
    })
  })

  // 14. i18n keys render correctly
  it('i18n keys render correctly (trigger label)', () => {
    render(<FeedbackWidget />)
    // The global i18n mock in setup.ts resolves real en/translation.json keys
    expect(screen.getByRole('button', { name: 'Open feedback menu' })).toBeInTheDocument()
  })

  // 15. Focus moves to first link when panel opens
  it('focus moves to first link when panel opens', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)

    await waitFor(() => {
      const bugLink = screen.getByRole('link', { name: /report a bug/i })
      expect(document.activeElement).toBe(bugLink)
    })
  })

  // 16. Docs link has correct href
  it('docs link has correct href', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    const docsLink = screen.getByRole('link', { name: /view documentation/i })
    expect(docsLink).toHaveAttribute('href', DOCS_URL)
  })

  // 17. Docs link opens in new tab
  it('docs link opens in new tab', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    const docsLink = screen.getByRole('link', { name: /view documentation/i })
    expect(docsLink).toHaveAttribute('target', '_blank')
  })

  // 18. Docs link has rel="noopener noreferrer"
  it('docs link has rel="noopener noreferrer"', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    const docsLink = screen.getByRole('link', { name: /view documentation/i })
    expect(docsLink).toHaveAttribute('rel', 'noopener noreferrer')
  })

  // 19. Docs link has correct aria-label from i18n key
  it('docs link has correct aria-label', async () => {
    render(<FeedbackWidget />)
    const trigger = screen.getByRole('button', { name: 'Open feedback menu' })
    await userEvent.click(trigger)
    const docsLink = screen.getByRole('link', {
      name: 'View documentation (opens docs site in new tab)',
    })
    expect(docsLink).toBeInTheDocument()
  })
})
