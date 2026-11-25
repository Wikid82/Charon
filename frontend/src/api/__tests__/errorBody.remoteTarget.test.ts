import { AxiosError, type AxiosResponse } from 'axios'
import { describe, it, expect } from 'vitest'

import { remoteTargetSaveErrorMessage } from '../errorBody'

const axiosError = (status: number, data: unknown): AxiosError => {
  const response = { status, statusText: '', headers: {}, config: {}, data } as AxiosResponse
  return new AxiosError(`Request failed with status code ${status}`, 'ERR_BAD_REQUEST', undefined, undefined, response)
}

describe('remoteTargetSaveErrorMessage', () => {
  it('shows the server text of a 400 refusal', () => {
    const err = axiosError(400, { error: 'Use an https:// address.', error_code: 'webdav_https_required' })

    expect(remoteTargetSaveErrorMessage(err)).toBe('Use an https:// address.')
  })

  it('keeps the request message when a 400 carries no text', () => {
    expect(remoteTargetSaveErrorMessage(axiosError(400, { error: '' }))).toBe(
      'Request failed with status code 400',
    )
    expect(remoteTargetSaveErrorMessage(axiosError(400, undefined))).toBe('Request failed with status code 400')
  })

  it('keeps the request message for other statuses', () => {
    expect(remoteTargetSaveErrorMessage(axiosError(500, { error: 'boom' }))).toBe(
      'Request failed with status code 500',
    )
  })

  it('keeps the message of a non-request error', () => {
    expect(remoteTargetSaveErrorMessage(new Error('network down'))).toBe('network down')
  })
})
