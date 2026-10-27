import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as securityApi from '../../api/security'
import WafConfig from '../WafConfig'

import type { RuleSetsResponse, SecurityRuleSet } from '../../api/security'

vi.mock('../../api/security')

const ruleSet = (overrides: Partial<SecurityRuleSet> = {}): SecurityRuleSet => ({
  uuid: 'uuid-1',
  name: 'OWASP CRS',
  source_url: '',
  mode: 'blocking',
  last_updated: '2024-01-15T10:00:00Z',
  content: 'SecRule REQUEST_URI "@contains /admin" "id:1000,phase:1,deny,status:403"',
  ...overrides,
})

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

const pending = () => new Promise<never>(() => {})

describe('WafConfig page pending and edge states', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(undefined as unknown as securityApi.SecurityStatus)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue({ config: null })
  })

  it('shows the empty state when the response has no rule set list', async () => {
    vi.mocked(securityApi.getRuleSets).mockResolvedValue({} as RuleSetsResponse)
    renderPage()

    expect(await screen.findByTestId('waf-empty-state')).toBeInTheDocument()
  })

  it('shows a dash when a rule set has no update time', async () => {
    vi.mocked(securityApi.getRuleSets).mockResolvedValue({ rulesets: [ruleSet({ last_updated: '' })] })
    renderPage()

    expect(await screen.findByTestId('rulesets-table')).toBeInTheDocument()
    expect(screen.getByText('-')).toBeInTheDocument()
  })

  it('opens the edit form with the detection mode selected for a detection rule set', async () => {
    vi.mocked(securityApi.getRuleSets).mockResolvedValue({ rulesets: [ruleSet({ mode: 'detection' })] })
    renderPage()

    await userEvent.click(await screen.findByTestId('edit-ruleset-uuid-1'))

    expect(screen.getByTestId('mode-detection')).toBeChecked()
  })

  it('shows the load failure text when the error is not an Error instance', async () => {
    vi.mocked(securityApi.getRuleSets).mockRejectedValue('boom')
    renderPage()

    expect(await screen.findByTestId('waf-error')).toBeInTheDocument()
  })

  it('returns to no preset when the preset choice is cleared', async () => {
    vi.mocked(securityApi.getRuleSets).mockResolvedValue({ rulesets: [] })
    renderPage()

    await userEvent.click(await screen.findByTestId('create-ruleset-btn'))
    const presets = screen.getByTestId('preset-select') as HTMLSelectElement
    await userEvent.selectOptions(presets, 'Common Bad Bots')
    await userEvent.selectOptions(presets, '')

    expect(presets.value).toBe('')
  })

  it('shows the forging overlay while a new rule set is saved', async () => {
    vi.mocked(securityApi.getRuleSets).mockResolvedValue({ rulesets: [ruleSet()] })
    vi.mocked(securityApi.upsertRuleSet).mockReturnValue(pending())
    renderPage()

    await userEvent.click(await screen.findByTestId('create-ruleset-btn'))
    await userEvent.type(screen.getByTestId('ruleset-name-input'), 'Fresh rules')
    await userEvent.type(screen.getByTestId('ruleset-content-input'), 'SecRule ARGS "@contains x" "id:1,deny"')
    await userEvent.click(screen.getByRole('button', { name: 'Create Rule Set' }))

    expect(await screen.findByText('Forging new defenses...')).toBeInTheDocument()
  })

  it('shows the awakening overlay while an existing rule set is saved', async () => {
    vi.mocked(securityApi.getRuleSets).mockResolvedValue({ rulesets: [ruleSet()] })
    vi.mocked(securityApi.upsertRuleSet).mockReturnValue(pending())
    renderPage()

    await userEvent.click(await screen.findByTestId('edit-ruleset-uuid-1'))
    await userEvent.click(screen.getByRole('button', { name: 'Update Rule Set' }))

    expect(await screen.findByText('Cerberus awakens...')).toBeInTheDocument()
    await waitFor(() => expect(securityApi.upsertRuleSet).toHaveBeenCalledTimes(1))
  })

  it('shows the removal overlay while a rule set is being deleted', async () => {
    vi.mocked(securityApi.getRuleSets).mockResolvedValue({ rulesets: [ruleSet()] })
    vi.mocked(securityApi.deleteRuleSet).mockReturnValue(pending())
    renderPage()

    await userEvent.click(await screen.findByTestId('delete-ruleset-uuid-1'))
    await userEvent.click(screen.getByTestId('confirm-delete-btn'))

    expect(await screen.findByText('Lowering a barrier...')).toBeInTheDocument()
    await waitFor(() => expect(securityApi.deleteRuleSet).toHaveBeenCalledWith('uuid-1'))
  })
})
