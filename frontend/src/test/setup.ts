// Ensure React's act environment flag is set for React 18+ to avoid warnings
// This must be set before importing testing utilities.
// See: https://github.com/facebook/react/issues/24560#issuecomment-1021997243
declare global { var IS_REACT_ACT_ENVIRONMENT: boolean | undefined }
globalThis.IS_REACT_ACT_ENVIRONMENT = true

import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, vi } from 'vitest'

// Global mock for react-i18next to return English translations in tests
// The import must be done inside the factory since vi.mock is hoisted
vi.mock('react-i18next', async () => {
  // Dynamic import inside the factory ensures translations are loaded
  const enTranslations = await import('../locales/en/translation.json').then(m => m.default)

  // Helper to get nested translation value by dot-notation key
  function getTranslation(key: string): string {
    const keys = key.split('.')
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let result: any = enTranslations
    for (const k of keys) {
      if (result && typeof result === 'object' && k in result) {
        result = result[k]
      } else {
        // Key not found, return the key itself
        return key
      }
    }
    return typeof result === 'string' ? result : key
  }

  return {
    useTranslation: () => ({
      t: (key: string, options?: Record<string, unknown>) => {
        let result = getTranslation(key)
        // Handle interpolation: replace {{variable}} with the value from options
        if (options && typeof result === 'string') {
          Object.entries(options).forEach(([k, v]) => {
            result = result.replace(new RegExp(`\\{\\{${k}\\}\\}`, 'g'), String(v))
          })
        }
        return result
      },
      i18n: {
        changeLanguage: vi.fn(),
        language: 'en',
      },
    }),
    Trans: ({ children }: { children: React.ReactNode }) => children,
    initReactI18next: { type: '3rdParty', init: () => {} },
  }
})

// Cleanup after each test
afterEach(() => {
  cleanup()
})

// Mock window.matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => {},
  }),
})

// Filter noisy React act environment warnings that can appear in some environments
const _origConsoleError = console.error
console.error = (...args: unknown[]) => {
  try {
    const msg = args[0]
    if (typeof msg === 'string') {
      if (
        msg.includes("The current testing environment is not configured to support act(") ||
        msg.includes('Test connection failed') ||
        msg.includes('Connection failed')
      ) {
        return
      }
    }
  } catch {
    // fallthrough to original
  }
  _origConsoleError.apply(console, args)
}
