import { AxiosError, type AxiosResponse } from 'axios'
import { describe, it, expect } from 'vitest'

import { permissionsErrorMessage } from '../errorBody'

import type { TFunction } from 'i18next'

// Stand-in for i18next's t(): echoes the key so assertions name the message used.
const t = ((key: string) => `translated:${key}`) as unknown as TFunction

const axiosError = (data: unknown): AxiosError => {
  const response = { status: 403, statusText: 'Forbidden', headers: {}, config: {}, data } as AxiosResponse
  return new AxiosError('Request failed with status code 403', 'ERR_BAD_REQUEST', undefined, undefined, response)
}

describe('permissionsErrorMessage', () => {
  it('maps permissions_admin_only to its translated text', () => {
    expect(permissionsErrorMessage(axiosError({ error_code: 'permissions_admin_only' }), t)).toBe(
      'translated:permissions.adminOnlyError',
    )
  })

  it('returns null for a code it does not know', () => {
    expect(permissionsErrorMessage(axiosError({ error_code: 'redirect_target_invalid' }), t)).toBeNull()
  })

  it('returns null for a non-axios error', () => {
    expect(permissionsErrorMessage(new Error('boom'), t)).toBeNull()
  })
})
