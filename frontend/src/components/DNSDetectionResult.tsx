import { CheckCircle2, AlertCircle, Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge, Button, Alert } from './ui'

import type { DetectionResult } from '../api/dnsDetection'
import type { DNSProvider } from '../api/dnsProviders'

interface DNSDetectionResultProps {
  result: DetectionResult
  onUseSuggested?: (provider: DNSProvider) => void
  onSelectManually?: () => void
  isLoading?: boolean
}

export function DNSDetectionResult({
  result,
  onUseSuggested,
  onSelectManually,
  isLoading = false,
}: DNSDetectionResultProps) {
  const { t } = useTranslation()

  if (isLoading) {
    return (
      <Alert variant="info">
        <Info className="h-4 w-4" />
        <div className="ml-2">
          <p className="text-sm font-medium">{t('dns_detection.detecting')}</p>
        </div>
      </Alert>
    )
  }

  if (result.error) {
    return (
      <Alert variant="warning">
        <AlertCircle className="h-4 w-4" />
        <div className="ml-2">
          <p className="text-sm font-medium">{t('dns_detection.error', { error: result.error })}</p>
        </div>
      </Alert>
    )
  }

  if (!result.detected) {
    return (
      <Alert variant="info">
        <Info className="h-4 w-4" />
        <div className="ml-2">
          <p className="text-sm font-medium">{t('dns_detection.not_detected')}</p>
          {result.nameservers.length > 0 && (
            <div className="mt-2">
              <p className="text-xs text-content-secondary">{t('dns_detection.nameservers')}:</p>
              <ul className="text-xs text-content-secondary mt-1 space-y-0.5">
                {result.nameservers.map((ns, i) => (
                  <li key={i} className="font-mono">
                    {ns}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </Alert>
    )
  }

  const getConfidenceBadgeVariant = (confidence: string) => {
    switch (confidence) {
      case 'high':
        return 'success'
      case 'medium':
        return 'warning'
      case 'low':
        return 'outline'
      default:
        return 'outline'
    }
  }

  const getConfidenceLabel = (confidence: string) => {
    return t(`dns_detection.confidence_${confidence}`)
  }

  return (
    <Alert variant="success" className="border-brand-500/30 bg-brand-500/5">
      <CheckCircle2 className="h-4 w-4 text-brand-500" />
      <div className="ml-2 flex-1">
        <div className="flex items-center gap-2 flex-wrap">
          <p className="text-sm font-medium">
            {t('dns_detection.detected', { provider: result.provider_type })}
          </p>
          <Badge variant={getConfidenceBadgeVariant(result.confidence)} size="sm">
            {getConfidenceLabel(result.confidence)}
          </Badge>
        </div>

        {result.suggested_provider && (
          <div className="mt-3 flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="primary"
              onClick={() => onUseSuggested?.(result.suggested_provider!)}
            >
              {t('dns_detection.use_suggested', { provider: result.suggested_provider.name })}
            </Button>
            <Button size="sm" variant="outline" onClick={onSelectManually}>
              {t('dns_detection.select_manually')}
            </Button>
          </div>
        )}

        {result.nameservers.length > 0 && (
          <details className="mt-3">
            <summary className="text-xs text-content-secondary cursor-pointer hover:text-content-primary">
              {t('dns_detection.nameservers')} ({result.nameservers.length})
            </summary>
            <ul className="text-xs text-content-secondary mt-2 space-y-0.5 ml-4">
              {result.nameservers.map((ns, i) => (
                <li key={i} className="font-mono">
                  {ns}
                </li>
              ))}
            </ul>
          </details>
        )}
      </div>
    </Alert>
  )
}
