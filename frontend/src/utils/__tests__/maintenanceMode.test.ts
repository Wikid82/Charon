import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  enterMaintenance,
  isMaintenanceActive,
  isMaintenanceError,
  leaveMaintenance,
  useMaintenanceMode,
} from '../maintenanceMode'

describe('maintenanceMode store', () => {
  afterEach(() => leaveMaintenance())

  it('toggles and notifies hook subscribers once per change', () => {
    const { result } = renderHook(() => useMaintenanceMode())
    expect(result.current).toBe(false)

    act(() => enterMaintenance())
    expect(result.current).toBe(true)
    expect(isMaintenanceActive()).toBe(true)

    act(() => enterMaintenance())
    expect(result.current).toBe(true)

    act(() => leaveMaintenance())
    expect(result.current).toBe(false)
    act(() => leaveMaintenance())
    expect(result.current).toBe(false)
  })

  it('does not re-render subscribers on a repeated enter', () => {
    const render = vi.fn()
    renderHook(() => {
      render()
      return useMaintenanceMode()
    })
    act(() => enterMaintenance())
    const calls = render.mock.calls.length
    act(() => enterMaintenance())
    expect(render.mock.calls.length).toBe(calls)
  })
})

describe('isMaintenanceError', () => {
  it('is true only for a 503 whose body says maintenance:true', () => {
    expect(isMaintenanceError({ response: { status: 503, data: { maintenance: true } } })).toBe(true)
    expect(isMaintenanceError({ response: { status: 503, data: { maintenance: false } } })).toBe(false)
    expect(isMaintenanceError({ response: { status: 503, data: 'text' } })).toBe(false)
    expect(isMaintenanceError({ response: { status: 503, data: null } })).toBe(false)
    expect(isMaintenanceError({ response: { status: 500, data: { maintenance: true } } })).toBe(false)
    expect(isMaintenanceError({ response: { status: 401, data: { maintenance: true } } })).toBe(false)
    expect(isMaintenanceError({})).toBe(false)
    expect(isMaintenanceError(null)).toBe(false)
    expect(isMaintenanceError('boom')).toBe(false)
  })
})
