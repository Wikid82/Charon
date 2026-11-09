import { AxiosError, type AxiosResponse } from 'axios'
import { describe, it, expect } from 'vitest'

import { accessListErrorMessage } from '../errorBody'

import type { TFunction } from 'i18next'

// Stand-in for i18next's t(): echoes the key so assertions name the message used.
const t = ((key: string) => `translated:${key}`) as unknown as TFunction

const axiosError = (data: unknown): AxiosError => {
  const response = { status: 400, statusText: 'Bad Request', headers: {}, config: {}, data } as AxiosResponse
  return new AxiosError('Request failed with status code 400', 'ERR_BAD_REQUEST', undefined, undefined, response)
}

describe('accessListErrorMessage', () => {
  it('maps access_list_rules_required to its translated text', () => {
    expect(accessListErrorMessage(axiosError({ error_code: 'access_list_rules_required' }), t)).toBe(
      'translated:accessLists.errors.access_list_rules_required',
    )
  })

  it('returns null for a code it does not know', () => {
    expect(accessListErrorMessage(axiosError({ error_code: 'crowdsec_busy' }), t)).toBeNull()
  })

  it('returns null when the response has no error code', () => {
    expect(accessListErrorMessage(axiosError({ error: 'bad request' }), t)).toBeNull()
  })

  it('returns null for a non-axios error', () => {
    expect(accessListErrorMessage(new Error('network'), t)).toBeNull()
  })
})
