import { AxiosError, type AxiosResponse } from 'axios'
import { describe, it, expect } from 'vitest'

import { redirectionErrorMessage } from '../errorBody'

import type { TFunction } from 'i18next'

// Stand-in for i18next's t(): echoes the key so assertions name the message used.
const t = ((key: string) => `translated:${key}`) as unknown as TFunction

const axiosError = (data: unknown): AxiosError => {
  const response = { status: 400, statusText: 'Bad Request', headers: {}, config: {}, data } as AxiosResponse
  return new AxiosError('Request failed with status code 400', 'ERR_BAD_REQUEST', undefined, undefined, response)
}

describe('redirectionErrorMessage', () => {
  it('maps redirect_target_invalid to its translated text', () => {
    expect(redirectionErrorMessage(axiosError({ error_code: 'redirect_target_invalid' }), t)).toBe(
      'translated:redirectionHosts.errors.redirect_target_invalid',
    )
  })

  it('returns null for a code it does not know', () => {
    expect(redirectionErrorMessage(axiosError({ error_code: 'access_list_rules_required' }), t)).toBeNull()
  })

  it('returns null when the response has no error code', () => {
    expect(redirectionErrorMessage(axiosError({ error: 'target invalid' }), t)).toBeNull()
  })

  it('returns null for a non-axios error', () => {
    expect(redirectionErrorMessage(new Error('network'), t)).toBeNull()
  })
})
