import { AxiosError, type AxiosResponse } from 'axios'
import { describe, it, expect } from 'vitest'

import { connectionTestErrorMessage } from '../errorBody'

import type { TFunction } from 'i18next'

// Stand-in for i18next's t(): echoes the key and any interpolation values so assertions name the message used.
const t = ((key: string, options?: Record<string, unknown>) =>
  options ? `translated:${key}:${JSON.stringify(options)}` : `translated:${key}`) as unknown as TFunction

const axiosError = (status: number, data: unknown, headers: Record<string, string> = {}): AxiosError => {
  const response = { status, statusText: '', headers, config: {}, data } as AxiosResponse
  return new AxiosError(`Request failed with status code ${status}`, 'ERR_BAD_REQUEST', undefined, undefined, response)
}

describe('connectionTestErrorMessage', () => {
  it.each([
    ['forward_target_refused', 'translated:errors.forwardTargetRefused'],
    ['destination_refused', 'translated:errors.destinationRefused'],
    ['connection_test_failed', 'translated:errors.connectionTestFailed'],
  ])('maps %s to its translated text', (code, expected) => {
    expect(connectionTestErrorMessage(axiosError(400, { error_code: code }), t)).toBe(expected)
  })

  it('maps a throttled test (429) with a Retry-After in seconds to the seconds text', () => {
    const err = axiosError(429, {}, { 'retry-after': '42' })
    expect(connectionTestErrorMessage(err, t)).toBe('translated:errors.tooManyRequestsSeconds:{"count":42}')
  })

  it('maps a throttled test (429) with a longer Retry-After to whole minutes', () => {
    const err = axiosError(429, {}, { 'retry-after': '120' })
    expect(connectionTestErrorMessage(err, t)).toBe('translated:errors.tooManyRequestsMinutes:{"count":2}')
  })

  it('maps a throttled test (429) without Retry-After to the generic text', () => {
    expect(connectionTestErrorMessage(axiosError(429, {}), t)).toBe('translated:errors.tooManyRequests')
  })

  it('returns null for a code it does not know', () => {
    expect(connectionTestErrorMessage(axiosError(400, { error_code: 'redirect_target_invalid' }), t)).toBeNull()
  })

  it('returns null for an error without a code', () => {
    expect(connectionTestErrorMessage(axiosError(400, { error: 'bad input' }), t)).toBeNull()
  })

  it('returns null for a non-axios error', () => {
    expect(connectionTestErrorMessage(new Error('boom'), t)).toBeNull()
  })
})
