import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { formatAbsoluteTime, formatRelativeTime } from '../formatRelativeTime'

describe('formatRelativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2025-01-10T12:00:00Z'))
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it.each([
    ['2025-01-10T11:59:30Z', '30s ago'],
    ['2025-01-10T11:55:00Z', '5m ago'],
    ['2025-01-10T09:00:00Z', '3h ago'],
    ['2025-01-08T12:00:00Z', '2d ago'],
    ['2025-01-10T12:05:00Z', 'just now'],
  ])('formats %s as %s', (input, expected) => {
    expect(formatRelativeTime(input)).toBe(expected)
  })

  it.each(['', 'not-a-date'])('returns a dash for %j', (input) => {
    expect(formatRelativeTime(input)).toBe('—')
  })
})

describe('formatAbsoluteTime', () => {
  it('returns a localized string for a valid value', () => {
    expect(formatAbsoluteTime('2025-01-10T12:00:00Z')).toBe(
      new Date('2025-01-10T12:00:00Z').toLocaleString(),
    )
  })

  it.each(['', 'not-a-date'])('returns undefined for %j', (input) => {
    expect(formatAbsoluteTime(input)).toBeUndefined()
  })
})
