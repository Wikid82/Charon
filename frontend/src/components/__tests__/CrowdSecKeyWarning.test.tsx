import { render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import userEvent from '@testing-library/user-event'
import { CrowdSecKeyWarning } from '../CrowdSecKeyWarning'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import * as crowdsecApi from '../../api/crowdsec'
import { toast } from '../../utils/toast'

vi.mock('../../api/crowdsec')
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    ready: true,
  }),
}))
// Mock toast
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  })
  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  Wrapper.displayName = 'QueryClientWrapper'
  return Wrapper
}

describe('CrowdSecKeyWarning', () => {
  const defaultStatus = {
    key_source: 'env' as const,
    env_key_rejected: true,
    full_key: 'new-valid-key',
    current_key_preview: 'old...',
    message: 'Key rejected',
  }

  beforeEach(() => {
    vi.clearAllMocks()
    // Clear localStorage
    localStorage.clear()
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: vi.fn() },
      configurable: true,
    })
  })

  it('renders when key is rejected (missing/invalid)', async () => {
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue(defaultStatus)

    render(<CrowdSecKeyWarning />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('security.crowdsec.keyWarning.title')).toBeInTheDocument()
    })
  })

  it('returns null when key is valid (present)', async () => {
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue({
      key_source: 'env',
      env_key_rejected: false,
      current_key_preview: 'valid...',
      message: 'OK',
    })

    const { container } = render(<CrowdSecKeyWarning />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(crowdsecApi.getCrowdsecKeyStatus).toHaveBeenCalled()
    })

    expect(container).toBeEmptyDOMElement()
  })

  it('does not render when dismissed for the same key', async () => {
    localStorage.setItem('crowdsec-key-warning-dismissed', JSON.stringify({
      dismissed: true,
      key: defaultStatus.full_key,
    }))
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue(defaultStatus)

    const { container } = render(<CrowdSecKeyWarning />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(crowdsecApi.getCrowdsecKeyStatus).toHaveBeenCalled()
    })

    expect(container).toBeEmptyDOMElement()
  })

  it('re-renders when dismissal key differs', async () => {
    localStorage.setItem('crowdsec-key-warning-dismissed', JSON.stringify({
      dismissed: true,
      key: 'old-key',
    }))
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue(defaultStatus)

    render(<CrowdSecKeyWarning />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('security.crowdsec.keyWarning.title')).toBeInTheDocument()
    })
  })

  it('copies the key and toggles the copied state', async () => {
    const user = userEvent.setup()
    const clipboardWrite = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: clipboardWrite },
      configurable: true,
    })
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue(defaultStatus)

    render(<CrowdSecKeyWarning />, { wrapper: createWrapper() })

    const copyButton = await screen.findByRole('button', {
      name: 'security.crowdsec.keyWarning.copyButton',
    })

    await user.click(copyButton)

    expect(clipboardWrite).toHaveBeenCalledWith(defaultStatus.full_key)
    expect(toast.success).toHaveBeenCalledWith('security.crowdsec.keyWarning.copied')
    expect(
      screen.getByRole('button', { name: 'security.crowdsec.keyWarning.copied' })
    ).toBeInTheDocument()
  })

  it('shows a toast when copy fails', async () => {
    const user = userEvent.setup()
    const clipboardWrite = vi.fn().mockRejectedValue(new Error('copy failed'))
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: clipboardWrite },
      configurable: true,
    })
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue(defaultStatus)

    render(<CrowdSecKeyWarning />, { wrapper: createWrapper() })

    const copyButton = await screen.findByRole('button', {
      name: 'security.crowdsec.keyWarning.copyButton',
    })
    await user.click(copyButton)

    expect(toast.error).toHaveBeenCalledWith('security.crowdsec.copyFailed')
  })

  it('toggles key visibility', async () => {
    const user = userEvent.setup()
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue(defaultStatus)

    render(<CrowdSecKeyWarning />, { wrapper: createWrapper() })

    const codeBlock = await screen.findByText(/CHARON_SECURITY_CROWDSEC_API_KEY=/)
    expect(codeBlock).not.toHaveTextContent(defaultStatus.full_key)

    const showButton = screen.getByTitle('Show key')
    await user.click(showButton)

    expect(codeBlock).toHaveTextContent(defaultStatus.full_key)
    expect(screen.getByTitle('Hide key')).toBeInTheDocument()
  })

  it('persists dismissal when closed', async () => {
    const user = userEvent.setup()
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue(defaultStatus)

    const { container } = render(<CrowdSecKeyWarning />, { wrapper: createWrapper() })

    const closeButton = await screen.findByRole('button', { name: 'common.close' })
    await user.click(closeButton)

    expect(localStorage.getItem('crowdsec-key-warning-dismissed')).toContain(defaultStatus.full_key)
    expect(container).toBeEmptyDOMElement()
  })
})
