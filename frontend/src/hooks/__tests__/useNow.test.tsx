import { renderHook, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import { useNow } from '../useNow'

describe('useNow', () => {
  beforeEach(() => {
    vi.useFakeTimers({ now: new Date('2026-01-01T00:00:00Z') })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns the current time on mount', () => {
    const { result } = renderHook(() => useNow(1000))
    expect(result.current).toBe(Date.parse('2026-01-01T00:00:00Z'))
  })

  it('refreshes the value on each interval tick while enabled', () => {
    const { result } = renderHook(() => useNow(1000))
    act(() => { vi.advanceTimersByTime(2500) })
    expect(result.current).toBe(Date.parse('2026-01-01T00:00:02Z'))
  })

  it('does not tick when disabled', () => {
    const { result } = renderHook(() => useNow(1000, false))
    const initial = result.current
    act(() => { vi.advanceTimersByTime(5000) })
    expect(result.current).toBe(initial)
  })

  it('stops ticking when toggled from enabled to disabled', () => {
    const { result, rerender } = renderHook(
      ({ enabled }: { enabled: boolean }) => useNow(1000, enabled),
      { initialProps: { enabled: true } }
    )
    act(() => { vi.advanceTimersByTime(1000) })
    const frozen = result.current
    rerender({ enabled: false })
    act(() => { vi.advanceTimersByTime(5000) })
    expect(result.current).toBe(frozen)
  })
})
