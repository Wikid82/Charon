import { describe, it, expect } from 'vitest'

import { isStreamLimitClose, STREAM_LIMIT_CLOSE_CODE, STREAM_LIMIT_CLOSE_REASON } from '../streamLimit'

describe('isStreamLimitClose', () => {
  it('uses close code 1013 (Try Again Later)', () => {
    expect(STREAM_LIMIT_CLOSE_CODE).toBe(1013)
  })

  it('recognises the 1013 close code', () => {
    expect(isStreamLimitClose({ code: 1013, reason: '' })).toBe(true)
  })

  it('recognises the stream_limit_reached reason on any code', () => {
    expect(isStreamLimitClose({ code: 1000, reason: STREAM_LIMIT_CLOSE_REASON })).toBe(true)
  })

  it('does not treat a normal or abnormal close as a limit', () => {
    expect(isStreamLimitClose({ code: 1000, reason: '' })).toBe(false)
    expect(isStreamLimitClose({ code: 1006, reason: '' })).toBe(false)
    expect(isStreamLimitClose({ code: 1011, reason: 'server error' })).toBe(false)
  })
})
