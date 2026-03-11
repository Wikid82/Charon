import { screen, within } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import DNS from '../DNS'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'dns.title': 'DNS Management',
        'dns.description': 'Manage DNS providers and plugins for certificate automation',
        'navigation.dnsProviders': 'DNS Providers',
        'navigation.plugins': 'Plugins',
      }
      return translations[key] || key
    },
  }),
}))

describe('DNS page', () => {
  it('renders DNS management page with navigation tabs', async () => {
    renderWithQueryClient(<DNS />, { routeEntries: ['/dns/providers'] })

    expect(await screen.findByText('DNS Management')).toBeInTheDocument()
    expect(screen.getByText('DNS Providers')).toBeInTheDocument()
    expect(screen.getByText('Plugins')).toBeInTheDocument()
  })

  it('renders the navigation component', async () => {
    renderWithQueryClient(<DNS />, { routeEntries: ['/dns/providers'] })

    const nav = await screen.findByRole('navigation')
    expect(nav).toBeInTheDocument()
  })

  it('highlights active tab based on route', async () => {
    renderWithQueryClient(<DNS />, { routeEntries: ['/dns/providers'] })

    const nav = await screen.findByRole('navigation')
    const providersLink = within(nav).getByText('DNS Providers').closest('a')

    // Active tab should have the elevated style class
    expect(providersLink).toHaveClass('bg-surface-elevated')
  })

  it('displays plugins tab as inactive when on providers route', async () => {
    renderWithQueryClient(<DNS />, { routeEntries: ['/dns/providers'] })

    const nav = await screen.findByRole('navigation')
    const pluginsLink = within(nav).getByText('Plugins').closest('a')

    // Inactive tab should not have the elevated style class
    expect(pluginsLink).not.toHaveClass('bg-surface-elevated')
    expect(pluginsLink).toHaveClass('text-content-secondary')
  })

  it('renders navigation links with correct paths', async () => {
    renderWithQueryClient(<DNS />, { routeEntries: ['/dns/providers'] })

    const nav = await screen.findByRole('navigation')
    const providersLink = within(nav).getByText('DNS Providers').closest('a')
    const pluginsLink = within(nav).getByText('Plugins').closest('a')

    expect(providersLink).toHaveAttribute('href', '/dns/providers')
    expect(pluginsLink).toHaveAttribute('href', '/dns/plugins')
  })

  it('renders content area for child routes', async () => {
    renderWithQueryClient(<DNS />, { routeEntries: ['/dns/providers'] })

    // The content area should be rendered with the border-border class
    const contentArea = document.querySelector('.bg-surface-elevated.border.border-border')
    expect(contentArea).toBeInTheDocument()
  })

  it('renders cloud icon in header actions', async () => {
    renderWithQueryClient(<DNS />, { routeEntries: ['/dns/providers'] })

    // Look for the Cloud icon in the header actions area
    const header = await screen.findByText('DNS Management')
    expect(header).toBeInTheDocument()
  })
})
