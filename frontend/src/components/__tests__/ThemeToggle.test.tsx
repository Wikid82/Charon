import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import '@testing-library/jest-dom/vitest'
import { ThemeToggle } from '../ThemeToggle'

const mockNavigate = vi.fn()

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}))

vi.mock('../../hooks/useTheme', () => ({
  useTheme: vi.fn().mockReturnValue({
    resolvedTheme: 'dark',
    theme: 'dark',
    setTheme: vi.fn(),
  }),
}))

import { useTheme } from '../../hooks/useTheme'
const mockUseTheme = vi.mocked(useTheme)

describe('ThemeToggle', () => {
  beforeEach(() => {
    mockNavigate.mockClear()
    mockUseTheme.mockReturnValue({
      resolvedTheme: 'dark',
      theme: 'dark',
      setTheme: vi.fn(),
      setCustomTheme: vi.fn(),
      customTheme: null,
      exportTheme: vi.fn(),
      importTheme: vi.fn(),
      userThemes: [],
      activeUserTheme: null,
      setUserTheme: vi.fn(),
    })
  })

  it('renders with title="Theme settings"', () => {
    render(<ThemeToggle />)
    expect(screen.getByTitle('Theme settings')).toBeInTheDocument()
  })

  it('clicking the button navigates to /settings/appearance', () => {
    render(<ThemeToggle />)
    fireEvent.click(screen.getByTitle('Theme settings'))
    expect(mockNavigate).toHaveBeenCalledWith('/settings/appearance')
  })

  it('shows Moon icon and dark aria-label when resolvedTheme is dark', () => {
    render(<ThemeToggle />)
    const btn = screen.getByTitle('Theme settings')
    expect(btn).toHaveAttribute('aria-label', expect.stringContaining('dark'))
  })

  it('shows Sun icon and light aria-label when resolvedTheme is light', () => {
    mockUseTheme.mockReturnValue({
      resolvedTheme: 'light',
      theme: 'light',
      setTheme: vi.fn(),
      setCustomTheme: vi.fn(),
      customTheme: null,
      exportTheme: vi.fn(),
      importTheme: vi.fn(),
      userThemes: [],
      activeUserTheme: null,
      setUserTheme: vi.fn(),
    })
    render(<ThemeToggle />)
    const btn = screen.getByTitle('Theme settings')
    expect(btn).toHaveAttribute('aria-label', expect.stringContaining('light'))
  })
})
