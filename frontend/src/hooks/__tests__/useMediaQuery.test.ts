import { renderHook, act } from '@testing-library/react'
import { createElement } from 'react'
import { renderToString } from 'react-dom/server'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { useMediaQuery } from '../useMediaQuery'

type ChangeListener = (e: MediaQueryListEvent) => void

/** Builds a stateful matchMedia stub: `matches` can change, and change listeners are tracked. */
function createMediaQueryList(initialMatches: boolean) {
  const listeners = new Set<ChangeListener>()
  const mql = {
    matches: initialMatches,
    addEventListener: vi.fn((_event: string, listener: ChangeListener) => {
      listeners.add(listener)
    }),
    removeEventListener: vi.fn((_event: string, listener: ChangeListener) => {
      listeners.delete(listener)
    }),
  }
  const fireChange = (matches: boolean) => {
    mql.matches = matches
    listeners.forEach(listener => listener({ matches } as MediaQueryListEvent))
  }
  return { mql, fireChange, listenerCount: () => listeners.size }
}

describe('useMediaQuery', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      configurable: true,
      value: vi.fn(),
    })
    vi.restoreAllMocks()
  })

  it('returns false when media query does not match', () => {
    const { mql } = createMediaQueryList(false)
    vi.spyOn(window, 'matchMedia').mockReturnValue(mql as unknown as MediaQueryList)
    const { result } = renderHook(() => useMediaQuery('(max-width: 1023px)'))
    expect(result.current).toBe(false)
  })

  it('returns true when media query matches', () => {
    const { mql } = createMediaQueryList(true)
    vi.spyOn(window, 'matchMedia').mockReturnValue(mql as unknown as MediaQueryList)
    const { result } = renderHook(() => useMediaQuery('(max-width: 1023px)'))
    expect(result.current).toBe(true)
  })

  it('updates when a change event reports a new match state', () => {
    const { mql, fireChange } = createMediaQueryList(false)
    vi.spyOn(window, 'matchMedia').mockReturnValue(mql as unknown as MediaQueryList)
    const { result } = renderHook(() => useMediaQuery('(max-width: 1023px)'))
    expect(result.current).toBe(false)
    act(() => {
      fireChange(true)
    })
    expect(result.current).toBe(true)
    act(() => {
      fireChange(false)
    })
    expect(result.current).toBe(false)
  })

  it('updates matches immediately when query prop changes', () => {
    vi.spyOn(window, 'matchMedia').mockImplementation((q: string) => ({
      matches: q === '(min-width: 768px)',
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList))

    const { result, rerender } = renderHook(
      ({ query }: { query: string }) => useMediaQuery(query),
      { initialProps: { query: '(max-width: 767px)' } }
    )
    expect(result.current).toBe(false)

    rerender({ query: '(min-width: 768px)' })
    expect(result.current).toBe(true)
  })

  it('moves the change listener to the new query when the query prop changes', () => {
    const first = createMediaQueryList(false)
    const second = createMediaQueryList(true)
    vi.spyOn(window, 'matchMedia').mockImplementation((q: string) =>
      (q === 'first' ? first.mql : second.mql) as unknown as MediaQueryList
    )

    const { rerender } = renderHook(({ query }: { query: string }) => useMediaQuery(query), {
      initialProps: { query: 'first' },
    })
    expect(first.listenerCount()).toBe(1)

    rerender({ query: 'second' })
    expect(first.listenerCount()).toBe(0)
    expect(second.listenerCount()).toBe(1)
  })

  it('removes its change listener on unmount', () => {
    const { mql, listenerCount } = createMediaQueryList(false)
    vi.spyOn(window, 'matchMedia').mockReturnValue(mql as unknown as MediaQueryList)
    const { unmount } = renderHook(() => useMediaQuery('(max-width: 1023px)'))
    expect(listenerCount()).toBe(1)
    unmount()
    expect(mql.removeEventListener).toHaveBeenCalledTimes(1)
    expect(listenerCount()).toBe(0)
  })

  it('reports false during server rendering regardless of the media query', () => {
    const { mql } = createMediaQueryList(true)
    vi.spyOn(window, 'matchMedia').mockReturnValue(mql as unknown as MediaQueryList)
    function Probe() {
      return createElement('span', null, String(useMediaQuery('(max-width: 1023px)')))
    }
    expect(renderToString(createElement(Probe))).toBe('<span>false</span>')
  })
})
