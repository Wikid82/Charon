import { AxiosError, type AxiosResponse } from 'axios'
import { describe, it, expect } from 'vitest'

import { trustedProxyErrorMessage } from '../errorBody'

import type { TFunction } from 'i18next'

// Stand-in for i18next's t(): echoes the key and the entry so assertions name the message used.
const t = ((key: string, options?: { entry?: string }) => `translated:${key}:${options?.entry ?? ''}`) as unknown as TFunction

const axiosError = (data: unknown): AxiosError => {
  const response = { status: 400, statusText: 'Bad Request', headers: {}, config: {}, data } as AxiosResponse
  return new AxiosError('Request failed with status code 400', 'ERR_BAD_REQUEST', undefined, undefined, response)
}

describe('trustedProxyErrorMessage', () => {
  it('names the refused entry for an invalid entry', () => {
    const err = axiosError({ error_code: 'trusted_proxy_invalid', entry: 'not-an-address' })
    expect(trustedProxyErrorMessage(err, t)).toBe('translated:errors.trustedProxyInvalid:not-an-address')
  })

  it('names the refused entry for a too-broad entry', () => {
    const err = axiosError({ error_code: 'trusted_proxy_too_broad', entry: '0.0.0.0/0' })
    expect(trustedProxyErrorMessage(err, t)).toBe('translated:errors.trustedProxyTooBroad:0.0.0.0/0')
  })

  it('maps a list over the entry limit without an entry', () => {
    const err = axiosError({ error_code: 'trusted_proxies_too_many' })
    expect(trustedProxyErrorMessage(err, t)).toBe('translated:errors.trustedProxiesTooMany:')
  })

  it('passes an empty entry when the server sent a non-string entry', () => {
    const err = axiosError({ error_code: 'trusted_proxy_invalid', entry: 42 })
    expect(trustedProxyErrorMessage(err, t)).toBe('translated:errors.trustedProxyInvalid:')
  })

  it('returns null for a code it does not know', () => {
    expect(trustedProxyErrorMessage(axiosError({ error_code: 'crowdsec_busy' }), t)).toBeNull()
  })

  it('returns null for a failure that is not an HTTP response', () => {
    expect(trustedProxyErrorMessage(new Error('network down'), t)).toBeNull()
  })
})
