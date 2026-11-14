import { describe, it, expect } from 'vitest'

import { externalProxyBindErrorMessage } from '../errorBody'

import type { AxiosError } from 'axios'
import type { TFunction } from 'i18next'

const t = ((key: string) => `translated:${key}`) as unknown as TFunction

function axiosError(data: unknown): AxiosError {
  const err = new Error('request failed') as AxiosError
  err.isAxiosError = true
  err.response = { data, status: 400, statusText: 'Bad Request', headers: {}, config: {} } as AxiosError['response']
  return err
}

describe('externalProxyBindErrorMessage', () => {
  it('maps external_proxy_bind_invalid to its translated text', () => {
    expect(externalProxyBindErrorMessage(axiosError({ error_code: 'external_proxy_bind_invalid' }), t)).toBe(
      'translated:hecate.externalProxy.bindInvalid',
    )
  })

  it('returns null for other error codes', () => {
    expect(externalProxyBindErrorMessage(axiosError({ error_code: 'something_else' }), t)).toBeNull()
  })

  it('returns null for non-axios errors', () => {
    expect(externalProxyBindErrorMessage(new Error('boom'), t)).toBeNull()
  })
})
