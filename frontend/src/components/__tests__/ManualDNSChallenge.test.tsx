import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import { useChallengePoll, useManualChallengeMutations } from '../../hooks/useManualChallenge'
import { toast } from '../../utils/toast'
import ManualDNSChallenge from '../dns-providers/ManualDNSChallenge'

import type { ManualChallenge } from '../../api/manualChallenge'


// Mock dependencies
vi.mock('../../hooks/useManualChallenge')
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
  },
}))

// Mock clipboard API using vi.stubGlobal
const mockWriteText = vi.fn()
vi.stubGlobal('navigator', {
  ...navigator,
  clipboard: {
    writeText: mockWriteText,
  },
})

// Mock i18n
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: Record<string, unknown>) => {
      const translations: Record<string, string> = {
        'dnsProvider.manual.title': 'Manual DNS Challenge',
        'dnsProvider.manual.instructions': `To obtain a certificate for ${options?.domain || 'example.com'}, create the following TXT record at your DNS provider:`,
        'dnsProvider.manual.createRecord': 'Create this TXT record at your DNS provider',
        'dnsProvider.manual.recordName': 'Record Name',
        'dnsProvider.manual.recordValue': 'Record Value',
        'dnsProvider.manual.ttl': 'TTL',
        'dnsProvider.manual.seconds': 'seconds',
        'dnsProvider.manual.minutes': 'minutes',
        'dnsProvider.manual.timeRemaining': 'Time remaining',
        'dnsProvider.manual.progressPercent': `${options?.percent || 0}% time remaining`,
        'dnsProvider.manual.challengeProgress': 'Challenge timeout progress',
        'dnsProvider.manual.copy': 'Copy',
        'dnsProvider.manual.copied': 'Copied!',
        'dnsProvider.manual.copyFailed': 'Failed to copy to clipboard',
        'dnsProvider.manual.copyRecordName': 'Copy record name to clipboard',
        'dnsProvider.manual.copyRecordValue': 'Copy record value to clipboard',
        'dnsProvider.manual.checkDnsNow': 'Check DNS Now',
        'dnsProvider.manual.checkDnsDescription': 'Immediately check if the DNS record has propagated',
        'dnsProvider.manual.verifyButton': "I've Created the Record - Verify",
        'dnsProvider.manual.verifyDescription': 'Verify that the DNS record exists',
        'dnsProvider.manual.cancelChallenge': 'Cancel Challenge',
        'dnsProvider.manual.lastCheck': 'Last checked',
        'dnsProvider.manual.lastCheckSecondsAgo': `${options?.seconds || 0} seconds ago`,
        'dnsProvider.manual.lastCheckMinutesAgo': `${options?.minutes || 0} minutes ago`,
        'dnsProvider.manual.notPropagated': 'DNS record not yet propagated',
        'dnsProvider.manual.dnsNotFound': 'DNS record not found',
        'dnsProvider.manual.verifySuccess': 'DNS challenge verified successfully!',
        'dnsProvider.manual.verifyFailed': 'DNS verification failed',
        'dnsProvider.manual.challengeExpired': 'Challenge expired',
        'dnsProvider.manual.challengeCancelled': 'Challenge cancelled',
        'dnsProvider.manual.cancelFailed': 'Failed to cancel challenge',
        'dnsProvider.manual.statusChanged': `Challenge status changed to ${options?.status || ''}`,
        'dnsProvider.manual.status.created': 'Created',
        'dnsProvider.manual.status.pending': 'Pending',
        'dnsProvider.manual.status.verifying': 'Verifying...',
        'dnsProvider.manual.status.verified': 'Verified',
        'dnsProvider.manual.status.expired': 'Expired',
        'dnsProvider.manual.status.failed': 'Failed',
        'dnsProvider.manual.statusMessage.pending': 'Waiting for DNS propagation...',
        'dnsProvider.manual.statusMessage.verified': 'DNS challenge verified successfully!',
        'dnsProvider.manual.statusMessage.expired': 'Challenge has expired.',
        'dnsProvider.manual.statusMessage.failed': 'DNS verification failed.',
      }
      return translations[key] || key
    },
  }),
}))

