import { describe, it, expect } from 'vitest'

import { isValidCronExpression, parseCronPreset } from '../cron'

describe('isValidCronExpression', () => {
  it('accepts the default daily cron', () => {
    expect(isValidCronExpression('0 3 * * *')).toBe(true)
  })

  it('accepts a weekly cron with a day-of-week field', () => {
    expect(isValidCronExpression('0 3 * * 0')).toBe(true)
  })

  it('accepts step syntax', () => {
    expect(isValidCronExpression('0 */6 * * *')).toBe(true)
  })

  it('accepts ranges and lists', () => {
    expect(isValidCronExpression('0 9-17 * * 1-5')).toBe(true)
    expect(isValidCronExpression('0,30 9 * * 1,3,5')).toBe(true)
  })

  it('rejects a non-cron string', () => {
    expect(isValidCronExpression('not a cron expression')).toBe(false)
  })

  it('rejects an empty string', () => {
    expect(isValidCronExpression('')).toBe(false)
    expect(isValidCronExpression('   ')).toBe(false)
  })

  it('rejects the wrong number of fields', () => {
    expect(isValidCronExpression('0 3 * *')).toBe(false)
    expect(isValidCronExpression('0 3 * * * *')).toBe(false)
  })

  it('rejects out-of-range values', () => {
    expect(isValidCronExpression('60 3 * * *')).toBe(false) // minute > 59
    expect(isValidCronExpression('0 24 * * *')).toBe(false) // hour > 23
    expect(isValidCronExpression('0 3 32 * *')).toBe(false) // day of month > 31
    expect(isValidCronExpression('0 3 * 13 *')).toBe(false) // month > 12
    expect(isValidCronExpression('0 3 * * 8')).toBe(false) // day of week > 7
  })

  it('rejects an inverted range', () => {
    expect(isValidCronExpression('0 17-9 * * *')).toBe(false)
  })

  it('rejects a zero or negative step', () => {
    expect(isValidCronExpression('*/0 * * * *')).toBe(false)
  })

  it('rejects a malformed field', () => {
    expect(isValidCronExpression('a b c d e')).toBe(false)
  })
})

describe('parseCronPreset', () => {
  it('recognizes a daily cron and normalizes the hour', () => {
    expect(parseCronPreset('0 3 * * *')).toEqual({ frequency: 'daily', time: '03:00', dayOfWeek: '0' })
  })

  it('recognizes a weekly cron with its day of week', () => {
    expect(parseCronPreset('0 6 * * 2')).toEqual({ frequency: 'weekly', time: '06:00', dayOfWeek: '2' })
  })

  it('falls back to custom for anything else', () => {
    expect(parseCronPreset('0 */6 * * *')).toEqual({ frequency: 'custom', time: '03:00', dayOfWeek: '0' })
    expect(parseCronPreset('*/5 * * * *')).toEqual({ frequency: 'custom', time: '03:00', dayOfWeek: '0' })
  })
})
