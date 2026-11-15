import { AxiosError, type AxiosResponse } from 'axios'
import { describe, it, expect } from 'vitest'

import { accessListDeleteErrorMessage, accessListErrorMessage } from '../errorBody'

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

describe('accessListErrorMessage (management codes)', () => {
  it.each([
    ['management_list_unknown', 'translated:errors.managementListUnknown'],
    ['management_access_would_lock_out', 'translated:errors.managementAccessWouldLockOut'],
    ['access_list_invalid', 'translated:errors.accessListInvalid'],
  ])('maps %s to its translated text', (code, expected) => {
    expect(accessListErrorMessage(axiosError({ error_code: code }), t)).toBe(expected)
  })
})

describe('accessListDeleteErrorMessage', () => {
  it('uses the coded text when the server sends a known code', () => {
    expect(accessListDeleteErrorMessage(axiosError({ error_code: 'access_list_rules_required' }), t)).toBe(
      'translated:accessLists.errors.access_list_rules_required',
    )
  })

  it('uses the translated text for a list in use by Charon\'s own interface, ignoring the server text', () => {
    const err = new AxiosError('Request failed with status code 409', 'ERR_BAD_REQUEST', undefined, undefined, {
      status: 409,
      statusText: 'Conflict',
      headers: {},
      config: {},
      data: { error: "access list is used by Charon's own interface", error_code: 'access_list_in_use' },
    } as AxiosResponse)
    expect(accessListDeleteErrorMessage(err, t)).toBe('translated:accessLists.errors.access_list_in_use')
  })

  it('shows the server text for a 409 refusal without a code', () => {
    const err = new AxiosError('Request failed with status code 409', 'ERR_BAD_REQUEST', undefined, undefined, {
      status: 409,
      statusText: 'Conflict',
      headers: {},
      config: {},
      data: { error: "access list is used by Charon's own interface" },
    } as AxiosResponse)
    expect(accessListDeleteErrorMessage(err, t)).toBe("access list is used by Charon's own interface")
  })

  it('returns null for a 409 without a usable message', () => {
    const err = new AxiosError('Request failed with status code 409', 'ERR_BAD_REQUEST', undefined, undefined, {
      status: 409,
      statusText: 'Conflict',
      headers: {},
      config: {},
      data: { error: '' },
    } as AxiosResponse)
    expect(accessListDeleteErrorMessage(err, t)).toBeNull()
  })

  it('returns null for other refusals and for non-HTTP failures', () => {
    expect(accessListDeleteErrorMessage(axiosError({ error: 'bad request' }), t)).toBeNull()
    expect(accessListDeleteErrorMessage(new Error('network down'), t)).toBeNull()
  })
})
