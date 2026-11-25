import { isAxiosError } from 'axios'

import { rateLimitMessage } from '../utils/rateLimit'

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
  crowdsec_engine_exited: 'crowdsec.errors.engineExited',
  import_config_invalid: 'crowdsec.errors.importConfigInvalid',
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
  ruleset_empty: 'wafConfig.errors.ruleset_empty',
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

/**
 * Translated text for a refused external proxy listen setting (`external_proxy_bind_invalid`).
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @returns The text, or null when the refusal is for another reason
 */
export function externalProxyBindErrorMessage(err: unknown, t: TFunction): string | null {
  return getApiErrorCode(err, ['external_proxy_bind_invalid'] as const) ? t('hecate.externalProxy.bindInvalid') : null
}

/**
 * Coded access-list refusals (saves, guarded changes and the management choice) with a translated text of their
 * own, keyed by the server's `error_code`. Every other code keeps the generic error text.
 */
const ACCESS_LIST_CODE_MESSAGE_KEYS = {
  access_list_rules_required: 'accessLists.errors.access_list_rules_required',
  access_list_in_use: 'accessLists.errors.access_list_in_use',
  management_list_unknown: 'errors.managementListUnknown',
  management_access_would_lock_out: 'errors.managementAccessWouldLockOut',
  access_list_invalid: 'errors.accessListInvalid',
} as const

type AccessListCodedMessage = keyof typeof ACCESS_LIST_CODE_MESSAGE_KEYS

const ACCESS_LIST_CODED_MESSAGES = Object.keys(ACCESS_LIST_CODE_MESSAGE_KEYS) as AccessListCodedMessage[]

/**
 * Translated text for a coded access-list save refusal, for display inside the form.
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @returns The text, or null when the refusal has no text of its own
 */
export function accessListErrorMessage(err: unknown, t: TFunction): string | null {
  const code = getApiErrorCode(err, ACCESS_LIST_CODED_MESSAGES)
  return code ? t(ACCESS_LIST_CODE_MESSAGE_KEYS[code]) : null
}

/**
 * Translated text for a refused access-list delete. A coded refusal uses its own text. The server answers a delete
 * that the proxy or Charon's own interface still uses with HTTP 409 and its message; that message is shown as sent.
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @returns The text, or null when the refusal has no text the caller can show
 */
export function accessListDeleteErrorMessage(err: unknown, t: TFunction): string | null {
  const coded = accessListErrorMessage(err, t)
  if (coded !== null) return coded
  if (!isAxiosError<{ error?: unknown }>(err) || err.response?.status !== 409) return null
  const message = err.response.data?.error
  return typeof message === 'string' && message !== '' ? message : null
}

/**
 * Text for a refused remote-target save. A 400 response carries the reason in its `error` text (for example an http://
 * WebDAV address without the unencrypted option), so that text is shown; any other failure keeps its own message.
 * @param err - The error thrown by the save request
 * @returns The text to show
 */
export function remoteTargetSaveErrorMessage(err: Error): string {
  if (isAxiosError<{ error?: unknown }>(err) && err.response?.status === 400) {
    const message = err.response.data?.error
    if (typeof message === 'string' && message !== '') return message
  }
  return err.message
}

/**
 * Coded redirection-host save refusals with a translated text of their own, keyed by the server's `error_code`.
 * Every other code keeps the form's fallback text.
 */
const REDIRECTION_CODE_MESSAGE_KEYS = {
  redirect_target_invalid: 'redirectionHosts.errors.redirect_target_invalid',
} as const

type RedirectionCodedMessage = keyof typeof REDIRECTION_CODE_MESSAGE_KEYS

const REDIRECTION_CODED_MESSAGES = Object.keys(REDIRECTION_CODE_MESSAGE_KEYS) as RedirectionCodedMessage[]

/**
 * Translated text for a coded redirection-host save refusal, for display inside the form.
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @returns The text, or null when the refusal has no text of its own
 */
export function redirectionErrorMessage(err: unknown, t: TFunction): string | null {
  const code = getApiErrorCode(err, REDIRECTION_CODED_MESSAGES)
  return code ? t(REDIRECTION_CODE_MESSAGE_KEYS[code]) : null
}

/**
 * Error codes of the GeoIP database routes (reload, lookup, replace, remove) that have a translated text. Keep in step
 * with the backend's error_code values and the `security.geoip.errors.*` locale keys.
 */
export const GEOIP_ERROR_CODES = [
  'geoip_file_required',
  'geoip_too_large',
  'geoip_not_mmdb',
  'geoip_corrupt',
  'geoip_wrong_type',
  'geoip_older_than_current',
  'geoip_build_date_in_future',
  'geoip_custom_path_set',
  'geoip_busy',
  'geoip_write_failed',
  'geoip_reload_failed',
  'geoip_no_data_copy',
  'geoip_service_unavailable',
  'proxy_apply_failed',
] as const

/**
 * Locale key of the text for a GeoIP error code. An unknown or missing code uses the generic text.
 * @param code - The `error_code` of the refusal, if any
 * @returns The translation key under `security.geoip.errors`
 */
