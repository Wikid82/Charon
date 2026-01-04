import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DNSDetectionResult } from '../DNSDetectionResult'
import type { DetectionResult } from '../../api/dnsDetection'
import type { DNSProvider } from '../../api/dnsProviders'

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const translations: Record<string, string> = {
        'dns_detection.detecting': 'Detecting DNS provider...',
        'dns_detection.detected': `${params?.provider} detected`,
        'dns_detection.confidence_high': 'High confidence',
        'dns_detection.confidence_medium': 'Medium confidence',
        'dns_detection.confidence_low': 'Low confidence',
        'dns_detection.confidence_none': 'No match',
        'dns_detection.not_detected': 'Could not detect DNS provider',
        'dns_detection.use_suggested': `Use ${params?.provider}`,
        'dns_detection.select_manually': 'Select manually',
        'dns_detection.nameservers': 'Nameservers',
        'dns_detection.error': `Detection failed: ${params?.error}`,
      }
      return translations[key] || key
    },
  }),
}))

describe('DNSDetectionResult', () => {
  const mockSuggestedProvider: DNSProvider = {
    id: 1,
    uuid: 'test-uuid',
    name: 'Production Cloudflare',
    provider_type: 'cloudflare',
    enabled: true,
    is_default: true,
    has_credentials: true,
    propagation_timeout: 120,
    polling_interval: 5,
    success_count: 10,
    failure_count: 0,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  }

  it('should show loading state', () => {
    render(
      <DNSDetectionResult
        result={{} as DetectionResult}
        isLoading={true}
      />
    )

    expect(screen.getByText('Detecting DNS provider...')).toBeInTheDocument()
  })

  it('should show error message', () => {
    const result: DetectionResult = {
      domain: 'example.com',
      detected: false,
      nameservers: [],
      confidence: 'none',
      error: 'Network error',
    }

    render(<DNSDetectionResult result={result} />)

    expect(screen.getByText(/Detection failed: Network error/)).toBeInTheDocument()
  })

  it('should show not detected message with nameservers', () => {
    const result: DetectionResult = {
      domain: 'example.com',
      detected: false,
      nameservers: ['ns1.unknown.com', 'ns2.unknown.com'],
      confidence: 'none',
    }

    render(<DNSDetectionResult result={result} />)

    expect(screen.getByText('Could not detect DNS provider')).toBeInTheDocument()
    expect(screen.getByText(/nameservers/i)).toBeInTheDocument()
    expect(screen.getByText('ns1.unknown.com')).toBeInTheDocument()
    expect(screen.getByText('ns2.unknown.com')).toBeInTheDocument()
  })

  it('should show successful detection with high confidence', () => {
    const result: DetectionResult = {
      domain: 'example.com',
      detected: true,
      provider_type: 'cloudflare',
      nameservers: ['ns1.cloudflare.com', 'ns2.cloudflare.com'],
      confidence: 'high',
      suggested_provider: mockSuggestedProvider,
    }

    render(<DNSDetectionResult result={result} />)

    expect(screen.getByText('cloudflare detected')).toBeInTheDocument()
    expect(screen.getByText('High confidence')).toBeInTheDocument()
    expect(screen.getByText('Use Production Cloudflare')).toBeInTheDocument()
    expect(screen.getByText('Select manually')).toBeInTheDocument()
  })

  it('should call onUseSuggested when "Use" button is clicked', async () => {
    const user = userEvent.setup()
    const onUseSuggested = vi.fn()

    const result: DetectionResult = {
      domain: 'example.com',
      detected: true,
      provider_type: 'cloudflare',
      nameservers: ['ns1.cloudflare.com'],
      confidence: 'high',
      suggested_provider: mockSuggestedProvider,
    }

    render(
      <DNSDetectionResult
        result={result}
        onUseSuggested={onUseSuggested}
      />
    )

    await user.click(screen.getByText('Use Production Cloudflare'))

    expect(onUseSuggested).toHaveBeenCalledWith(mockSuggestedProvider)
  })

  it('should call onSelectManually when "Select manually" button is clicked', async () => {
    const user = userEvent.setup()
    const onSelectManually = vi.fn()

    const result: DetectionResult = {
      domain: 'example.com',
      detected: true,
      provider_type: 'cloudflare',
      nameservers: ['ns1.cloudflare.com'],
      confidence: 'high',
      suggested_provider: mockSuggestedProvider,
    }

    render(
      <DNSDetectionResult
        result={result}
        onSelectManually={onSelectManually}
      />
    )

    await user.click(screen.getByText('Select manually'))

    expect(onSelectManually).toHaveBeenCalled()
  })

  it('should show medium confidence badge', () => {
    const result: DetectionResult = {
      domain: 'example.com',
      detected: true,
      provider_type: 'route53',
      nameservers: ['ns-123.awsdns-12.com'],
      confidence: 'medium',
    }

    render(<DNSDetectionResult result={result} />)

    expect(screen.getByText('Medium confidence')).toBeInTheDocument()
  })

  it('should show low confidence badge', () => {
    const result: DetectionResult = {
      domain: 'example.com',
      detected: true,
      provider_type: 'digitalocean',
      nameservers: ['ns1.digitalocean.com'],
      confidence: 'low',
    }

    render(<DNSDetectionResult result={result} />)

    expect(screen.getByText('Low confidence')).toBeInTheDocument()
  })

  it('should show expandable nameservers list', async () => {
    const user = userEvent.setup()

    const result: DetectionResult = {
      domain: 'example.com',
      detected: true,
      provider_type: 'cloudflare',
      nameservers: ['ns1.cloudflare.com', 'ns2.cloudflare.com', 'ns3.cloudflare.com'],
      confidence: 'high',
      suggested_provider: mockSuggestedProvider,
    }

    render(<DNSDetectionResult result={result} />)

    // Nameservers are in a details element
    const summary = screen.getByText(/Nameservers \(3\)/)
    await user.click(summary)

    expect(screen.getByText('ns1.cloudflare.com')).toBeInTheDocument()
    expect(screen.getByText('ns2.cloudflare.com')).toBeInTheDocument()
    expect(screen.getByText('ns3.cloudflare.com')).toBeInTheDocument()
  })

  it('should not show action buttons when no suggested provider', () => {
    const result: DetectionResult = {
      domain: 'example.com',
      detected: true,
      provider_type: 'cloudflare',
      nameservers: ['ns1.cloudflare.com'],
      confidence: 'high',
    }

    render(<DNSDetectionResult result={result} />)

    expect(screen.queryByText(/Use/)).not.toBeInTheDocument()
    expect(screen.queryByText('Select manually')).not.toBeInTheDocument()
  })
})
