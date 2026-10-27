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
 * Coded CrowdSec refusals with a translated text of their own, keyed by the server's `error_code`.
 * Every other code keeps the caller's fallback.
 */
const CROWDSEC_CODE_MESSAGE_KEYS = {
  file_managed_by_charon: 'crowdsecConfig.toasts.fileManaged',
  crowdsec_state_changed: 'crowdsec.errors.stateChanged',
  crowdsec_unavailable: 'crowdsec.errors.unavailable',
} as const

type CrowdsecCodedMessage = keyof typeof CROWDSEC_CODE_MESSAGE_KEYS

const CROWDSEC_CODED_MESSAGES = Object.keys(CROWDSEC_CODE_MESSAGE_KEYS) as CrowdsecCodedMessage[]

/**
 * User-facing text for a refused CrowdSec change: the busy text for a busy refusal, the
 * translated text for a coded refusal (managed file, state changed, unavailable), otherwise
 * the caller's own fallback.
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @param fallback - Text to show for every other error
 */
export function crowdsecErrorMessage(err: unknown, t: TFunction, fallback: string): string {
  const busy = crowdsecBusyMessage(err, t)
  if (busy !== null) return busy
  const code = getApiErrorCode(err, CROWDSEC_CODED_MESSAGES)
  return code ? t(CROWDSEC_CODE_MESSAGE_KEYS[code]) : fallback
}

/**
 * Coded WAF rule-set refusals with a translated text of their own, keyed by the server's `error_code`.
 * Every other code keeps the caller's fallback.
 */
const WAF_CODE_MESSAGE_KEYS = {
  waf_rules_source_unknown: 'wafConfig.errors.waf_rules_source_unknown',
  waf_ruleset_in_use: 'wafConfig.errors.waf_ruleset_in_use',
  invalid_waf_mode: 'wafConfig.errors.invalid_waf_mode',
} as const

type WafCodedMessage = keyof typeof WAF_CODE_MESSAGE_KEYS

const WAF_CODED_MESSAGES = Object.keys(WAF_CODE_MESSAGE_KEYS) as WafCodedMessage[]

/**
 * User-facing text for a refused WAF rule-set change: the translated text for a coded refusal
 * (unknown source, rule set in use, invalid mode), otherwise the caller's own fallback.
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @param fallback - Text to show for every other error
 */
export function wafErrorMessage(err: unknown, t: TFunction, fallback: string): string {
  const code = getApiErrorCode(err, WAF_CODED_MESSAGES)
  return code ? t(WAF_CODE_MESSAGE_KEYS[code]) : fallback
}
