import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ProxyHostForm from '../ProxyHostForm'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

vi.mock('../../api/uptime', () => ({
  syncMonitors: vi.fn(() => Promise.resolve({})),
}))

// Minimal hook mocks used by the component
vi.mock('../../hooks/useRemoteServers', () => ({
  useRemoteServers: vi.fn(() => ({
    servers: [],
    isLoading: false,
    error: null,
    createRemoteServer: vi.fn(),
    updateRemoteServer: vi.fn(),
    deleteRemoteServer: vi.fn(),
  })),
}))

vi.mock('../../hooks/useDocker', () => ({
  useDocker: vi.fn(() => ({ containers: [], isLoading: false, error: null, refetch: vi.fn() })),
}))

vi.mock('../../hooks/useDomains', () => ({
  useDomains: vi.fn(() => ({ domains: [], createDomain: vi.fn().mockResolvedValue({}), isLoading: false, error: null })),
}))

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })),
}))

// stub global fetch for health endpoint
vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ json: () => Promise.resolve({ internal_ip: '127.0.0.1' }) })))

describe('ProxyHostForm Add Uptime flow', () => {
  it('submits host and requests uptime sync when Add Uptime is checked', async () => {
    const onSubmit = vi.fn(() => Promise.resolve())
    const onCancel = vi.fn()

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    render(
      <QueryClientProvider client={queryClient}>
        <ProxyHostForm onSubmit={onSubmit} onCancel={onCancel} />
      </QueryClientProvider>
    )

    // Fill required fields
    await userEvent.type(screen.getByPlaceholderText('My Service'), 'My Service')
    await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), 'example.com')
    await userEvent.type(screen.getByLabelText(/^Host$/), '127.0.0.1')
    await userEvent.clear(screen.getByLabelText(/^Port$/))
    await userEvent.type(screen.getByLabelText(/^Port$/), '8080')

    // Check Add Uptime
    const addUptimeCheckbox = screen.getByLabelText(/Add Uptime monitoring for this host/i)
    await userEvent.click(addUptimeCheckbox)

    // Adjust uptime options — locate the container for the uptime inputs
    const uptimeCheckbox = screen.getByLabelText(/Add Uptime monitoring for this host/i)
    const uptimeContainer = uptimeCheckbox.closest('label')?.parentElement
    if (!uptimeContainer) throw new Error('Uptime container not found')

    const { within } = await import('@testing-library/react')
    const spinbuttons = within(uptimeContainer).getAllByRole('spinbutton')
    // first spinbutton is interval, second is max retries
    fireEvent.change(spinbuttons[0], { target: { value: '30' } })
    fireEvent.change(spinbuttons[1], { target: { value: '2' } })

    // Submit
    const submitBtn = document.querySelector('button[type="submit"]') as HTMLButtonElement
    if (!submitBtn) throw new Error('Submit button not found')
    await userEvent.click(submitBtn)

    // wait for onSubmit to have been called
    await waitFor(() => expect(onSubmit).toHaveBeenCalled())

    // Ensure uptime API was called with provided options
    const uptime = await import('../../api/uptime')
    await waitFor(() => expect(uptime.syncMonitors).toHaveBeenCalledWith({ interval: 30, max_retries: 2 }))

    // Ensure onSubmit payload does not include temporary uptime keys
    const onSubmitMock = onSubmit as unknown as import('vitest').Mock
    const submittedPayload = onSubmitMock.mock.calls[0][0]
    expect(submittedPayload).not.toHaveProperty('addUptime')
    expect(submittedPayload).not.toHaveProperty('uptimeInterval')
    expect(submittedPayload).not.toHaveProperty('uptimeMaxRetries')
  })
})
