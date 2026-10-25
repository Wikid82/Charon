import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import en from '../../locales/en/translation.json'
import { CrowdSecKeyWarning } from '../CrowdSecKeyWarning'

vi.mock('../../api/crowdsec')

// Resolve keys against the real English catalog so the assertions check the shipped wording
vi.mock('react-i18next', async () => {
  const catalog: Record<string, unknown> = (await import('../../locales/en/translation.json')).default
  const lookup = (key: string): string =>
    key.split('.').reduce<unknown>(
      (node, part) => (typeof node === 'object' && node !== null ? (node as Record<string, unknown>)[part] : undefined),
      catalog
    ) as string
  return {
    useTranslation: () => ({ t: (key: string) => lookup(key) ?? key, ready: true }),
  }
})

const renderWithClient = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <CrowdSecKeyWarning />
    </QueryClientProvider>
  )
}

describe('CrowdSecKeyWarning English copy', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
  })

  it('describes the saved key Charon now uses and asks to remove the stale variable', async () => {
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue({
      key_source: 'auto-generated',
      env_key_rejected: true,
      full_key: 'generated-key-0123456789abcdef',
      current_key_preview: 'gene...',
      rejected_key_preview: 'old...',
      message: 'rejected',
    })

    renderWithClient()

    expect(
      await screen.findByText(en.security.crowdsec.keyWarning.description)
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        'Remove the old CrowdSec key variable from docker-compose.yml. If you want to keep one, set it to this key:'
      )
    ).toBeInTheDocument()
    expect(screen.getByText('No restart is needed: Charon already uses this key.')).toBeInTheDocument()
    expect(screen.queryByText(/restart the container/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/update your docker-compose/i)).not.toBeInTheDocument()
  })

  it('renders nothing when the key source is none and no env key was rejected', async () => {
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue({
      key_source: 'none',
      env_key_rejected: false,
      current_key_preview: '',
      message: 'No CrowdSec API key configured',
    })

    const { container } = renderWithClient()

    await waitFor(() => {
      expect(crowdsecApi.getCrowdsecKeyStatus).toHaveBeenCalled()
    })
    expect(container).toBeEmptyDOMElement()
  })
})
