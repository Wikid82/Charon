import { useTranslation } from 'react-i18next'

import { isApplyJobTerminal } from '../../utils/crowdsecApplyJob'
import { Button } from '../ui/Button'

import type { CrowdsecApplyJob } from '../../api/presets'

interface PresetInstallProgressProps {
  job: CrowdsecApplyJob
  /** A status poll failed; the numbers shown are the last ones the server reported. */
  isWaitingForServer: boolean
  isCancelling: boolean
  onCancel: () => void
  /** Translated reason for a failed install; null falls back to a generic text. */
  failureText: string | null
}

/**
 * Passive status of a background hub preset install: an inline progress row while the job runs
 * and a plain result text once it ended. No spinner overlay, banner or alert.
 */
export function PresetInstallProgress({ job, isWaitingForServer, isCancelling, onCancel, failureText }: Readonly<PresetInstallProgressProps>) {
  const { t } = useTranslation()
  const prefix = 'crowdsecConfig.presets.installProgress'

  const formatElapsed = (totalSeconds: number): string => {
    const seconds = Math.max(0, Math.floor(totalSeconds))
    return seconds < 60
      ? t(`${prefix}.elapsedSeconds`, { seconds })
      : t(`${prefix}.elapsed`, { minutes: Math.floor(seconds / 60), seconds: seconds % 60 })
  }

  if (!isApplyJobTerminal(job)) {
    const phaseText = job.state === 'queued' ? t(`${prefix}.starting`) : t(`${prefix}.phase.${job.phase}`)
    const parts = [
      phaseText,
      job.items_installed > 0 ? t(`${prefix}.itemsSoFar`, { count: job.items_installed }) : null,
      formatElapsed(job.elapsed_seconds),
      isWaitingForServer ? t(`${prefix}.waitingForServer`) : null,
    ].filter((part): part is string => part !== null)
    const cancelling = job.cancel_requested || isCancelling

    return (
      <div
        role="status"
        aria-label={t(`${prefix}.label`)}
        className="flex flex-wrap items-center gap-3 text-sm text-gray-200"
        data-testid="preset-install-progress"
      >
        <span>{parts.join(', ')}</span>
        <Button size="sm" variant="secondary" onClick={onCancel} disabled={cancelling}>
          {cancelling ? t(`${prefix}.cancelling`) : t(`${prefix}.cancel`)}
        </Button>
      </div>
    )
  }

  const interrupted = job.error_code === 'interrupted'
  let headline: string
  let restored = true
  if (job.state === 'succeeded') {
    restored = false
    headline = job.items_installed > 0
      ? `${t(`${prefix}.succeeded`)} ${t(`${prefix}.itemsInstalled`, { count: job.items_installed })}`
      : t(`${prefix}.succeeded`)
  } else if (job.state === 'cancelled') {
    headline = t(`${prefix}.cancelled`)
  } else if (job.state === 'timed_out') {
    headline = t(`${prefix}.timedOut`)
  } else if (interrupted) {
    headline = t(`${prefix}.interrupted`)
  } else {
    headline = `${t(`${prefix}.failed`)} ${failureText ?? t(`${prefix}.failedGeneric`)}`
  }

  return (
    <div className="space-y-1 text-sm text-gray-200" data-testid="preset-install-result">
      <p>{headline}</p>
      {restored && <p className="text-gray-400">{t(`${prefix}.rollbackNote`)}</p>}
      {job.state === 'succeeded' && job.warnings.length > 0 && (
        <div className="text-yellow-300" data-testid="preset-apply-warnings">
          <p>{t('crowdsecConfig.presets.status.applyDependencyNote')}</p>
          <ul className="list-disc pl-4">
            {job.warnings.map((warning) => (
              <li key={warning}>{warning}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
