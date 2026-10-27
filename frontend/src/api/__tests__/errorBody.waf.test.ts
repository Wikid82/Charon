import { AxiosError, type AxiosResponse } from 'axios'
import { describe, it, expect } from 'vitest'

import { wafErrorMessage } from '../errorBody'

import type { TFunction } from 'i18next'

// Stand-in for i18next's t(): echoes the key so assertions name the message used.
const t = ((key: string) => `translated:${key}`) as unknown as TFunction

const axiosError = (data: unknown): AxiosError => {
  const response = { status: 409, statusText: 'Conflict', headers: {}, config: {}, data } as AxiosResponse
  return new AxiosError('Request failed', 'ERR_BAD_REQUEST', undefined, undefined, response)
}

describe('wafErrorMessage', () => {
  it.each([
    ['waf_rules_source_unknown', 'translated:wafConfig.errors.waf_rules_source_unknown'],
    ['waf_ruleset_in_use', 'translated:wafConfig.errors.waf_ruleset_in_use'],
    ['invalid_waf_mode', 'translated:wafConfig.errors.invalid_waf_mode'],
  ])('maps the %s code to its translated text', (code, expected) => {
    expect(wafErrorMessage(axiosError({ error_code: code }), t, 'fallback')).toBe(expected)
  })

  it('uses the fallback for a code it does not know', () => {
    expect(wafErrorMessage(axiosError({ error_code: 'crowdsec_busy' }), t, 'fallback')).toBe('fallback')
  })

  it('uses the fallback when the response has no error code', () => {
    expect(wafErrorMessage(axiosError({ error: 'boom' }), t, 'fallback')).toBe('fallback')
  })

  it('uses the fallback for a non-axios error', () => {
    expect(wafErrorMessage(new Error('network'), t, 'fallback')).toBe('fallback')
  })
})
