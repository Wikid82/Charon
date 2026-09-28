import type { TFunction } from 'i18next'

const SECONDS_PER_MINUTE = 60

/**
 * Parses a Retry-After header value into whole seconds (at least 1).
 * Accepts delta-seconds ("42") or an HTTP-date; anything else yields null.
 */
export function parseRetryAfter(value: unknown, nowMs: number = Date.now()): number | null {
  if (typeof value === 'number') {
    return Number.isInteger(value) && value >= 0 ? Math.max(1, value) : null
  }
  if (typeof value !== 'string') return null
  const trimmed = value.trim()
  if (trimmed === '') return null
  if (/^\d+$/.test(trimmed)) {
    return Math.max(1, Number(trimmed))
  }
  // HTTP-dates always start with a weekday name; this keeps Date.parse from accepting junk like "-5"
  if (!/^[A-Za-z]/.test(trimmed)) return null
  const dateMs = Date.parse(trimmed)
  return Number.isNaN(dateMs) ? null : Math.max(1, Math.ceil((dateMs - nowMs) / 1000));
}

interface ErrorWithResponse {
  response?: {
    status?: number
    headers?: unknown
  }
}

function asErrorWithResponse(error: unknown): ErrorWithResponse | null {
  return typeof error === 'object' && error !== null ? (error as ErrorWithResponse) : null
}

/** Reads a header from an AxiosHeaders instance or a plain object (case-insensitive). */
function readHeader(headers: unknown, name: string): unknown {
  if (typeof headers !== 'object' || headers === null) return undefined
  const getter = (headers as { get?: unknown }).get
  if (typeof getter === 'function') {
    return (getter as (key: string) => unknown).call(headers, name)
  }
  const match = Object.keys(headers).find((key) => key.toLowerCase() === name)
  return match === undefined ? undefined : (headers as Record<string, unknown>)[match]
}

/** Seconds to wait according to the error response's Retry-After header, or null. */
export function getRetryAfterSeconds(error: unknown): number | null {
  return parseRetryAfter(readHeader(asErrorWithResponse(error)?.response?.headers, 'retry-after'))
}

/** True when the error carries an HTTP 429 response. */
export function isRateLimitError(error: unknown): boolean {
  return asErrorWithResponse(error)?.response?.status === 429
}

/**
 * Localized "please wait" message for a 429 error, or null for any other error.
 * Under a minute is shown in seconds, longer waits are rounded up to whole minutes.
 */
export function rateLimitMessage(t: TFunction, error: unknown): string | null {
  if (!isRateLimitError(error)) return null
  const seconds = getRetryAfterSeconds(error)
  if (seconds === null) return t('errors.tooManyRequests')
  return seconds < SECONDS_PER_MINUTE ? t('errors.tooManyRequestsSeconds', { count: seconds }) : t('errors.tooManyRequestsMinutes', { count: Math.ceil(seconds / SECONDS_PER_MINUTE) });
}
