import { render } from '@testing-library/react'
import { describe, it, expect, beforeEach, afterEach } from 'vitest'

import '@testing-library/jest-dom/vitest'
import { ThemePreviewOverlay } from '../ThemePreviewOverlay'

describe('ThemePreviewOverlay', () => {
  const originalDataTheme = document.documentElement.getAttribute('data-theme')

  beforeEach(() => {
    document.documentElement.setAttribute('data-theme', 'dark')
  })

  afterEach(() => {
    if (originalDataTheme !== null) {
      document.documentElement.setAttribute('data-theme', originalDataTheme)
    } else {
      document.documentElement.removeAttribute('data-theme')
    }
  })

  it('renders null — no DOM output', () => {
    const { container } = render(
      <ThemePreviewOverlay previewTheme={null} resolvedCurrentTheme="dark" />
    )
    expect(container.firstChild).toBeNull()
  })

  it('sets data-theme when previewTheme is non-null', () => {
    render(<ThemePreviewOverlay previewTheme="light" resolvedCurrentTheme="dark" />)
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('sets data-theme to resolved value for built-in themes', () => {
    render(<ThemePreviewOverlay previewTheme="solarized" resolvedCurrentTheme="dark" />)
    expect(document.documentElement.getAttribute('data-theme')).toBe('solarized')
  })

  it('sets data-theme to high-contrast-dark', () => {
    render(<ThemePreviewOverlay previewTheme="high-contrast-dark" resolvedCurrentTheme="dark" />)
    expect(document.documentElement.getAttribute('data-theme')).toBe('high-contrast-dark')
  })

  it('restores resolvedCurrentTheme when previewTheme is null', () => {
    const { rerender } = render(
      <ThemePreviewOverlay previewTheme="light" resolvedCurrentTheme="dark" />
    )
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')

    rerender(<ThemePreviewOverlay previewTheme={null} resolvedCurrentTheme="dark" />)
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
  })

  it('restores to light when resolvedCurrentTheme is light', () => {
    const { rerender } = render(
      <ThemePreviewOverlay previewTheme="solarized" resolvedCurrentTheme="light" />
    )
    rerender(<ThemePreviewOverlay previewTheme={null} resolvedCurrentTheme="light" />)
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('resolves "system" previewTheme using OS preference', () => {
    // jsdom does not support matchMedia — it returns false for matches
    // so system resolves to 'dark' in the test environment
    render(<ThemePreviewOverlay previewTheme="system" resolvedCurrentTheme="dark" />)
    const attr = document.documentElement.getAttribute('data-theme')
    expect(['dark', 'light']).toContain(attr)
  })

  it('resolves "custom" previewTheme to "custom" data-theme', () => {
    render(<ThemePreviewOverlay previewTheme="custom" resolvedCurrentTheme="dark" />)
    expect(document.documentElement.getAttribute('data-theme')).toBe('custom')
  })

  it('updates data-theme when previewTheme changes', () => {
    const { rerender } = render(
      <ThemePreviewOverlay previewTheme="light" resolvedCurrentTheme="dark" />
    )
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')

    rerender(<ThemePreviewOverlay previewTheme="solarized" resolvedCurrentTheme="dark" />)
    expect(document.documentElement.getAttribute('data-theme')).toBe('solarized')
  })
})
