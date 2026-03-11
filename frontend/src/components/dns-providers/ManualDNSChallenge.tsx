import {
  Copy,
  Check,
  Clock,
  RefreshCw,
  CheckCircle2,
  XCircle,
  AlertCircle,
  Loader2,
  Info,
} from 'lucide-react'
import { useState, useEffect, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { useChallengePoll, useManualChallengeMutations } from '../../hooks/useManualChallenge'
import { toast } from '../../utils/toast'
import { Button, Card, CardHeader, CardContent, Progress, Alert } from '../ui'

import type { ManualChallenge, ChallengeStatus } from '../../api/manualChallenge'


interface ManualDNSChallengeProps {
  /** The DNS provider ID */
  providerId: number
  /** Initial challenge data */
  challenge: ManualChallenge
  /** Callback when challenge is completed or cancelled */
  onComplete: (success: boolean) => void
  /** Callback when challenge is cancelled */
  onCancel: () => void
}

/** Maps challenge status to visual properties */
const STATUS_CONFIG: Record<
  ChallengeStatus,
  {
    icon: typeof CheckCircle2
    colorClass: string
    labelKey: string
  }
> = {
  created: {
    icon: Clock,
    colorClass: 'text-content-muted',
    labelKey: 'dnsProvider.manual.status.created',
  },
  pending: {
    icon: Clock,
    colorClass: 'text-yellow-500',
    labelKey: 'dnsProvider.manual.status.pending',
  },
  verifying: {
    icon: Loader2,
    colorClass: 'text-brand-500',
    labelKey: 'dnsProvider.manual.status.verifying',
  },
  verified: {
    icon: CheckCircle2,
    colorClass: 'text-success',
    labelKey: 'dnsProvider.manual.status.verified',
  },
  expired: {
    icon: XCircle,
    colorClass: 'text-error',
    labelKey: 'dnsProvider.manual.status.expired',
  },
  failed: {
    icon: AlertCircle,
    colorClass: 'text-error',
    labelKey: 'dnsProvider.manual.status.failed',
  },
}

/** Terminal states where polling should stop */
const TERMINAL_STATES: ChallengeStatus[] = ['verified', 'expired', 'failed']

/**
 * Formats seconds into MM:SS display format
 */
function formatTimeRemaining(seconds: number): string {
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

/**
 * Calculates progress percentage based on time elapsed
 */
function calculateProgress(expiresAt: string, createdAt: string): number {
  const now = Date.now()
  const created = new Date(createdAt).getTime()
  const expires = new Date(expiresAt).getTime()
  const total = expires - created
  const elapsed = now - created
  const remaining = Math.max(0, 100 - (elapsed / total) * 100)
  return Math.round(remaining)
}

/**
 * Calculate time remaining in seconds
 */
function getTimeRemainingSeconds(expiresAt: string): number {
  const now = Date.now()
  const expires = new Date(expiresAt).getTime()
  return Math.max(0, Math.floor((expires - now) / 1000))
}

export default function ManualDNSChallenge({
  providerId,
  challenge,
  onComplete,
  onCancel,
}: ManualDNSChallengeProps) {
  const { t } = useTranslation()
  const [copiedField, setCopiedField] = useState<'name' | 'value' | null>(null)
  const [timeRemaining, setTimeRemaining] = useState(() =>
    getTimeRemainingSeconds(challenge.expires_at)
  )
  const [progress, setProgress] = useState(() =>
    calculateProgress(challenge.expires_at, challenge.created_at)
  )
  const statusAnnouncerRef = useRef<HTMLDivElement>(null)
  const previousStatusRef = useRef<ChallengeStatus>(challenge.status)

  // Determine if challenge is in a terminal state
  const isTerminal = TERMINAL_STATES.includes(challenge.status)

  // Poll for status updates (every 10 seconds when not terminal)
  const { data: pollData } = useChallengePoll(
    providerId,
    challenge.id,
    !isTerminal,
    10000
  )

  const { verifyMutation, deleteMutation } = useManualChallengeMutations()

  // Current status from poll data or initial challenge
  const currentStatus: ChallengeStatus = pollData?.status || challenge.status
  const dnsPropagated = pollData?.dns_propagated ?? challenge.dns_propagated
  const lastCheckAt = pollData?.last_check_at ?? challenge.last_check_at

  // Update countdown timer
  useEffect(() => {
    if (isTerminal) return

    const interval = setInterval(() => {
      const remaining = getTimeRemainingSeconds(challenge.expires_at)
      setTimeRemaining(remaining)
      setProgress(calculateProgress(challenge.expires_at, challenge.created_at))

      // Auto-expire if time runs out
      if (remaining <= 0) {
        clearInterval(interval)
      }
    }, 1000)

    return () => clearInterval(interval)
  }, [challenge.expires_at, challenge.created_at, isTerminal])

  // Announce status changes to screen readers
  useEffect(() => {
    if (currentStatus !== previousStatusRef.current) {
      previousStatusRef.current = currentStatus
      const statusLabel = t(STATUS_CONFIG[currentStatus].labelKey)

      // Announce the status change for screen readers
      if (statusAnnouncerRef.current) {
        statusAnnouncerRef.current.textContent = t('dnsProvider.manual.statusChanged', {
          status: statusLabel,
        })
      }

      // Show toast for terminal states
      if (currentStatus === 'verified') {
        toast.success(t('dnsProvider.manual.verifySuccess'))
        onComplete(true)
      } else if (currentStatus === 'expired') {
        toast.error(t('dnsProvider.manual.challengeExpired'))
        onComplete(false)
      } else if (currentStatus === 'failed') {
        toast.error(pollData?.error_message || t('dnsProvider.manual.verifyFailed'))
        onComplete(false)
      }
    }
  }, [currentStatus, pollData?.error_message, onComplete, t])

  // Copy to clipboard handler
  const handleCopy = useCallback(
    async (field: 'name' | 'value', text: string) => {
      try {
        await navigator.clipboard.writeText(text)
        setCopiedField(field)
        toast.success(t('dnsProvider.manual.copied'))

        // Reset copied state after 2 seconds
        setTimeout(() => setCopiedField(null), 2000)
      } catch {
        toast.error(t('dnsProvider.manual.copyFailed'))
      }
    },
    [t]
  )

  // Verify challenge handler
  const handleVerify = useCallback(async () => {
    try {
      const result = await verifyMutation.mutateAsync({
        providerId,
        challengeId: challenge.id,
      })

      if (result.success) {
        toast.success(t('dnsProvider.manual.verifySuccess'))
      } else if (!result.dns_found) {
        toast.warning(t('dnsProvider.manual.dnsNotFound'))
      }
    } catch (error: unknown) {
      const err = error as { response?: { data?: { message?: string } } }
      toast.error(err.response?.data?.message || t('dnsProvider.manual.verifyFailed'))
    }
  }, [verifyMutation, providerId, challenge.id, t])

  // Cancel challenge handler
  const handleCancel = useCallback(async () => {
    try {
      await deleteMutation.mutateAsync({
        providerId,
        challengeId: challenge.id,
      })
      toast.info(t('dnsProvider.manual.challengeCancelled'))
      onCancel()
    } catch (error: unknown) {
      const err = error as { response?: { data?: { message?: string } } }
      toast.error(err.response?.data?.message || t('dnsProvider.manual.cancelFailed'))
    }
  }, [deleteMutation, providerId, challenge.id, onCancel, t])

  // Get status display properties
  const statusConfig = STATUS_CONFIG[currentStatus]
  const StatusIcon = statusConfig.icon

  // Format last check time
  const getLastCheckText = (): string => {
    if (!lastCheckAt) return ''
    const seconds = Math.floor((Date.now() - new Date(lastCheckAt).getTime()) / 1000)
    if (seconds < 60) return t('dnsProvider.manual.lastCheckSecondsAgo', { seconds })
    const minutes = Math.floor(seconds / 60)
    return t('dnsProvider.manual.lastCheckMinutesAgo', { minutes })
  }

  return (
    <Card className="w-full max-w-2xl">
      {/* Screen reader announcer for status changes */}
      <div
        ref={statusAnnouncerRef}
        role="status"
        aria-live="polite"
        aria-atomic="true"
        className="sr-only"
      />

      <CardHeader>
        <h2 className="text-lg font-semibold leading-tight text-content-primary flex items-center gap-2">
          <span aria-hidden="true">🔐</span>
          {t('dnsProvider.manual.title')}
        </h2>
      </CardHeader>

      <CardContent className="space-y-6">
        {/* Instructions */}
        <p className="text-content-secondary">
          {t('dnsProvider.manual.instructions', { domain: challenge.fqdn.replace('_acme-challenge.', '') })}
        </p>

        {/* DNS Record Details */}
        <div
          className="border border-border rounded-lg overflow-hidden"
          role="region"
          aria-labelledby="dns-record-heading"
        >
          <div className="bg-surface-subtle px-4 py-2 border-b border-border">
            <h3 id="dns-record-heading" className="text-sm font-medium flex items-center gap-2">
              <span aria-hidden="true">📋</span>
              {t('dnsProvider.manual.createRecord')}
            </h3>
          </div>

          {/* Record Name */}
          <div className="px-4 py-3 border-b border-border">
            <div className="flex items-center justify-between gap-4">
              <div className="flex-1 min-w-0">
                <label
                  htmlFor="record-name"
                  className="block text-xs font-medium text-content-muted mb-1"
                >
                  {t('dnsProvider.manual.recordName')}
                </label>
                <code
                  id="record-name"
                  className="block text-sm font-mono text-content-primary truncate"
                  title={challenge.fqdn}
                >
                  {challenge.fqdn}
                </code>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => handleCopy('name', challenge.fqdn)}
                aria-label={t('dnsProvider.manual.copyRecordName')}
                className="shrink-0"
              >
                {copiedField === 'name' ? (
                  <Check className="h-4 w-4 text-success" aria-hidden="true" />
                ) : (
                  <Copy className="h-4 w-4" aria-hidden="true" />
                )}
                <span className="sr-only">
                  {copiedField === 'name' ? t('dnsProvider.manual.copied') : t('dnsProvider.manual.copy')}
                </span>
              </Button>
            </div>
          </div>

          {/* Record Value */}
          <div className="px-4 py-3 border-b border-border">
            <div className="flex items-center justify-between gap-4">
              <div className="flex-1 min-w-0">
                <label
                  htmlFor="record-value"
                  className="block text-xs font-medium text-content-muted mb-1"
                >
                  {t('dnsProvider.manual.recordValue')}
                </label>
                <code
                  id="record-value"
                  className="block text-sm font-mono text-content-primary truncate"
                  title={challenge.value}
                >
                  {challenge.value}
                </code>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => handleCopy('value', challenge.value)}
                aria-label={t('dnsProvider.manual.copyRecordValue')}
                className="shrink-0"
              >
                {copiedField === 'value' ? (
                  <Check className="h-4 w-4 text-success" aria-hidden="true" />
                ) : (
                  <Copy className="h-4 w-4" aria-hidden="true" />
                )}
                <span className="sr-only">
                  {copiedField === 'value' ? t('dnsProvider.manual.copied') : t('dnsProvider.manual.copy')}
                </span>
              </Button>
            </div>
          </div>

          {/* TTL */}
          <div className="px-4 py-3 bg-surface-subtle/50">
            <span className="text-sm text-content-muted">
              {t('dnsProvider.manual.ttl')}: {challenge.ttl} {t('dnsProvider.manual.seconds')} ({Math.floor(challenge.ttl / 60)} {t('dnsProvider.manual.minutes')})
            </span>
          </div>
        </div>

        {/* Progress Section */}
        {!isTerminal && (
          <div className="space-y-2" role="region" aria-labelledby="time-remaining-heading">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Clock className="h-4 w-4 text-content-muted" aria-hidden="true" />
                <span id="time-remaining-heading" className="text-sm font-medium">
                  {t('dnsProvider.manual.timeRemaining')}: {formatTimeRemaining(timeRemaining)}
                </span>
              </div>
              <span
                className="text-sm text-content-muted tabular-nums"
                aria-label={t('dnsProvider.manual.progressPercent', { percent: progress })}
              >
                {progress}%
              </span>
            </div>
            <Progress
              value={progress}
              variant={progress < 25 ? 'error' : progress < 50 ? 'warning' : 'default'}
              aria-label={t('dnsProvider.manual.challengeProgress')}
            />
          </div>
        )}

        {/* Action Buttons */}
        <div className="flex flex-wrap gap-3">
          <Button
            variant="secondary"
            onClick={handleVerify}
            disabled={isTerminal || verifyMutation.isPending}
            isLoading={verifyMutation.isPending}
            leftIcon={RefreshCw}
            aria-describedby="check-dns-description"
          >
            {t('dnsProvider.manual.checkDnsNow')}
          </Button>
          <span id="check-dns-description" className="sr-only">
            {t('dnsProvider.manual.checkDnsDescription')}
          </span>

          <Button
            variant="primary"
            onClick={handleVerify}
            disabled={isTerminal || verifyMutation.isPending}
            isLoading={verifyMutation.isPending}
            leftIcon={CheckCircle2}
            aria-describedby="verify-description"
          >
            {t('dnsProvider.manual.verifyButton')}
          </Button>
          <span id="verify-description" className="sr-only">
            {t('dnsProvider.manual.verifyDescription')}
          </span>
        </div>

        {/* Status Indicator */}
        <Alert
          variant={
            currentStatus === 'verified'
              ? 'success'
              : currentStatus === 'failed' || currentStatus === 'expired'
              ? 'error'
              : 'info'
          }
        >
          <div className="flex items-start gap-3">
            <StatusIcon
              className={`h-5 w-5 shrink-0 ${statusConfig.colorClass} ${
                currentStatus === 'verifying' ? 'animate-spin' : ''
              }`}
              aria-hidden="true"
            />
            <div className="flex-1 min-w-0">
              <p className="font-medium">
                {t(`dnsProvider.manual.statusMessage.${currentStatus}`, {
                  defaultValue: t(statusConfig.labelKey),
                })}
              </p>
              {lastCheckAt && !isTerminal && (
                <p className="text-sm text-content-muted mt-1">
                  {t('dnsProvider.manual.lastCheck')}: {getLastCheckText()}
                </p>
              )}
              {!dnsPropagated && !isTerminal && (
                <p className="text-sm text-content-muted mt-1 flex items-center gap-1">
                  <Info className="h-3 w-3" aria-hidden="true" />
                  {t('dnsProvider.manual.notPropagated')}
                </p>
              )}
              {pollData?.error_message && (
                <p className="text-sm text-error mt-1">{pollData.error_message}</p>
              )}
            </div>
          </div>
        </Alert>

        {/* Cancel Button */}
        {!isTerminal && (
          <div className="flex justify-end pt-2 border-t border-border">
            <Button
              variant="ghost"
              onClick={handleCancel}
              disabled={deleteMutation.isPending}
              isLoading={deleteMutation.isPending}
            >
              {t('dnsProvider.manual.cancelChallenge')}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
