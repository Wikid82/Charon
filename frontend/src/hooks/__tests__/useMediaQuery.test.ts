import { renderHook, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useMediaQuery } from '../useMediaQuery'

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
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList)
    const { result } = renderHook(() => useMediaQuery('(max-width: 1023px)'))
    expect(result.current).toBe(false)
  })

  it('returns true when media query matches', () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList)
    const { result } = renderHook(() => useMediaQuery('(max-width: 1023px)'))
    expect(result.current).toBe(true)
  })

  it('updates when media query changes', () => {
    let handler: ((e: MediaQueryListEvent) => void) | undefined
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: false,
      addEventListener: vi.fn((_event, h) => { handler = h }),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList)
    const { result } = renderHook(() => useMediaQuery('(max-width: 1023px)'))
    expect(result.current).toBe(false)
    act(() => {
      handler?.({ matches: true } as MediaQueryListEvent)
    })
    expect(result.current).toBe(true)
  })

  it('returns false in SSR environment when window is undefined', () => {
    vi.stubGlobal('window', undefined)
    try {
      const { result } = renderHook(() => useMediaQuery('(max-width: 1023px)'))
      expect(result.current).toBe(false)
    } finally {
      vi.unstubAllGlobals()
    }
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
})
