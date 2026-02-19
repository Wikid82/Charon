import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import '@testing-library/jest-dom/vitest'
import Settings from '../Settings'

const translations: Record<string, string> = {
  'settings.title': 'Settings',
  'settings.description': 'Configure your Charon instance',
  'settings.system': 'System',
  'navigation.notifications': 'Notifications',
  'settings.smtp': 'Email (SMTP)',
  'settings.account': 'Account',
}

const t = (key: string) => translations[key] ?? key

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t }),
}))

const renderWithRoute = (route: string) =>
  render(
    <MemoryRouter initialEntries={[route]}>
      <Routes>
        <Route path="/settings" element={<Settings />}>
          <Route path="system" element={<div>System Page</div>} />
          <Route path="notifications" element={<div>Notifications Page</div>} />
          <Route path="smtp" element={<div>SMTP Page</div>} />
          <Route path="account" element={<div>Account Page</div>} />
        </Route>
      </Routes>
    </MemoryRouter>
  )

describe('Settings page', () => {
  it('highlights the active nav item for the current route', () => {
    renderWithRoute('/settings/system')

    const activeLink = screen.getByRole('link', { name: 'System' })
    const inactiveLink = screen.getByRole('link', { name: 'Notifications' })

    expect(activeLink).toHaveClass('bg-surface-elevated')
    expect(activeLink).toHaveClass('text-content-primary')
    expect(inactiveLink).toHaveClass('text-content-secondary')
    expect(screen.getByText('System Page')).toBeInTheDocument()
  })

  it('keeps navigation order consistent', () => {
    renderWithRoute('/settings/notifications')

    const links = screen.getAllByRole('link')
    const labels = links.map(link => link.textContent)

    expect(labels).toEqual(['System', 'Notifications', 'Email (SMTP)', 'Account'])
  })
})
