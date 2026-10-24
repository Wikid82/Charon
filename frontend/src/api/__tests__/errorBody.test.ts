import { AxiosError, type AxiosResponse } from 'axios'
import type { TFunction } from 'i18next'
import { describe, it, expect } from 'vitest'

import { crowdsecErrorMessage, getApiErrorCode, isCrowdsecBusyError } from '../errorBody'

const BUSY_TEXT = 'CrowdSec is busy installing a preset.'

// Stand-in for i18next's t(): echoes the key so assertions name the message used.
const t = ((key: string) => (key === 'crowdsecConfig.busy' ? BUSY_TEXT : `translated:${key}`)) as unknown as TFunction

const axiosError = (data: unknown): AxiosError => {
  const response = { status: 409, statusText: 'Conflict', headers: {}, config: {}, data } as AxiosResponse
  return new AxiosError('Request failed with status code 409', 'ERR_BAD_REQUEST', undefined, undefined, response)
}

describe('getApiErrorCode', () => {
  it('returns the error_code when it is one of the known codes', () => {
    expect(getApiErrorCode(axiosError({ error_code: 'crowdsec_busy' }), ['crowdsec_busy'] as const)).toBe('crowdsec_busy')
  })

  it('returns undefined when the code is not in the known list', () => {
    expect(getApiErrorCode(axiosError({ error_code: 'other' }), ['crowdsec_busy'] as const)).toBeUndefined()
  })

  it('returns undefined when the response has no error_code', () => {
    expect(getApiErrorCode(axiosError({ error: 'conflict' }), ['crowdsec_busy'] as const)).toBeUndefined()
  })

  it('returns undefined when the response has no body', () => {
    expect(getApiErrorCode(new AxiosError('network down'), ['crowdsec_busy'] as const)).toBeUndefined()
  })

  it('returns undefined for non-Axios errors', () => {
    expect(getApiErrorCode(new Error('boom'), ['crowdsec_busy'] as const)).toBeUndefined()
    expect(getApiErrorCode(null, ['crowdsec_busy'] as const)).toBeUndefined()
  })
})

describe('isCrowdsecBusyError', () => {
  it('is true for a crowdsec_busy refusal', () => {
    expect(isCrowdsecBusyError(axiosError({ error_code: 'crowdsec_busy', job_id: 'job-1' }))).toBe(true)
  })

  it('is false for other refusals and plain errors', () => {
    expect(isCrowdsecBusyError(axiosError({ error: 'already running' }))).toBe(false)
    expect(isCrowdsecBusyError(new Error('boom'))).toBe(false)
    expect(isCrowdsecBusyError(undefined)).toBe(false)
  })
})

describe('crowdsecErrorMessage', () => {
  it('returns the busy text for a crowdsec_busy refusal, ignoring the fallback', () => {
    expect(crowdsecErrorMessage(axiosError({ error_code: 'crowdsec_busy' }), t, 'Server text')).toBe(BUSY_TEXT)
  })

  it('returns the caller fallback for any other error', () => {
    expect(crowdsecErrorMessage(axiosError({ error: 'nope' }), t, 'Server text')).toBe('Server text')
    expect(crowdsecErrorMessage(new Error('boom'), t, 'Generic')).toBe('Generic')
  })
})
