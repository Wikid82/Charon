import { AxiosError, type AxiosResponse } from 'axios'
import { describe, it, expect } from 'vitest'

import { setupErrorMessage } from '../errorBody'

import type { TFunction } from 'i18next'

// Stand-in for i18next's t(): echoes the key so assertions name the message used.
const t = ((key: string) => `translated:${key}`) as unknown as TFunction

const axiosError = (status: number, data: unknown): AxiosError => {
  const response = { status, statusText: '', headers: {}, config: {}, data } as AxiosResponse
  return new AxiosError(`Request failed with status code ${status}`, 'ERR_BAD_REQUEST', undefined, undefined, response)
}

describe('setupErrorMessage', () => {
  it('maps setup_code_invalid to its translated text', () => {
    expect(setupErrorMessage(axiosError(403, { error_code: 'setup_code_invalid' }), t)).toBe(
      'translated:setup.errors.setup_code_invalid',
    )
  })

  it('maps HTTP 429 to the throttled text whatever the body says', () => {
    expect(setupErrorMessage(axiosError(429, { error: 'Too many requests' }), t)).toBe(
      'translated:setup.errors.rate_limited',
    )
  })

  it('returns null for a 403 without a known code', () => {
    expect(setupErrorMessage(axiosError(403, { error: 'Setup already completed' }), t)).toBeNull()
  })

  it('returns null for a code it does not know', () => {
    expect(setupErrorMessage(axiosError(400, { error_code: 'redirect_target_invalid' }), t)).toBeNull()
  })

  it('returns null for a non-axios error', () => {
    expect(setupErrorMessage(new Error('network'), t)).toBeNull()
  })
})
