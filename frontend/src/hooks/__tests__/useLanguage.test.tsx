import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { ReactNode } from 'react'
import { useLanguage } from '../useLanguage'
import { LanguageProvider } from '../../context/LanguageContext'

// Mock i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: {
      changeLanguage: vi.fn(),
      language: 'en',
    },
  }),
}))

describe('useLanguage', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('throws error when used outside LanguageProvider', () => {
    // Suppress console.error for this test as React logs the error
    const consoleSpy = vi.spyOn(console, 'error')
    consoleSpy.mockImplementation(() => {})

    expect(() => {
      renderHook(() => useLanguage())
    }).toThrow('useLanguage must be used within a LanguageProvider')

    consoleSpy.mockRestore()
  })

  it('provides default language', () => {
    const wrapper = ({ children }: { children: ReactNode }) => (
      <LanguageProvider>{children}</LanguageProvider>
    )

    const { result } = renderHook(() => useLanguage(), { wrapper })

    expect(result.current.language).toBe('en')
  })

  it('changes language', () => {
    const wrapper = ({ children }: { children: ReactNode }) => (
      <LanguageProvider>{children}</LanguageProvider>
    )

    const { result } = renderHook(() => useLanguage(), { wrapper })

    act(() => {
      result.current.setLanguage('es')
    })

    expect(result.current.language).toBe('es')
    expect(localStorage.getItem('charon-language')).toBe('es')
  })

  it('persists language selection', () => {
    localStorage.setItem('charon-language', 'fr')

    const wrapper = ({ children }: { children: ReactNode }) => (
      <LanguageProvider>{children}</LanguageProvider>
    )

    const { result } = renderHook(() => useLanguage(), { wrapper })

    expect(result.current.language).toBe('fr')
  })

  it('supports all configured languages', () => {
    const wrapper = ({ children }: { children: ReactNode }) => (
      <LanguageProvider>{children}</LanguageProvider>
    )

    const { result } = renderHook(() => useLanguage(), { wrapper })

    const languages = ['en', 'es', 'fr', 'de', 'zh'] as const

    languages.forEach((lang) => {
      act(() => {
        result.current.setLanguage(lang)
      })
      expect(result.current.language).toBe(lang)
    })
  })
})
