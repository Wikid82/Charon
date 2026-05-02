import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import HecateAgent from '../HecateAgent'

vi.mock('../../hooks/useOrthrus', () => ({
  useAgentList: vi.fn(),
  useProvisionAgent: vi.fn(),
  useOrthrus: vi.fn(),
}))

vi.mock('../../components/hecate/OrthrusAgentManager', () => ({
  OrthrusAgentManager: ({ agents }: { agents: unknown[] }) => (
    <div data-testid="agent-manager">Agent count: {agents.length}</div>
  ),
}))

vi.mock('../../components/hecate/OrthrusInstallWizard', () => ({
  OrthrusInstallWizard: ({ open, onClose }: { open: boolean; onClose: () => void }) =>
    open ? <div data-testid="install-wizard"><button onClick={onClose}>Close Wizard</button></div> : null,
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

import { useAgentList, useProvisionAgent, useOrthrus } from '../../hooks/useOrthrus'

const mockUseAgentList = vi.mocked(useAgentList)
const mockUseProvisionAgent = vi.mocked(useProvisionAgent)
const mockUseOrthrus = vi.mocked(useOrthrus)

const renderComponent = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <HecateAgent />
      </BrowserRouter>
    </QueryClientProvider>
  )
}

describe('HecateAgent', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAgentList.mockReturnValue({ data: [], isLoading: false, error: null } as unknown as ReturnType<typeof useAgentList>)
    mockUseProvisionAgent.mockReturnValue({ mutateAsync: vi.fn() } as unknown as ReturnType<typeof useProvisionAgent>)
    mockUseOrthrus.mockReturnValue({ getInstallSnippets: vi.fn() } as unknown as ReturnType<typeof useOrthrus>)
  })

  it('renders the agent manager component', async () => {
    renderComponent()
    expect(await screen.findByTestId('agent-manager')).toBeInTheDocument()
  })

  it('renders the provision agent button', async () => {
    renderComponent()
    expect(await screen.findByText('hecate.page.provisionAgent')).toBeInTheDocument()
  })

  it('opens the provision dialog when button is clicked', async () => {
    const user = userEvent.setup()
    renderComponent()
    const provisionBtn = await screen.findByRole('button', { name: /hecate.page.provisionAgent/i })
    await user.click(provisionBtn)
    await waitFor(() => {
      const dialog = screen.getByRole('dialog')
      expect(dialog).toBeInTheDocument()
    })
  })

  it('shows agents in agent manager', async () => {
    mockUseAgentList.mockReturnValue({
      data: [
        { uuid: 'agent-1', name: 'Agent One', status: 'online', last_seen: null, created_at: '' },
      ],
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useAgentList>)
    renderComponent()
    expect(await screen.findByText('Agent count: 1')).toBeInTheDocument()
  })
})
