import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { act } from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import RedirectionHostForm from '../RedirectionHostForm'

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(() => ({
    certificates: [
      { id: 1, uuid: 'cert-1', name: 'Cert 1', domains: 'example.com', provider: 'letsencrypt', issuer: 'LE', status: 'valid', has_key: true, in_use: true, expires_at: '2027-01-01' },
    ],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useDNSProviders', () => ({
  useDNSProviders: vi.fn(() => ({ data: [], isLoading: false })),
}))

// RedirectionHostForm's certificate Select only ever emits 'none' or an
// id:/uuid:-prefixed token (via getEntityToken), so resolveTokenToFormValue's
// bare-numeric-string and generic-string fallback branches can never be
// reached by driving the real Radix Select through the DOM. This mock lets
// the test emit those raw tokens directly, mirroring the established pattern
// in ProxyHostForm-token-coverage.test.tsx for the same duplicated helper.
vi.mock('../ui/Select', () => {
  const findText = (children: React.ReactNode): string => {
    if (typeof children === 'string') return children
    if (Array.isArray(children)) return children.map((child) => findText(child)).join(' ')
    if (children && typeof children === 'object' && 'props' in children) {
      const node = children as { props?: { children?: React.ReactNode } }
      return findText(node.props?.children)
    }
    return ''
  }

  const Select = ({
    value,
    onValueChange,
    children,
  }: {
    value?: string
    onValueChange?: (value: string) => void
    children?: React.ReactNode
  }) => {
    const text = findText(children)
    const isCertificateSelect = text.includes("Let's Encrypt")

    return (
      <div>
        {isCertificateSelect && (
          <>
            <div data-testid="certificate-select-value">{value}</div>
            <button type="button" onClick={() => onValueChange?.('5')}>
              emit-plain-numeric
            </button>
            <button type="button" onClick={() => onValueChange?.('legacy-cert-token')}>
              emit-generic-fallback
            </button>
          </>
        )}
        {children}
      </div>
    )
  }

  const SelectTrigger = ({ children, ...rest }: React.ComponentProps<'button'>) => (
    <button type="button" {...rest}>
      {children}
    </button>
  )
  const SelectContent = ({ children }: { children?: React.ReactNode }) => <div>{children}</div>
  const SelectItem = ({ children }: { value: string; children?: React.ReactNode }) => <div>{children}</div>
  const SelectValue = () => <span />

  return { Select, SelectTrigger, SelectContent, SelectItem, SelectValue }
})

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })

const renderForm = async (onSubmit: (data: Record<string, unknown>) => Promise<void>) => {
  await act(async () => {
    render(
      <QueryClientProvider client={queryClient}>
        <RedirectionHostForm onSubmit={onSubmit} onCancel={vi.fn()} />
      </QueryClientProvider>
    )
  })
}

describe('RedirectionHostForm certificate token normalization edge cases', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('normalizes a bare numeric token (no id:/uuid: prefix) to a number on submit', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    await renderForm(onSubmit)

    await userEvent.type(screen.getByLabelText(/domain names/i), 'example.com')
    await userEvent.type(screen.getByLabelText(/target url/i), 'https://elsewhere.example.com')
    await userEvent.click(screen.getByRole('button', { name: 'emit-plain-numeric' }))
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ certificate_id: 5 }))
    })
  })

  it('preserves an unrecognized token as an opaque string on submit', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    await renderForm(onSubmit)

    await userEvent.type(screen.getByLabelText(/domain names/i), 'example.com')
    await userEvent.type(screen.getByLabelText(/target url/i), 'https://elsewhere.example.com')
    await userEvent.click(screen.getByRole('button', { name: 'emit-generic-fallback' }))
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ certificate_id: 'legacy-cert-token' }))
    })
  })
})
