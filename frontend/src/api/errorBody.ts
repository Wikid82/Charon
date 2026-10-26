import { isAxiosError } from 'axios'

import type { TFunction } from 'i18next'

/**
 * Reads the machine-readable `error_code` from a failed request.
 * @param err - The value thrown by the request
 * @param codes - The codes the caller knows how to handle
 * @returns The code, or undefined when it is missing or not one of `codes`
 */
export function getApiErrorCode<C extends string>(err: unknown, codes: readonly C[]): C | undefined {
  if (!isAxiosError<{ error_code?: string }>(err)) return undefined
  const code = err.response?.data?.error_code
  return codes.find((known) => known === code)
}

/** True when the server refused a CrowdSec change because another CrowdSec change is running. */
export function isCrowdsecBusyError(err: unknown): boolean {
  return getApiErrorCode(err, ['crowdsec_busy'] as const) !== undefined
}

/**
 * Reads the id of the background job behind a `crowdsec_busy` refusal.
 * @returns The job id, or undefined when the server named none
 */
export function getCrowdsecBusyJobId(err: unknown): string | undefined {
  if (!isAxiosError<{ job_id?: unknown }>(err)) return undefined
  const id = err.response?.data?.job_id
  return typeof id === 'string' && id !== '' ? id : undefined
}

/**
 * Translated text for a `crowdsec_busy` refusal. A refusal that names a job means a
 * background preset install runs; one without a job means another change holds CrowdSec.
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @returns The text, or null when the error is not a busy refusal
 */
export function crowdsecBusyMessage(err: unknown, t: TFunction): string | null {
  if (!isCrowdsecBusyError(err)) return null
  return getCrowdsecBusyJobId(err) ? t('crowdsecConfig.busy') : t('crowdsecConfig.busyOther')
}

/**
 * User-facing text for a refused CrowdSec change: the busy text for a busy refusal,
 * otherwise the caller's own fallback.
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @param fallback - Text to show for every other error
 */
export function crowdsecErrorMessage(err: unknown, t: TFunction, fallback: string): string {
  return crowdsecBusyMessage(err, t) ?? fallback
}
