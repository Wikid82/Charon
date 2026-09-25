import { AxiosHeaders } from 'axios'
import { describe, expect, it, vi } from 'vitest'

import {
  getRetryAfterSeconds,
  isRateLimitError,
  parseRetryAfter,
  rateLimitMessage,
} from '../rateLimit'

import type { TFunction } from 'i18next'

const NOW = Date.parse('2026-09-24T10:00:00Z')

const makeT = () =>
  vi.fn((key: string, options?: Record<string, unknown>) =>
    options ? `${key}:${JSON.stringify(options)}` : key
  ) as unknown as TFunction

const errorWith = (status: number, headers?: unknown) => ({ response: { status, headers } })

describe('parseRetryAfter', () => {
  it('parses delta-seconds and clamps to at least 1', () => {
    expect(parseRetryAfter('42')).toBe(42)
    expect(parseRetryAfter(' 7 ')).toBe(7)
    expect(parseRetryAfter('0')).toBe(1)
    expect(parseRetryAfter(5)).toBe(5)
    expect(parseRetryAfter(0)).toBe(1)
  })

  it('parses an HTTP-date relative to now, rounding up', () => {
    expect(parseRetryAfter('Thu, 24 Sep 2026 10:00:30 GMT', NOW)).toBe(30)
    expect(parseRetryAfter('Thu, 24 Sep 2026 10:00:00 GMT', NOW + 500)).toBe(1)
    expect(parseRetryAfter('Thu, 24 Sep 2026 09:00:00 GMT', NOW)).toBe(1)
  })

  it('defaults now to the current time', () => {
    const future = new Date(Date.now() + 90_000).toUTCString()
    const seconds = parseRetryAfter(future)
    expect(seconds).toBeGreaterThanOrEqual(88)
    expect(seconds).toBeLessThanOrEqual(90)
  })

  it.each([undefined, null, '', '   ', 'soon', '-5', '1.5', 1.5, -1, {}, []])(
    'returns null for unusable value %j',
    (value) => {
      expect(parseRetryAfter(value)).toBeNull()
    }
  )
})

describe('getRetryAfterSeconds', () => {
  it('reads AxiosHeaders', () => {
    const headers = new AxiosHeaders({ 'Retry-After': '12' })
    expect(getRetryAfterSeconds(errorWith(429, headers))).toBe(12)
  })

  it('reads plain-object headers case-insensitively', () => {
    expect(getRetryAfterSeconds(errorWith(429, { 'Retry-After': '9' }))).toBe(9)
    expect(getRetryAfterSeconds(errorWith(429, { 'retry-after': '3' }))).toBe(3)
  })

  it('returns null without a usable header or response', () => {
    expect(getRetryAfterSeconds(errorWith(429, {}))).toBeNull()
    expect(getRetryAfterSeconds(errorWith(429, 'nope'))).toBeNull()
    expect(getRetryAfterSeconds(errorWith(429))).toBeNull()
    expect(getRetryAfterSeconds(new Error('x'))).toBeNull()
    expect(getRetryAfterSeconds(null)).toBeNull()
    expect(getRetryAfterSeconds('string')).toBeNull()
  })
})

describe('isRateLimitError', () => {
  it('is true only for a 429 response', () => {
    expect(isRateLimitError(errorWith(429))).toBe(true)
    expect(isRateLimitError(errorWith(401))).toBe(false)
    expect(isRateLimitError(new Error('x'))).toBe(false)
    expect(isRateLimitError(undefined)).toBe(false)
  })
})

describe('rateLimitMessage', () => {
  it('returns null for non-429 errors', () => {
    expect(rateLimitMessage(makeT(), errorWith(500, { 'retry-after': '5' }))).toBeNull()
    expect(rateLimitMessage(makeT(), new Error('x'))).toBeNull()
  })

  it('uses seconds under a minute', () => {
    expect(rateLimitMessage(makeT(), errorWith(429, { 'retry-after': '42' }))).toBe(
      'errors.tooManyRequestsSeconds:{"count":42}'
    )
    expect(rateLimitMessage(makeT(), errorWith(429, { 'retry-after': '59' }))).toBe(
      'errors.tooManyRequestsSeconds:{"count":59}'
    )
  })

  it('rounds up to whole minutes from 60 seconds', () => {
    expect(rateLimitMessage(makeT(), errorWith(429, { 'retry-after': '60' }))).toBe(
      'errors.tooManyRequestsMinutes:{"count":1}'
    )
    expect(rateLimitMessage(makeT(), errorWith(429, { 'retry-after': '61' }))).toBe(
      'errors.tooManyRequestsMinutes:{"count":2}'
    )
  })

  it('falls back to the generic message without Retry-After', () => {
    expect(rateLimitMessage(makeT(), errorWith(429, {}))).toBe('errors.tooManyRequests')
  })
})
