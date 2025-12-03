import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import WafConfig from '../WafConfig'
import * as securityApi from '../../api/security'
import type { SecurityRuleSet, RuleSetsResponse } from '../../api/security'

vi.mock('../../api/security')

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

const renderWithProviders = (ui: React.ReactNode) => {
  const qc = createQueryClient()
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>{ui}</BrowserRouter>
    </QueryClientProvider>
  )
}

const mockRuleSet: SecurityRuleSet = {
  id: 1,
  uuid: 'uuid-1',
  name: 'OWASP CRS',
  source_url: '',
  mode: 'blocking',
  last_updated: '2024-01-15T10:00:00Z',
  content: 'SecRule REQUEST_URI "@contains /admin" "id:1000,phase:1,deny,status:403"',
}

describe('WafConfig page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('shows loading state while fetching rulesets', async () => {
    // Keep the promise pending to test loading state
    vi.mocked(securityApi.getRuleSets).mockReturnValue(new Promise(() => {}))

    renderWithProviders(<WafConfig />)

    expect(screen.getByTestId('waf-loading')).toBeInTheDocument()
    expect(screen.getByText('Loading WAF configuration...')).toBeInTheDocument()
  })

  it('shows error state when fetch fails', async () => {
    vi.mocked(securityApi.getRuleSets).mockRejectedValue(new Error('Network error'))

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('waf-error')).toBeInTheDocument()
    })
    expect(screen.getByText(/Failed to load WAF configuration/)).toBeInTheDocument()
    expect(screen.getByText(/Network error/)).toBeInTheDocument()
  })

  it('shows empty state when no rulesets exist', async () => {
    const response: RuleSetsResponse = { rulesets: [] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('waf-empty-state')).toBeInTheDocument()
    })
    expect(screen.getByText('No Rule Sets')).toBeInTheDocument()
    expect(screen.getByText(/Create your first WAF rule set/)).toBeInTheDocument()
  })

  it('renders rulesets table when data exists', async () => {
    const response: RuleSetsResponse = { rulesets: [mockRuleSet] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('rulesets-table')).toBeInTheDocument()
    })
    expect(screen.getByText('OWASP CRS')).toBeInTheDocument()
    expect(screen.getByText('Blocking')).toBeInTheDocument()
    expect(screen.getByText('Inline')).toBeInTheDocument()
  })

  it('shows create form when Add Rule Set button is clicked', async () => {
    const response: RuleSetsResponse = { rulesets: [] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('create-ruleset-btn')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('create-ruleset-btn'))

    expect(screen.getByRole('heading', { name: 'Create Rule Set' })).toBeInTheDocument()
    expect(screen.getByTestId('ruleset-name-input')).toBeInTheDocument()
    expect(screen.getByTestId('ruleset-content-input')).toBeInTheDocument()
  })

  it('submits new ruleset and closes form on success', async () => {
    const response: RuleSetsResponse = { rulesets: [] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)
    vi.mocked(securityApi.upsertRuleSet).mockResolvedValue({ id: 1 })

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('create-ruleset-btn')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('create-ruleset-btn'))

    // Fill in the form
    await userEvent.type(screen.getByTestId('ruleset-name-input'), 'Test Rules')
    await userEvent.type(
      screen.getByTestId('ruleset-content-input'),
      'SecRule ARGS "@contains test" "id:1,phase:1,deny"'
    )

    // Submit
    const submitBtn = screen.getByRole('button', { name: 'Create Rule Set' })
    await userEvent.click(submitBtn)

    await waitFor(() => {
      expect(securityApi.upsertRuleSet).toHaveBeenCalledWith({
        id: undefined,
        name: 'Test Rules',
        source_url: undefined,
        content: 'SecRule ARGS "@contains test" "id:1,phase:1,deny"',
        mode: 'blocking',
      })
    })
  })

  it('opens edit form when edit button is clicked', async () => {
    const response: RuleSetsResponse = { rulesets: [mockRuleSet] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('rulesets-table')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('edit-ruleset-1'))

    expect(screen.getByText('Edit Rule Set')).toBeInTheDocument()
    expect(screen.getByDisplayValue('OWASP CRS')).toBeInTheDocument()
  })

  it('opens delete confirmation dialog and deletes on confirm', async () => {
    const response: RuleSetsResponse = { rulesets: [mockRuleSet] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)
    vi.mocked(securityApi.deleteRuleSet).mockResolvedValue(undefined)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('rulesets-table')).toBeInTheDocument()
    })

    // Click delete button
    await userEvent.click(screen.getByTestId('delete-ruleset-1'))

    // Confirm dialog should appear
    expect(screen.getByText('Delete Rule Set')).toBeInTheDocument()
    expect(screen.getByText(/Are you sure you want to delete "OWASP CRS"/)).toBeInTheDocument()

    // Confirm deletion
    await userEvent.click(screen.getByTestId('confirm-delete-btn'))

    await waitFor(() => {
      expect(securityApi.deleteRuleSet).toHaveBeenCalledWith(1)
    })
  })

  it('cancels delete when clicking cancel button', async () => {
    const response: RuleSetsResponse = { rulesets: [mockRuleSet] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('rulesets-table')).toBeInTheDocument()
    })

    // Click delete button
    await userEvent.click(screen.getByTestId('delete-ruleset-1'))

    // Click cancel
    await userEvent.click(screen.getByText('Cancel'))

    // Dialog should be closed
    await waitFor(() => {
      expect(screen.queryByText('Delete Rule Set')).not.toBeInTheDocument()
    })
    expect(securityApi.deleteRuleSet).not.toHaveBeenCalled()
  })

  it('cancels delete when clicking backdrop', async () => {
    const response: RuleSetsResponse = { rulesets: [mockRuleSet] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('rulesets-table')).toBeInTheDocument()
    })

    // Click delete button
    await userEvent.click(screen.getByTestId('delete-ruleset-1'))

    // Click backdrop
    await userEvent.click(screen.getByTestId('confirm-dialog-backdrop'))

    // Dialog should be closed
    await waitFor(() => {
      expect(screen.queryByText('Delete Rule Set')).not.toBeInTheDocument()
    })
  })

  it('displays mode correctly for detection-only rulesets', async () => {
    const detectionRuleset: SecurityRuleSet = {
      ...mockRuleSet,
      mode: 'detection',
    }
    const response: RuleSetsResponse = { rulesets: [detectionRuleset] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('rulesets-table')).toBeInTheDocument()
    })

    expect(screen.getByText('Detection')).toBeInTheDocument()
  })

  it('displays URL link when source_url is provided', async () => {
    const urlRuleset: SecurityRuleSet = {
      ...mockRuleSet,
      source_url: 'https://example.com/rules.conf',
      content: '',
    }
    const response: RuleSetsResponse = { rulesets: [urlRuleset] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('rulesets-table')).toBeInTheDocument()
    })

    const urlLink = screen.getByText('URL')
    expect(urlLink).toHaveAttribute('href', 'https://example.com/rules.conf')
    expect(urlLink).toHaveAttribute('target', '_blank')
  })

  it('validates form - submit disabled without name', async () => {
    const response: RuleSetsResponse = { rulesets: [] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('create-ruleset-btn')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('create-ruleset-btn'))

    // Only add content, no name
    await userEvent.type(screen.getByTestId('ruleset-content-input'), 'SecRule test')

    const submitBtn = screen.getByRole('button', { name: 'Create Rule Set' })
    expect(submitBtn).toBeDisabled()
  })

  it('validates form - submit disabled without content or URL', async () => {
    const response: RuleSetsResponse = { rulesets: [] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('create-ruleset-btn')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('create-ruleset-btn'))

    // Only add name, no content or URL
    await userEvent.type(screen.getByTestId('ruleset-name-input'), 'Test')

    const submitBtn = screen.getByRole('button', { name: 'Create Rule Set' })
    expect(submitBtn).toBeDisabled()
  })

  it('allows form submission with URL instead of content', async () => {
    const response: RuleSetsResponse = { rulesets: [] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)
    vi.mocked(securityApi.upsertRuleSet).mockResolvedValue({ id: 1 })

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('create-ruleset-btn')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('create-ruleset-btn'))

    // Add name and URL, no content
    await userEvent.type(screen.getByTestId('ruleset-name-input'), 'Remote Rules')
    await userEvent.type(screen.getByTestId('ruleset-url-input'), 'https://example.com/rules.conf')

    const submitBtn = screen.getByRole('button', { name: 'Create Rule Set' })
    expect(submitBtn).not.toBeDisabled()

    await userEvent.click(submitBtn)

    await waitFor(() => {
      expect(securityApi.upsertRuleSet).toHaveBeenCalledWith({
        id: undefined,
        name: 'Remote Rules',
        source_url: 'https://example.com/rules.conf',
        content: undefined,
        mode: 'blocking',
      })
    })
  })

  it('toggles between blocking and detection mode', async () => {
    const response: RuleSetsResponse = { rulesets: [] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)
    vi.mocked(securityApi.upsertRuleSet).mockResolvedValue({ id: 1 })

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('create-ruleset-btn')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('create-ruleset-btn'))

    // Fill required fields
    await userEvent.type(screen.getByTestId('ruleset-name-input'), 'Test')
    await userEvent.type(screen.getByTestId('ruleset-content-input'), 'SecRule test')

    // Select detection mode
    await userEvent.click(screen.getByTestId('mode-detection'))

    // Verify mode description changed
    expect(screen.getByText(/Malicious requests will be logged but not blocked/)).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Create Rule Set' }))

    await waitFor(() => {
      expect(securityApi.upsertRuleSet).toHaveBeenCalledWith(
        expect.objectContaining({ mode: 'detection' })
      )
    })
  })

  it('hides form when cancel is clicked', async () => {
    const response: RuleSetsResponse = { rulesets: [] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('create-ruleset-btn')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByTestId('create-ruleset-btn'))
    expect(screen.getByRole('heading', { name: 'Create Rule Set' })).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    // Form should be hidden, empty state visible
    await waitFor(() => {
      expect(screen.getByTestId('waf-empty-state')).toBeInTheDocument()
    })
  })

  it('updates existing ruleset correctly', async () => {
    const response: RuleSetsResponse = { rulesets: [mockRuleSet] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)
    vi.mocked(securityApi.upsertRuleSet).mockResolvedValue({ id: 1 })

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('rulesets-table')).toBeInTheDocument()
    })

    // Open edit form
    await userEvent.click(screen.getByTestId('edit-ruleset-1'))

    // Update name
    const nameInput = screen.getByTestId('ruleset-name-input')
    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, 'Updated CRS')

    // Submit
    await userEvent.click(screen.getByText('Update Rule Set'))

    await waitFor(() => {
      expect(securityApi.upsertRuleSet).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 1,
          name: 'Updated CRS',
        })
      )
    })
  })

  it('opens delete from edit form', async () => {
    const response: RuleSetsResponse = { rulesets: [mockRuleSet] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('rulesets-table')).toBeInTheDocument()
    })

    // Open edit form
    await userEvent.click(screen.getByTestId('edit-ruleset-1'))

    // Click delete button in edit form header
    const deleteBtn = screen.getByRole('button', { name: /delete/i })
    await userEvent.click(deleteBtn)

    // Confirm dialog should appear
    expect(screen.getByText('Delete Rule Set')).toBeInTheDocument()
  })

  it('counts rules correctly in table', async () => {
    const multiRuleSet: SecurityRuleSet = {
      ...mockRuleSet,
      content: `SecRule ARGS "@contains test1" "id:1,phase:1,deny"
SecRule ARGS "@contains test2" "id:2,phase:1,deny"
SecRule ARGS "@contains test3" "id:3,phase:1,deny"`,
    }
    const response: RuleSetsResponse = { rulesets: [multiRuleSet] }
    vi.mocked(securityApi.getRuleSets).mockResolvedValue(response)

    renderWithProviders(<WafConfig />)

    await waitFor(() => {
      expect(screen.getByTestId('rulesets-table')).toBeInTheDocument()
    })

    expect(screen.getByText('3 rule(s)')).toBeInTheDocument()
  })
})
