import { renderHook } from '@testing-library/react'
import { describe, it, expect } from 'vitest'

import { ThemeProvider } from '../../context/ThemeContext'
import { useTheme } from '../useTheme'

function wrapper({ children }: { children: React.ReactNode }) {
  return <ThemeProvider>{children}</ThemeProvider>
}

describe('useTheme', () => {
  it('throws error when used outside ThemeProvider', () => {
    // Suppress console.error for this test as React logs the error
    const consoleSpy = vi.spyOn(console, 'error')
    consoleSpy.mockImplementation(() => {})

    expect(() => {
      renderHook(() => useTheme())
    }).toThrow('useTheme must be used within a ThemeProvider')

    consoleSpy.mockRestore()
  })

  it('returns the full context shape when used inside ThemeProvider', () => {
    const { result } = renderHook(() => useTheme(), { wrapper })

    expect(result.current).toBeDefined()
    expect(typeof result.current.theme).toBe('string')
    expect(typeof result.current.resolvedTheme).toBe('string')
    expect(typeof result.current.setTheme).toBe('function')
    expect(typeof result.current.setCustomTheme).toBe('function')
    expect(typeof result.current.exportTheme).toBe('function')
    expect(typeof result.current.importTheme).toBe('function')
  })

  it('returns dark as the default theme', () => {
    localStorage.clear()
    const { result } = renderHook(() => useTheme(), { wrapper })
    expect(result.current.theme).toBe('dark')
  })

  it('returns null for customTheme when none is saved', () => {
    localStorage.clear()
    const { result } = renderHook(() => useTheme(), { wrapper })
    expect(result.current.customTheme).toBeNull()
  })

  it('returns a valid resolvedTheme that is a string', () => {
    localStorage.clear()
    const { result } = renderHook(() => useTheme(), { wrapper })
    const validResolved = ['dark', 'light', 'high-contrast-dark', 'high-contrast-light', 'solarized', 'custom']
    expect(validResolved).toContain(result.current.resolvedTheme)
  })

  it('exportTheme returns an object with version 1', () => {
    const { result } = renderHook(() => useTheme(), { wrapper })
    const exported = result.current.exportTheme()
    expect(exported.version).toBe(1)
    expect(typeof exported.exportedAt).toBe('string')
    expect(typeof exported.theme).toBe('string')
  })
})