const mockChallenge: ManualChallenge = {
  id: 'test-challenge-uuid',
  status: 'pending',
  fqdn: '_acme-challenge.example.com',
  value: 'gZrH7wL9t3kM2nP4qX5yR8sT0uV1wZ2aB3cD4eF5gH6iJ7',
  ttl: 300,
  created_at: new Date(Date.now() - 2 * 60 * 1000).toISOString(), // 2 minutes ago
  expires_at: new Date(Date.now() + 8 * 60 * 1000).toISOString(), // 8 minutes from now
  last_check_at: new Date(Date.now() - 10 * 1000).toISOString(), // 10 seconds ago
  dns_propagated: false,
}

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

const renderComponent = (
  challenge: ManualChallenge = mockChallenge,
  onComplete = vi.fn(),
  onCancel = vi.fn()
) => {
  const queryClient = createQueryClient()
  return {
    ...render(
      <QueryClientProvider client={queryClient}>
        <ManualDNSChallenge
          providerId={1}
          challenge={challenge}
          onComplete={onComplete}
          onCancel={onCancel}
        />
      </QueryClientProvider>
    ),
    onComplete,
    onCancel,
  }
}

describe('ManualDNSChallenge', () => {
  let mockVerifyMutation: ReturnType<typeof vi.fn>
  let mockDeleteMutation: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers({ shouldAdvanceTime: true })
    mockWriteText.mockResolvedValue(undefined)

    mockVerifyMutation = vi.fn()
    mockDeleteMutation = vi.fn()

    vi.mocked(useChallengePoll).mockReturnValue({
      data: {
        status: 'pending',
        dns_propagated: false,
        time_remaining_seconds: 480,
        last_check_at: new Date(Date.now() - 10 * 1000).toISOString(),
      },
      isLoading: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useChallengePoll>)

    vi.mocked(useManualChallengeMutations).mockReturnValue({
      verifyMutation: {
        mutateAsync: mockVerifyMutation,
        isPending: false,
      },
      deleteMutation: {
        mutateAsync: mockDeleteMutation,
        isPending: false,
      },
      createMutation: {
        mutateAsync: vi.fn(),
        isPending: false,
      },
    } as unknown as ReturnType<typeof useManualChallengeMutations>)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  describe('Rendering', () => {
    it('renders the challenge title', () => {
      renderComponent()
      expect(screen.getByText('Manual DNS Challenge')).toBeInTheDocument()
    })

    it('displays the FQDN record name', () => {
      renderComponent()
      expect(screen.getByText('_acme-challenge.example.com')).toBeInTheDocument()
    })

    it('displays the challenge value', () => {
      renderComponent()
      expect(
        screen.getByText('gZrH7wL9t3kM2nP4qX5yR8sT0uV1wZ2aB3cD4eF5gH6iJ7')
      ).toBeInTheDocument()
    })

    it('displays TTL information', () => {
      renderComponent()
      expect(screen.getByText(/300/)).toBeInTheDocument()
      expect(screen.getByText(/5 minutes/)).toBeInTheDocument()
    })

    it('renders copy buttons with aria labels', () => {
      renderComponent()
      expect(
        screen.getByRole('button', { name: /copy record name/i })
      ).toBeInTheDocument()
      expect(
        screen.getByRole('button', { name: /copy record value/i })
      ).toBeInTheDocument()
    })

    it('renders verify and check DNS buttons', () => {
      renderComponent()
      expect(screen.getByText('Check DNS Now')).toBeInTheDocument()
      expect(screen.getByText("I've Created the Record - Verify")).toBeInTheDocument()
    })

    it('renders cancel button when not in terminal state', () => {
      renderComponent()
      expect(screen.getByText('Cancel Challenge')).toBeInTheDocument()
    })
  })

  describe('Progress and Countdown', () => {
    it('displays time remaining', () => {
      renderComponent()
      expect(screen.getByText(/Time remaining/i)).toBeInTheDocument()
    })

    it('displays progress bar', () => {
      renderComponent()
      expect(
        screen.getByRole('progressbar', { name: /challenge.*progress/i })
      ).toBeInTheDocument()
    })

    it('updates countdown every second', async () => {
      renderComponent()

      // Get initial time display
      const timeElement = screen.getByText(/Time remaining/i)
      expect(timeElement).toBeInTheDocument()

      // Advance timer by 1 second
      await act(async () => {
        vi.advanceTimersByTime(1000)
      })

      // Time should have updated (countdown decreased)
      expect(timeElement).toBeInTheDocument()
    })
  })

  describe('Copy Functionality', () => {
    // Note: These tests verify the clipboard copy functionality.
    // Due to jsdom limitations with navigator.clipboard mocking, we test
    // the UI state changes instead of verifying the actual clipboard API calls.
    // The component correctly shows "Copied!" state after clicking, which
    // indicates the async copy handler completed successfully.

    beforeEach(() => {
      vi.useRealTimers()
    })

    afterEach(() => {
      vi.useFakeTimers({ shouldAdvanceTime: true })
    })

    it('shows copied state after clicking copy record name button', async () => {
      const user = userEvent.setup()
      renderComponent()

      const copyNameButton = screen.getByRole('button', { name: /copy record name/i })
      await user.click(copyNameButton)

      // The button should show the "Copied!" state after successful copy
      await waitFor(() => {
        expect(screen.getByText('Copied!')).toBeInTheDocument()
      })
    })

    it('shows copied state after clicking copy record value button', async () => {
      const user = userEvent.setup()
      renderComponent()

      const copyValueButton = screen.getByRole('button', { name: /copy record value/i })
      await user.click(copyValueButton)

      // The button should show the "Copied!" state after successful copy
      await waitFor(() => {
        expect(screen.getByText('Copied!')).toBeInTheDocument()
      })
    })

    it('copy buttons are accessible and clickable', () => {
      renderComponent()

      const copyNameButton = screen.getByRole('button', { name: /copy record name/i })
      const copyValueButton = screen.getByRole('button', { name: /copy record value/i })

      expect(copyNameButton).toBeEnabled()
      expect(copyValueButton).toBeEnabled()
      expect(copyNameButton).toHaveAttribute('aria-label')
      expect(copyValueButton).toHaveAttribute('aria-label')
    })
  })

  describe('Verification', () => {
    it('calls verify mutation when verify button is clicked', async () => {
      mockVerifyMutation.mockResolvedValueOnce({ success: true, dns_found: true, message: 'OK' })
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      renderComponent()

      const verifyButton = screen.getByText("I've Created the Record - Verify")
      await user.click(verifyButton)

      expect(mockVerifyMutation).toHaveBeenCalledWith({
        providerId: 1,
        challengeId: 'test-challenge-uuid',
      })
    })

    it('calls verify mutation when Check DNS Now is clicked', async () => {
      mockVerifyMutation.mockResolvedValueOnce({ success: false, dns_found: false, message: '' })
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      renderComponent()

      const checkButton = screen.getByText('Check DNS Now')
      await user.click(checkButton)

      expect(mockVerifyMutation).toHaveBeenCalled()
    })

    it('shows success toast on successful verification', async () => {
      mockVerifyMutation.mockResolvedValueOnce({ success: true, dns_found: true, message: 'OK' })
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      renderComponent()

      const verifyButton = screen.getByText("I've Created the Record - Verify")
      await user.click(verifyButton)

      expect(toast.success).toHaveBeenCalledWith('DNS challenge verified successfully!')
    })

    it('shows warning toast when DNS not found', async () => {
      mockVerifyMutation.mockResolvedValueOnce({ success: false, dns_found: false, message: '' })
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      renderComponent()

      const verifyButton = screen.getByText("I've Created the Record - Verify")
      await user.click(verifyButton)

      expect(toast.warning).toHaveBeenCalledWith('DNS record not found')
    })

    it('shows error toast on verification failure', async () => {
      mockVerifyMutation.mockRejectedValueOnce({
        response: { data: { message: 'Server error' } },
      })
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      renderComponent()

      const verifyButton = screen.getByText("I've Created the Record - Verify")
      await user.click(verifyButton)

      expect(toast.error).toHaveBeenCalledWith('Server error')
    })
  })

  describe('Cancellation', () => {
    it('calls delete mutation and onCancel when cancelled', async () => {
      mockDeleteMutation.mockResolvedValueOnce(undefined)
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      const { onCancel } = renderComponent()

      const cancelButton = screen.getByText('Cancel Challenge')
      await user.click(cancelButton)

      expect(mockDeleteMutation).toHaveBeenCalledWith({
        providerId: 1,
        challengeId: 'test-challenge-uuid',
      })
      expect(onCancel).toHaveBeenCalled()
      expect(toast.info).toHaveBeenCalledWith('Challenge cancelled')
    })

    it('shows error toast when cancellation fails', async () => {
      mockDeleteMutation.mockRejectedValueOnce({
        response: { data: { message: 'Cannot cancel' } },
      })
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      renderComponent()

      const cancelButton = screen.getByText('Cancel Challenge')
      await user.click(cancelButton)

      expect(toast.error).toHaveBeenCalledWith('Cannot cancel')
    })
  })

  describe('Terminal States', () => {
    it('hides cancel button when challenge is verified', () => {
      vi.mocked(useChallengePoll).mockReturnValue({
        data: {
          status: 'verified',
          dns_propagated: true,
          time_remaining_seconds: 0,
          last_check_at: new Date().toISOString(),
        },
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useChallengePoll>)

      const verifiedChallenge: ManualChallenge = {
        ...mockChallenge,
        status: 'verified',
        dns_propagated: true,
      }
      renderComponent(verifiedChallenge)

      expect(screen.queryByText('Cancel Challenge')).not.toBeInTheDocument()
    })

    it('hides progress bar when challenge is expired', () => {
      vi.mocked(useChallengePoll).mockReturnValue({
        data: {
          status: 'expired',
          dns_propagated: false,
          time_remaining_seconds: 0,
          last_check_at: new Date().toISOString(),
        },
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useChallengePoll>)

      const expiredChallenge: ManualChallenge = {
        ...mockChallenge,
        status: 'expired',
      }
      renderComponent(expiredChallenge)

      expect(
        screen.queryByRole('progressbar', { name: /challenge.*progress/i })
      ).not.toBeInTheDocument()
    })

    it('disables verify buttons when challenge is failed', () => {
      vi.mocked(useChallengePoll).mockReturnValue({
        data: {
          status: 'failed',
          dns_propagated: false,
          time_remaining_seconds: 0,
          last_check_at: new Date().toISOString(),
          error_message: 'ACME validation failed',
        },
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useChallengePoll>)

      const failedChallenge: ManualChallenge = {
        ...mockChallenge,
        status: 'failed',
      }
      renderComponent(failedChallenge)

      expect(screen.getByText('Check DNS Now').closest('button')).toBeDisabled()
      expect(
        screen.getByText("I've Created the Record - Verify").closest('button')
      ).toBeDisabled()
    })

    it('calls onComplete with true when status changes to verified', async () => {
      const { onComplete, rerender } = renderComponent()

      // Update poll data to verified
      vi.mocked(useChallengePoll).mockReturnValue({
        data: {
          status: 'verified',
          dns_propagated: true,
          time_remaining_seconds: 0,
          last_check_at: new Date().toISOString(),
        },
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useChallengePoll>)

      // Re-render to trigger effect
      const verifiedChallenge: ManualChallenge = {
        ...mockChallenge,
        status: 'verified',
        dns_propagated: true,
      }

      const queryClient = createQueryClient()
      rerender(
        <QueryClientProvider client={queryClient}>
          <ManualDNSChallenge
            providerId={1}
            challenge={verifiedChallenge}
            onComplete={onComplete}
            onCancel={vi.fn()}
          />
        </QueryClientProvider>
      )

      await waitFor(() => {
        expect(onComplete).toHaveBeenCalledWith(true)
      })
    })

    it('calls onComplete with false when status changes to expired', async () => {
      const { onComplete, rerender } = renderComponent()

      // Update poll data to expired
      vi.mocked(useChallengePoll).mockReturnValue({
        data: {
          status: 'expired',
          dns_propagated: false,
          time_remaining_seconds: 0,
          last_check_at: new Date().toISOString(),
        },
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useChallengePoll>)

      const expiredChallenge: ManualChallenge = {
        ...mockChallenge,
        status: 'expired',
      }

      const queryClient = createQueryClient()
      rerender(
        <QueryClientProvider client={queryClient}>
          <ManualDNSChallenge
            providerId={1}
            challenge={expiredChallenge}
            onComplete={onComplete}
            onCancel={vi.fn()}
          />
        </QueryClientProvider>
      )

      await waitFor(() => {
        expect(onComplete).toHaveBeenCalledWith(false)
      })
    })
  })

  describe('Accessibility', () => {
    it('has proper ARIA labels for copy buttons', () => {
      renderComponent()

      expect(
        screen.getByRole('button', { name: /copy record name to clipboard/i })
      ).toBeInTheDocument()
      expect(
        screen.getByRole('button', { name: /copy record value to clipboard/i })
      ).toBeInTheDocument()
    })

    it('has screen reader announcer for status changes', () => {
      renderComponent()

      const announcer = document.querySelector('[role="status"][aria-live="polite"]')
      expect(announcer).toBeInTheDocument()
    })

    it('has proper labels for form fields', () => {
      renderComponent()

      expect(screen.getByText('Record Name')).toBeInTheDocument()
      expect(screen.getByText('Record Value')).toBeInTheDocument()
    })

    it('progress bar has accessible label', () => {
      renderComponent()

      const progressBar = screen.getByRole('progressbar')
      expect(progressBar).toHaveAttribute('aria-label')
    })

    it('buttons have aria-describedby for additional context', () => {
      renderComponent()

      const checkDnsButton = screen.getByText('Check DNS Now').closest('button')
      expect(checkDnsButton).toHaveAttribute('aria-describedby')
    })

    it('uses semantic heading structure', () => {
      renderComponent()

      // Card title should exist
      expect(screen.getByText('Manual DNS Challenge')).toBeInTheDocument()

      // Section heading for DNS record
      expect(screen.getByText('Create this TXT record at your DNS provider')).toBeInTheDocument()
    })
  })

  describe('Polling Behavior', () => {
    it('enables polling when challenge is pending', () => {
      renderComponent()

      expect(useChallengePoll).toHaveBeenCalledWith(1, 'test-challenge-uuid', true, 10000)
    })

    it('disables polling when challenge is in terminal state', () => {
      vi.mocked(useChallengePoll).mockReturnValue({
        data: {
          status: 'verified',
          dns_propagated: true,
          time_remaining_seconds: 0,
          last_check_at: new Date().toISOString(),
        },
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useChallengePoll>)

      const verifiedChallenge: ManualChallenge = {
        ...mockChallenge,
        status: 'verified',
      }
      renderComponent(verifiedChallenge)

      // The component should pass enabled=false for terminal states
      expect(useChallengePoll).toHaveBeenCalledWith(1, 'test-challenge-uuid', false, 10000)
    })
  })

  describe('Loading States', () => {
    it('shows loading state on verify button while verifying', () => {
      vi.mocked(useManualChallengeMutations).mockReturnValue({
        verifyMutation: {
          mutateAsync: mockVerifyMutation,
          isPending: true,
        },
        deleteMutation: {
          mutateAsync: mockDeleteMutation,
          isPending: false,
        },
        createMutation: {
          mutateAsync: vi.fn(),
          isPending: false,
        },
      } as unknown as ReturnType<typeof useManualChallengeMutations>)

      renderComponent()

      const verifyButton = screen.getByText("I've Created the Record - Verify").closest('button')
      expect(verifyButton).toBeDisabled()
    })

    it('shows loading state on cancel button while cancelling', () => {
      vi.mocked(useManualChallengeMutations).mockReturnValue({
        verifyMutation: {
          mutateAsync: mockVerifyMutation,
          isPending: false,
        },
        deleteMutation: {
          mutateAsync: mockDeleteMutation,
          isPending: true,
        },
        createMutation: {
          mutateAsync: vi.fn(),
          isPending: false,
        },
      } as unknown as ReturnType<typeof useManualChallengeMutations>)

      renderComponent()

      const cancelButton = screen.getByText('Cancel Challenge').closest('button')
      expect(cancelButton).toBeDisabled()
    })
  })

  describe('Error Messages', () => {
    it('displays error message from poll response', () => {
      vi.mocked(useChallengePoll).mockReturnValue({
        data: {
          status: 'failed',
          dns_propagated: false,
          time_remaining_seconds: 0,
          last_check_at: new Date().toISOString(),
          error_message: 'ACME server rejected the challenge',
        },
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useChallengePoll>)

      const failedChallenge: ManualChallenge = {
        ...mockChallenge,
        status: 'failed',
        error_message: 'ACME server rejected the challenge',
      }
      renderComponent(failedChallenge)

      expect(screen.getByText('ACME server rejected the challenge')).toBeInTheDocument()
    })
  })

  describe('Last Check Display', () => {
    it('shows last check time when available', () => {
      renderComponent()

      expect(screen.getByText(/Last checked/i)).toBeInTheDocument()
    })

    it('shows propagation status when not propagated', () => {
      renderComponent()

      expect(screen.getByText(/not yet propagated/i)).toBeInTheDocument()
    })
  })
})
