import { useTranslation } from 'react-i18next'

import { isApplyJobTerminal } from '../../utils/crowdsecApplyJob'
import { Button } from '../ui/Button'

import type { CrowdsecApplyJob } from '../../api/presets'
import type { ReactNode } from 'react'

interface PresetInstallProgressProps {
  job: CrowdsecApplyJob
  /** A status poll failed; the numbers shown are the last ones the server reported. */
  isWaitingForServer: boolean
  isCancelling: boolean
  /** The last cancel request failed; shown as plain text next to the button. */
  cancelFailed?: boolean
  onCancel: () => void
  /** Translated reason for a failed install; null falls back to a generic text. */
  failureText: string | null
}

/**
 * Passive status of a background hub preset install: an inline progress row while the job runs
 * and a plain result text once it ended. No spinner overlay, banner or alert.
 *
 * One live region stays mounted for the whole job. Only the phase and the result text sit in the
 * announced part; the ticking item count and elapsed time are aria-hidden so polls do not re-announce.
 */
export function PresetInstallProgress({ job, isWaitingForServer, isCancelling, cancelFailed = false, onCancel, failureText }: Readonly<PresetInstallProgressProps>) {
  const { t } = useTranslation()
  const prefix = 'crowdsecConfig.presets.installProgress'
  const running = !isApplyJobTerminal(job)

  const formatElapsed = (totalSeconds: number): string => {
    const seconds = Math.max(0, Math.floor(totalSeconds))
    return seconds < 60
      ? t(`${prefix}.elapsedSeconds`, { seconds })
      : t(`${prefix}.elapsed`, { minutes: Math.floor(seconds / 60), seconds: seconds % 60 })
  }

  let liveContent: ReactNode
  if (running) {
    const phaseText = job.state === 'queued' ? t(`${prefix}.starting`) : t(`${prefix}.phase.${job.phase}`)
    const ticking = [
      job.items_installed > 0 ? t(`${prefix}.itemsSoFar`, { count: job.items_installed }) : null,
      formatElapsed(job.elapsed_seconds),
    ].filter((part): part is string => part !== null)
    liveContent = (
      <>
        <span>{phaseText}</span>
        <span aria-hidden="true">{`, ${ticking.join(', ')}`}</span>
        {isWaitingForServer && <span>{`, ${t(`${prefix}.waitingForServer`)}`}</span>}
      </>
    )
  } else {
    liveContent = <ResultText job={job} failureText={failureText} />
  }

  const cancelling = job.cancel_requested || isCancelling

  return (
    <div
      className={running ? 'flex flex-wrap items-center gap-3 text-sm text-gray-200' : 'space-y-1 text-sm text-gray-200'}
      data-testid={running ? 'preset-install-progress' : 'preset-install-result'}
    >
      <div role="status" aria-label={t(`${prefix}.label`)} className={running ? undefined : 'space-y-1'}>
        {liveContent}
      </div>
      {running && (
        <>
          <Button size="sm" variant="secondary" onClick={onCancel} disabled={cancelling}>
            {cancelling ? t(`${prefix}.cancelling`) : t(`${prefix}.cancel`)}
          </Button>
          {cancelFailed && <span className="text-yellow-300">{t(`${prefix}.cancelFailed`)}</span>}
        </>
      )}
    </div>
  )
}

function ResultText({ job, failureText }: Readonly<{ job: CrowdsecApplyJob; failureText: string | null }>) {
  const { t } = useTranslation()
  const prefix = 'crowdsecConfig.presets.installProgress'
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
    <>
      <p>{headline}</p>
      {restored && <p className="text-gray-400">{t(`${prefix}.rollbackNote`)}</p>}
      {job.state === 'succeeded' && job.warnings.length > 0 && (
        <div className="text-yellow-300" data-testid="preset-apply-warnings">
          <p>{t('crowdsecConfig.presets.status.applyDependencyNote')}</p>
          <ul className="list-disc pl-4">
            {job.warnings.map((warning, index) => (
              <li key={`${index}-${warning}`}>{warning}</li>
            ))}
          </ul>
        </div>
      )}
    </>
  )
}
