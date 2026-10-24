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

/** True when the server refused a CrowdSec change because a background preset install is running. */
export function isCrowdsecBusyError(err: unknown): boolean {
  return getApiErrorCode(err, ['crowdsec_busy'] as const) !== undefined
}

/**
 * User-facing text for a refused CrowdSec change: the busy message while a background
 * preset install runs, otherwise the caller's own fallback.
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @param fallback - Text to show for every other error
 */
export function crowdsecErrorMessage(err: unknown, t: TFunction, fallback: string): string {
  return isCrowdsecBusyError(err) ? t('crowdsecConfig.busy') : fallback
}
