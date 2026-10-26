import { AxiosError, type AxiosResponse } from 'axios'
import type { TFunction } from 'i18next'
import { describe, it, expect } from 'vitest'

import { crowdsecBusyMessage, crowdsecErrorMessage, getApiErrorCode, getCrowdsecBusyJobId, isCrowdsecBusyError } from '../errorBody'

const BUSY_TEXT = 'CrowdSec is busy installing a preset.'
const BUSY_OTHER_TEXT = 'CrowdSec is finishing another change.'

// Stand-in for i18next's t(): echoes the key so assertions name the message used.
const t = ((key: string) => {
  if (key === 'crowdsecConfig.busy') return BUSY_TEXT
  if (key === 'crowdsecConfig.busyOther') return BUSY_OTHER_TEXT
  return `translated:${key}`
}) as unknown as TFunction

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

  it('is true for a crowdsec_busy refusal without a job_id', () => {
    expect(isCrowdsecBusyError(axiosError({ error_code: 'crowdsec_busy' }))).toBe(true)
  })

  it('is false for other refusals and plain errors', () => {
    expect(isCrowdsecBusyError(axiosError({ error: 'already running' }))).toBe(false)
    expect(isCrowdsecBusyError(new Error('boom'))).toBe(false)
    expect(isCrowdsecBusyError(undefined)).toBe(false)
  })
})

describe('getCrowdsecBusyJobId', () => {
  it('returns the job id named by a crowdsec_busy refusal', () => {
    expect(getCrowdsecBusyJobId(axiosError({ error_code: 'crowdsec_busy', job_id: 'abc' }))).toBe('abc')
  })

  it('returns undefined when the refusal names no job, or an empty one', () => {
    expect(getCrowdsecBusyJobId(axiosError({ error_code: 'crowdsec_busy' }))).toBeUndefined()
    expect(getCrowdsecBusyJobId(axiosError({ error_code: 'crowdsec_busy', job_id: '' }))).toBeUndefined()
  })

  it('returns undefined when the job id is not a string', () => {
    expect(getCrowdsecBusyJobId(axiosError({ error_code: 'crowdsec_busy', job_id: 42 }))).toBeUndefined()
  })

  it('returns undefined for non-Axios errors', () => {
    expect(getCrowdsecBusyJobId(new Error('x'))).toBeUndefined()
  })
})

describe('crowdsecBusyMessage', () => {
  it('returns the busy text when the refusal names a job', () => {
    expect(crowdsecBusyMessage(axiosError({ error_code: 'crowdsec_busy', job_id: 'job-1' }), t)).toBe(BUSY_TEXT)
  })

  it('returns the other-change text when the refusal names no job', () => {
    expect(crowdsecBusyMessage(axiosError({ error_code: 'crowdsec_busy' }), t)).toBe(BUSY_OTHER_TEXT)
  })

  it('returns null for every error that is not a crowdsec_busy refusal', () => {
    expect(crowdsecBusyMessage(axiosError({ error: 'nope' }), t)).toBeNull()
    expect(crowdsecBusyMessage(new Error('boom'), t)).toBeNull()
    expect(crowdsecBusyMessage(undefined, t)).toBeNull()
  })
})

describe('crowdsecErrorMessage', () => {
  it('returns the busy text for a refusal that names a job, ignoring the fallback', () => {
    expect(crowdsecErrorMessage(axiosError({ error_code: 'crowdsec_busy', job_id: 'job-1' }), t, 'Server text')).toBe(BUSY_TEXT)
  })

  it('returns the other-change text for a refusal without a job, ignoring the fallback', () => {
    expect(crowdsecErrorMessage(axiosError({ error_code: 'crowdsec_busy' }), t, 'Server text')).toBe(BUSY_OTHER_TEXT)
  })

  it('returns the caller fallback for any other error', () => {
    expect(crowdsecErrorMessage(axiosError({ error: 'nope' }), t, 'Server text')).toBe('Server text')
    expect(crowdsecErrorMessage(new Error('boom'), t, 'Generic')).toBe('Generic')
  })
})
