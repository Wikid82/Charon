import { renderHook, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { useDebouncedValue } from '../useDebouncedValue'

describe('useDebouncedValue', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  it('returns the initial value immediately', () => {
    const { result } = renderHook(() => useDebouncedValue('initial'))
    expect(result.current).toBe('initial')
  })

  it('does not update before the delay has elapsed', () => {
    const { result, rerender } = renderHook(({ value }) => useDebouncedValue(value), {
      initialProps: { value: 'a' },
    })

    rerender({ value: 'b' })
    act(() => {
      vi.advanceTimersByTime(299)
    })

    expect(result.current).toBe('a')
  })

  it('updates after the default 300ms delay', () => {
    const { result, rerender } = renderHook(({ value }) => useDebouncedValue(value), {
      initialProps: { value: 'a' },
    })

    rerender({ value: 'b' })
    act(() => {
      vi.advanceTimersByTime(300)
    })

    expect(result.current).toBe('b')
  })

  it('restarts the timer on rapid changes and only emits the last value', () => {
    const { result, rerender } = renderHook(({ value }) => useDebouncedValue(value), {
      initialProps: { value: 'a' },
    })

    rerender({ value: 'b' })
    act(() => {
      vi.advanceTimersByTime(200)
    })
    rerender({ value: 'c' })
    act(() => {
      vi.advanceTimersByTime(200)
    })
    expect(result.current).toBe('a')

    act(() => {
      vi.advanceTimersByTime(100)
    })
    expect(result.current).toBe('c')
  })

  it('honors a custom delay', () => {
    const { result, rerender } = renderHook(({ value }) => useDebouncedValue(value, 50), {
      initialProps: { value: 1 },
    })

    rerender({ value: 2 })
    act(() => {
      vi.advanceTimersByTime(50)
    })

    expect(result.current).toBe(2)
  })

  it('cleans up the pending timer on unmount', () => {
    const clearSpy = vi.spyOn(globalThis, 'clearTimeout')
    const { unmount, rerender } = renderHook(({ value }) => useDebouncedValue(value), {
      initialProps: { value: 'a' },
    })

    rerender({ value: 'b' })
    unmount()

    expect(clearSpy).toHaveBeenCalled()
    clearSpy.mockRestore()
  })
})