export function geoipErrorKey(code: string | undefined): string {
  const known = (GEOIP_ERROR_CODES as readonly string[]).includes(code ?? '')
  return known ? `security.geoip.errors.${code}` : 'security.geoip.errors.generic'
}

/**
 * Coded first-run setup refusals with a translated text of their own, keyed by the server's `error_code`.
 */
const SETUP_CODE_MESSAGE_KEYS = {
  setup_code_invalid: 'setup.errors.setup_code_invalid',
} as const

type SetupCodedMessage = keyof typeof SETUP_CODE_MESSAGE_KEYS

const SETUP_CODED_MESSAGES = Object.keys(SETUP_CODE_MESSAGE_KEYS) as SetupCodedMessage[]

/**
 * Translated text for a refused first-run setup: a fixed text for a wrong or missing setup code,
 * and a fixed text for a throttled attempt (HTTP 429). Other refusals return null so the caller decides.
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @returns The text, or null when the refusal has no text of its own
 */
export function setupErrorMessage(err: unknown, t: TFunction): string | null {
  if (isAxiosError(err) && err.response?.status === 429) return t('setup.errors.rate_limited')
  const code = getApiErrorCode(err, SETUP_CODED_MESSAGES)
  return code ? t(SETUP_CODE_MESSAGE_KEYS[code]) : null
}

/**
 * Coded trusted-proxy refusals with a translated text of their own, keyed by the server's `error_code`. Each text may
 * name the refused entry (`{{entry}}`).
 */
const TRUSTED_PROXY_CODE_MESSAGE_KEYS = {
  trusted_proxy_invalid: 'errors.trustedProxyInvalid',
  trusted_proxy_too_broad: 'errors.trustedProxyTooBroad',
  trusted_proxies_too_many: 'errors.trustedProxiesTooMany',
} as const

type TrustedProxyCodedMessage = keyof typeof TRUSTED_PROXY_CODE_MESSAGE_KEYS

const TRUSTED_PROXY_CODED_MESSAGES = Object.keys(TRUSTED_PROXY_CODE_MESSAGE_KEYS) as TrustedProxyCodedMessage[]

/**
 * Translated text for a refused trusted-proxy entry or list, naming the entry the server refused when it sent one.
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @returns The text, or null when the refusal has no text of its own
 */
export function trustedProxyErrorMessage(err: unknown, t: TFunction): string | null {
  const code = getApiErrorCode(err, TRUSTED_PROXY_CODED_MESSAGES)
  if (!code) return null
  const entry = isAxiosError<{ entry?: unknown }>(err) && typeof err.response?.data?.entry === 'string' ? err.response.data.entry : ''
  return t(TRUSTED_PROXY_CODE_MESSAGE_KEYS[code], { entry })
}

/**
 * Coded permission refusals with a translated text of their own, keyed by the server's `error_code`.
 */
const PERMISSIONS_CODE_MESSAGE_KEYS = {
  permissions_admin_only: 'permissions.adminOnlyError',
} as const

type PermissionsCodedMessage = keyof typeof PERMISSIONS_CODE_MESSAGE_KEYS

const PERMISSIONS_CODED_MESSAGES = Object.keys(PERMISSIONS_CODE_MESSAGE_KEYS) as PermissionsCodedMessage[]

/**
 * Translated text for a refusal because the caller is not an administrator.
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @returns The text, or null when the refusal is not a permission refusal
 */
export function permissionsErrorMessage(err: unknown, t: TFunction): string | null {
  const code = getApiErrorCode(err, PERMISSIONS_CODED_MESSAGES)
  return code ? t(PERMISSIONS_CODE_MESSAGE_KEYS[code]) : null
}

/**
 * Translated texts for refused destinations and failed connection tests, keyed by the server's `error_code`.
 * `forward_target_refused` comes from proxy-host saves and tests, `destination_refused` from monitor creation
 * and tests, and `connection_test_failed` from the test endpoints.
 */
const CONNECTION_TEST_CODE_MESSAGE_KEYS = {
  forward_target_refused: 'errors.forwardTargetRefused',
  destination_refused: 'errors.destinationRefused',
  connection_test_failed: 'errors.connectionTestFailed',
} as const

type ConnectionTestCodedMessage = keyof typeof CONNECTION_TEST_CODE_MESSAGE_KEYS

const CONNECTION_TEST_CODED_MESSAGES = Object.keys(CONNECTION_TEST_CODE_MESSAGE_KEYS) as ConnectionTestCodedMessage[]

/**
 * Translated text for a refused destination, a failed connection test or a throttled test (HTTP 429). Used by
 * proxy-host saves and tests, remote server tests and uptime monitor creation.
 * @param err - The value thrown by the request
 * @param t - The translation function of the calling component or hook
 * @returns The text, or null when the error has no text of its own
 */
export function connectionTestErrorMessage(err: unknown, t: TFunction): string | null {
  const waitMessage = rateLimitMessage(t, err)
  if (waitMessage !== null) return waitMessage
  const code = getApiErrorCode(err, CONNECTION_TEST_CODED_MESSAGES)
  return code ? t(CONNECTION_TEST_CODE_MESSAGE_KEYS[code]) : null
}
