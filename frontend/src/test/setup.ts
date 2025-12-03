// Ensure React's act environment flag is set for React 18+ to avoid warnings
// This must be set before importing testing utilities.
// See: https://github.com/facebook/react/issues/24560#issuecomment-1021997243
declare global { var IS_REACT_ACT_ENVIRONMENT: boolean | undefined }
globalThis.IS_REACT_ACT_ENVIRONMENT = true

import '@testing-library/jest-dom'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

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
