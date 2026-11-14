import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as securityApi from '../../api/security'
import { useAuth } from '../../hooks/useAuth'
import WafConfig from '../WafConfig'

import type { SecurityRuleSet } from '../../api/security'

vi.mock('../../api/security')
vi.mock('../../hooks/useAuth', () => ({ useAuth: vi.fn() }))

// The server withholds content from standard users and sends has_content plus content_line_count instead.
const redactedRuleSet: SecurityRuleSet = {
  uuid: 'uuid-1',
  name: 'OWASP CRS',
  mode: 'blocking',
  last_updated: '2024-01-15T10:00:00Z',
  has_content: true,
  content_line_count: 3,
}

const renderPage = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <WafConfig />
      </BrowserRouter>
    </QueryClientProvider>
  )
}

const setRole = (role: 'admin' | 'user') => {
  vi.mocked(useAuth).mockReturnValue({ user: { role } } as unknown as ReturnType<typeof useAuth>)
}

describe('WafConfig for standard users', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(undefined as unknown as securityApi.SecurityStatus)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue({ config: null })
    vi.mocked(securityApi.getRuleSets).mockResolvedValue({ rulesets: [redactedRuleSet] })
    setRole('user')
  })

  it('counts the rules from content_line_count when the content is withheld', async () => {
    renderPage()

    expect(await screen.findByText('3 rule(s)')).toBeInTheDocument()
  })

  it('shows no rule count for a rule set with no content', async () => {
    vi.mocked(securityApi.getRuleSets).mockResolvedValue({
      rulesets: [{ ...redactedRuleSet, has_content: false, content_line_count: 0 }],
    })
    renderPage()

    expect(await screen.findByTestId('rulesets-table')).toBeInTheDocument()
    expect(screen.queryByText(/rule\(s\)/)).not.toBeInTheDocument()
  })

  it('replaces the rule content editor with the hidden-for-role note when editing', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId('edit-ruleset-uuid-1'))

    expect(await screen.findByTestId('ruleset-content-hidden')).toHaveTextContent('Hidden for your role.')
    expect(screen.queryByTestId('ruleset-content-input')).not.toBeInTheDocument()
  })

  it('does not allow saving an edit while the rule content is withheld', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId('edit-ruleset-uuid-1'))
    await screen.findByTestId('ruleset-content-hidden')
    expect(screen.getByRole('button', { name: 'Update Rule Set' })).toBeDisabled()
  })
})

describe('WafConfig for administrators', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(undefined as unknown as securityApi.SecurityStatus)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue({ config: null })
    vi.mocked(securityApi.getRuleSets).mockResolvedValue({
      rulesets: [{ ...redactedRuleSet, has_content: undefined, content_line_count: undefined, content: 'SecRuleEngine On\nSecRule ARGS "x"' }],
    })
    setRole('admin')
  })

  it('counts the rules from the content when the server sends it', async () => {
    renderPage()

    expect(await screen.findByText('2 rule(s)')).toBeInTheDocument()
  })

  it('shows the content editor when editing', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId('edit-ruleset-uuid-1'))

    expect(await screen.findByTestId('ruleset-content-input')).toHaveValue('SecRuleEngine On\nSecRule ARGS "x"')
    expect(screen.queryByTestId('ruleset-content-hidden')).not.toBeInTheDocument()
  })
})
