import { renderHook, act } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'

import { useWidgetVisibility } from '../useWidgetVisibility'
import type { WidgetKey } from '../useWidgetVisibility'

const STORAGE_KEY = 'charon.stats.widgetVisibility'

const ALL_KEYS: WidgetKey[] = [
  'requestCount',
  'trafficVolume',
  'topHosts',
  'statusDistribution',
  'certExpiry',
  'serviceHealth',
]

beforeEach(() => {
  localStorage.clear()
})

describe('useWidgetVisibility', () => {
  it('all widgets are visible by default', () => {
    const { result } = renderHook(() => useWidgetVisibility())

    for (const key of ALL_KEYS) {
      expect(result.current.visibility[key]).toBe(true)
    }
  })

  it('toggleWidget hides a visible widget', () => {
    const { result } = renderHook(() => useWidgetVisibility())

    act(() => {
      result.current.toggleWidget('requestCount')
    })

    expect(result.current.visibility.requestCount).toBe(false)
  })

  it('toggleWidget shows a hidden widget', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ requestCount: false, trafficVolume: true, topHosts: true, statusDistribution: true, certExpiry: true, serviceHealth: true })
    )

    const { result } = renderHook(() => useWidgetVisibility())

    expect(result.current.visibility.requestCount).toBe(false)

    act(() => {
      result.current.toggleWidget('requestCount')
    })

    expect(result.current.visibility.requestCount).toBe(true)
  })

  it('persists visibility state to localStorage after toggle', () => {
    const { result } = renderHook(() => useWidgetVisibility())

    act(() => {
      result.current.toggleWidget('topHosts')
    })

    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}') as Record<string, boolean>
    expect(stored.topHosts).toBe(false)
  })

  it('loads persisted state from localStorage on mount', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ requestCount: false, trafficVolume: false, topHosts: true, statusDistribution: true, certExpiry: true, serviceHealth: true })
    )

    const { result } = renderHook(() => useWidgetVisibility())

    expect(result.current.visibility.requestCount).toBe(false)
    expect(result.current.visibility.trafficVolume).toBe(false)
    expect(result.current.visibility.topHosts).toBe(true)
  })

  it('resetAll restores all widgets to visible', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ requestCount: false, trafficVolume: false, topHosts: false, statusDistribution: false, certExpiry: false, serviceHealth: false })
    )

    const { result } = renderHook(() => useWidgetVisibility())

    act(() => {
      result.current.resetAll()
    })

    for (const key of ALL_KEYS) {
      expect(result.current.visibility[key]).toBe(true)
    }
  })

  it('resetAll writes defaults back to localStorage', () => {
    const { result } = renderHook(() => useWidgetVisibility())

    act(() => {
      result.current.toggleWidget('certExpiry')
    })
    act(() => {
      result.current.resetAll()
    })

    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}') as Record<string, boolean>
    expect(stored.certExpiry).toBe(true)
  })

  it('falls back to defaults when localStorage contains invalid JSON', () => {
    localStorage.setItem(STORAGE_KEY, 'not-valid-json{{{')

    const { result } = renderHook(() => useWidgetVisibility())

    for (const key of ALL_KEYS) {
      expect(result.current.visibility[key]).toBe(true)
    }
  })

  it('falls back to defaults when localStorage contains non-object value', () => {
    localStorage.setItem(STORAGE_KEY, '"just-a-string"')

    const { result } = renderHook(() => useWidgetVisibility())

    for (const key of ALL_KEYS) {
      expect(result.current.visibility[key]).toBe(true)
    }
  })

  it('ignores unknown keys in localStorage and keeps defaults for known keys', () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ unknownKey: false, requestCount: false })
    )

    const { result } = renderHook(() => useWidgetVisibility())

    expect(result.current.visibility.requestCount).toBe(false)
    expect(result.current.visibility.trafficVolume).toBe(true)
  })
})
