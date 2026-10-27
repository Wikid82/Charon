import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as securityApi from '../../api/security'
import WafConfig from '../WafConfig'

import type { SecurityConfigResponse, SecurityRuleSet, SecurityStatus } from '../../api/security'

vi.mock('../../api/security')

const ruleSet = (uuid: string, name: string): SecurityRuleSet => ({
  uuid,
  name,
  source_url: '',
  mode: 'blocking',
  last_updated: '2024-01-15T10:00:00Z',
  content: 'SecRule REQUEST_URI "@contains /admin" "id:1000,phase:1,deny,status:403"',
})

const RULE_SETS = [ruleSet('uuid-1', 'OWASP CRS'), ruleSet('uuid-2', 'Custom Rules')]

const statusWith = (waf: Partial<SecurityStatus['waf']>): SecurityStatus => ({
  crowdsec: { mode: 'disabled', api_url: '', enabled: false },
  waf: { mode: 'enabled', enabled: true, ...waf },
  rate_limit: { enabled: false },
  acl: { enabled: false },
})

const configWith = (config: SecurityConfigResponse['config']): SecurityConfigResponse => ({ config })

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

const selectOf = () => screen.getByTestId('waf-rules-source-select') as HTMLSelectElement

describe('WafConfig rules source', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(securityApi.getRuleSets).mockResolvedValue({ rulesets: RULE_SETS })
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(statusWith({ enforced: undefined }))
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(
      configWith({ waf_mode: 'block', waf_rules_source: '' })
    )
    vi.mocked(securityApi.updateSecurityConfig).mockResolvedValue({})
  })

  it('lists automatic choice and every rule set, with the stored source selected', async () => {
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(
      configWith({ waf_mode: 'block', waf_rules_source: 'Custom Rules' })
    )
    renderPage()

    await waitFor(() => expect(selectOf()).toHaveValue('Custom Rules'))
    const options = within(selectOf()).getAllByRole('option').map((o) => o.textContent)
    expect(options).toEqual(['Choose automatically', 'OWASP CRS', 'Custom Rules'])
  })

  it('saves exactly the name and the chosen rules source', async () => {
    renderPage()
    await waitFor(() => expect(selectOf()).toBeInTheDocument())

    await userEvent.selectOptions(selectOf(), 'OWASP CRS')

    await waitFor(() => expect(securityApi.updateSecurityConfig).toHaveBeenCalledTimes(1))
    expect(securityApi.updateSecurityConfig).toHaveBeenCalledWith({ name: 'default', waf_rules_source: 'OWASP CRS' })
  })

  it('saves the empty string when the admin chooses automatic selection', async () => {
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(
      configWith({ waf_mode: 'block', waf_rules_source: 'OWASP CRS' })
    )
    renderPage()
    await waitFor(() => expect(selectOf()).toHaveValue('OWASP CRS'))

    await userEvent.selectOptions(selectOf(), '')

    await waitFor(() => expect(securityApi.updateSecurityConfig).toHaveBeenCalledTimes(1))
    expect(securityApi.updateSecurityConfig).toHaveBeenCalledWith({ name: 'default', waf_rules_source: '' })
  })

  it('keeps a stored URL source visible as its own option', async () => {
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(
      configWith({ waf_mode: 'block', waf_rules_source: 'https://example.com/rules.tar.gz' })
    )
    renderPage()

    await waitFor(() => expect(selectOf()).toHaveValue('https://example.com/rules.tar.gz'))
    expect(within(selectOf()).getAllByRole('option')).toHaveLength(4)
  })

  it('does not add an option for a stored name that is a rule set', async () => {
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(
      configWith({ waf_mode: 'block', waf_rules_source: 'OWASP CRS' })
    )
    renderPage()

    await waitFor(() => expect(selectOf()).toHaveValue('OWASP CRS'))
    expect(within(selectOf()).getAllByRole('option')).toHaveLength(3)
  })

  it('shows automatic selection when the config row is not seeded yet', async () => {
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(configWith(null))
    renderPage()

    await waitFor(() => expect(selectOf()).toHaveValue(''))
  })

  it('renders without status or config data', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(undefined as unknown as SecurityStatus)
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(undefined as unknown as SecurityConfigResponse)
    renderPage()

    await waitFor(() => expect(selectOf()).toHaveValue(''))
    expect(screen.queryByTestId('waf-detection-mode')).not.toBeInTheDocument()
    expect(screen.queryByTestId('waf-enforcement-note')).not.toBeInTheDocument()
  })

  it('shows the detection-mode line when the WAF is on and the stored mode is monitor', async () => {
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(configWith({ waf_mode: 'monitor', waf_rules_source: '' }))
    renderPage()

    expect(await screen.findByTestId('waf-detection-mode')).toHaveTextContent(
      'Detection mode: requests that match the rules are logged, not blocked. Set a rule set\'s mode to blocking to block them.'
    )
  })

  it('hides the detection-mode line when the WAF is off', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(statusWith({ enabled: false, mode: 'disabled' }))
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue(configWith({ waf_mode: 'monitor', waf_rules_source: '' }))
    renderPage()

    await waitFor(() => expect(selectOf()).toBeInTheDocument())
    expect(screen.queryByTestId('waf-detection-mode')).not.toBeInTheDocument()
  })

  it('hides the detection-mode line when the stored mode is block', async () => {
    renderPage()

    await waitFor(() => expect(selectOf()).toBeInTheDocument())
    expect(screen.queryByTestId('waf-detection-mode')).not.toBeInTheDocument()
  })

  it('shows the applied-state line for a reason from the status', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(
      statusWith({ enforced: false, reason: 'ruleset_not_selected' })
    )
    renderPage()

    expect(await screen.findByTestId('waf-enforcement-note')).toHaveTextContent(
      'several rule sets exist and none is selected'
    )
  })

  it('does not show the in-use text for a delete that is not refused', async () => {
    vi.mocked(securityApi.deleteRuleSet).mockResolvedValue({})
    renderPage()

    await userEvent.click(await screen.findByTestId('delete-ruleset-uuid-1'))
    await userEvent.click(screen.getByTestId('confirm-delete-btn'))

    await waitFor(() => expect(securityApi.deleteRuleSet).toHaveBeenCalledWith('uuid-1'))
  })
})
