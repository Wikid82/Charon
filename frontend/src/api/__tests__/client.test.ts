import { beforeEach, describe, it, expect, vi, afterEach } from 'vitest'

type ResponseHandler = (value: unknown) => unknown
type ErrorHandler = (error: ResponseError) => Promise<never>

type ResponseError = {
  response?: {
    status?: number
    data?: Record<string, unknown>
  }
  config?: {
    url?: string
  }
  message?: string
}

// Use vi.hoisted() to declare variables accessible in hoisted mocks
const capturedHandlers = vi.hoisted(() => ({
  onFulfilled: undefined as ResponseHandler | undefined,
  onRejected: undefined as ErrorHandler | undefined,
}))

vi.mock('axios', () => {
  const mockClient = {
    defaults: {
      headers: {
        common: {} as Record<string, string>,
      },
    },
    interceptors: {
      response: {
        use: vi.fn((onFulfilled?: ResponseHandler, onRejected?: ErrorHandler) => {
          capturedHandlers.onFulfilled = onFulfilled
          capturedHandlers.onRejected = onRejected
          return vi.fn()
        }),
      },
    },
  }

  return {
    default: {
      create: vi.fn(() => mockClient),
    },
  }
})

// Must import AFTER mock definition
import { setAuthErrorHandler, setAuthToken } from '../client'
import axios from 'axios'

// Get mock client instance for header assertions
const getMockClient = () => {
  const mockAxios = vi.mocked(axios)
  return mockAxios.create()
}

describe('api client', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('sets and clears the Authorization header', () => {
    const mockClientInstance = getMockClient()

    setAuthToken('test-token')
    expect(mockClientInstance.defaults.headers.common.Authorization).toBe('Bearer test-token')

    setAuthToken(null)
    expect(mockClientInstance.defaults.headers.common.Authorization).toBeUndefined()
  })

  it('extracts error message from response payload', async () => {
    const error: ResponseError = {
      response: { data: { error: 'Bad request' } },
      config: { url: '/test' },
      message: 'Original',
    }

    const handler = capturedHandlers.onRejected
    expect(handler).toBeDefined()

    const resultPromise = handler ? handler(error) : Promise.reject(new Error('handler missing'))

    await expect(resultPromise).rejects.toBe(error)
    expect(error.message).toBe('Bad request')
  })

  it('keeps original message when response payload is not an object', async () => {
    const error: ResponseError = {
      response: { data: 'plain text error' as unknown as Record<string, unknown> },
      config: { url: '/test' },
      message: 'Original',
    }

    const handler = capturedHandlers.onRejected
    expect(handler).toBeDefined()

    const resultPromise = handler ? handler(error) : Promise.reject(new Error('handler missing'))

    await expect(resultPromise).rejects.toBe(error)
    expect(error.message).toBe('Original')
  })

  it('uses error field over message field when both exist', async () => {
    const error: ResponseError = {
      response: { data: { error: 'Preferred error', message: 'Secondary message' } },
      config: { url: '/test' },
      message: 'Original',
    }

    const handler = capturedHandlers.onRejected
    expect(handler).toBeDefined()

    const resultPromise = handler ? handler(error) : Promise.reject(new Error('handler missing'))

    await expect(resultPromise).rejects.toBe(error)
    expect(error.message).toBe('Preferred error')
  })

  it('invokes auth error handler on 401 outside auth endpoints', async () => {
    const onAuthError = vi.fn()
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    setAuthErrorHandler(onAuthError)

    const error: ResponseError = {
      response: { status: 401, data: { message: 'Unauthorized' } },
      config: { url: '/proxy-hosts' },
      message: 'Original',
    }

    const handler = capturedHandlers.onRejected
    expect(handler).toBeDefined()

    const resultPromise = handler ? handler(error) : Promise.reject(new Error('handler missing'))

    await expect(resultPromise).rejects.toBe(error)
    expect(onAuthError).toHaveBeenCalledTimes(1)
    expect(error.message).toBe('Unauthorized')

    warnSpy.mockRestore()
  })

  it('skips auth error handler for auth endpoints', async () => {
    const onAuthError = vi.fn()
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    setAuthErrorHandler(onAuthError)

    const error: ResponseError = {
      response: { status: 401, data: { message: 'Unauthorized' } },
      config: { url: '/auth/login' },
      message: 'Original',
    }

    const handler = capturedHandlers.onRejected
    expect(handler).toBeDefined()

    // Call handler with auth endpoint error to verify it skips the auth error handler
    const resultPromise = handler ? handler(error) : Promise.reject(new Error('handler missing'))

    await expect(resultPromise).rejects.toBe(error)
    expect(onAuthError).not.toHaveBeenCalled()

    warnSpy.mockRestore()
  })

  it('does not invoke auth error handler when status is not 401', async () => {
    const onAuthError = vi.fn()
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

    setAuthErrorHandler(onAuthError)

    const error: ResponseError = {
      response: { status: 403, data: { message: 'Forbidden' } },
      config: { url: '/proxy-hosts' },
      message: 'Original',
    }

    const handler = capturedHandlers.onRejected
    expect(handler).toBeDefined()

    const resultPromise = handler ? handler(error) : Promise.reject(new Error('handler missing'))

    await expect(resultPromise).rejects.toBe(error)
    expect(onAuthError).not.toHaveBeenCalled()
    expect(warnSpy).not.toHaveBeenCalled()

    warnSpy.mockRestore()
  })

  it('passes through successful responses via fulfilled interceptor', () => {
    const responsePayload = { data: { ok: true } }
    const fulfilled = capturedHandlers.onFulfilled

    expect(fulfilled).toBeDefined()
    const result = fulfilled ? fulfilled(responsePayload) : undefined
    expect(result).toBe(responsePayload)
  })
})
