/// <reference types="vitest/globals" />
// eslint-disable-next-line @typescript-eslint/triple-slash-reference
/// <reference types="@testing-library/jest-dom/vitest" />

// Import jest-dom matchers at runtime to extend expect()
import '@testing-library/jest-dom/vitest'

// Ensure React's act environment flag is set for React 18+ to avoid warnings
// This must be set before importing testing utilities.
// See: https://github.com/facebook/react/issues/24560#issuecomment-1021997243
declare global { var IS_REACT_ACT_ENVIRONMENT: boolean | undefined }
globalThis.IS_REACT_ACT_ENVIRONMENT = true

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
    let result: unknown = enTranslations
    for (const k of keys) {
      if (result && typeof result === 'object' && k in (result as Record<string, unknown>)) {
        result = (result as Record<string, unknown>)[k]
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
  vi.clearAllTimers()
  vi.useRealTimers()
  vi.clearAllMocks()
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

// Add ResizeObserver mock (required by Radix UI)
global.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

// Add pointer capture polyfills (required by Radix UI)
if (!HTMLElement.prototype.hasPointerCapture) {
  HTMLElement.prototype.hasPointerCapture = function() { return false }
  HTMLElement.prototype.setPointerCapture = function() {}
  HTMLElement.prototype.releasePointerCapture = function() {}
}

// Add scrollIntoView mock (required by Radix Select)
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = function() {}
}

// Prevent jsdom navigation errors for anchor clicks during tests
const anchorPrototype = HTMLAnchorElement.prototype as unknown as {
  __testNoNavClick?: boolean
  __originalClick?: typeof HTMLAnchorElement.prototype.click
}

if (!anchorPrototype.__testNoNavClick) {
  const originalClick = HTMLAnchorElement.prototype.click
  Object.defineProperty(HTMLAnchorElement.prototype, '__testNoNavClick', {
    value: true,
    configurable: false,
    writable: false,
  })
  HTMLAnchorElement.prototype.click = function() {
    const event = new MouseEvent('click', { bubbles: true, cancelable: true })
    this.dispatchEvent(event)
    return undefined
  }
  anchorPrototype.__originalClick = originalClick
}

// Filter noisy React act environment warnings that can appear in some environments
const _origConsoleError = console.error
console.error = (...args: unknown[]) => {
  try {
    const msg = args[0]
    if (typeof msg === 'string') {
      if (
        msg.includes("The current testing environment is not configured to support act(") ||
        msg.includes('not wrapped in act(') ||
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
