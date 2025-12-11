import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import client from './client'
import { getLogs, getLogContent, downloadLog, connectLiveLogs } from './logs'
import type { LiveLogEntry } from './logs'

vi.mock('./client', () => ({
  default: {
    get: vi.fn(),
  },
}))

const mockedClient = client as unknown as {
  get: ReturnType<typeof vi.fn>
}

class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSED = 3
  static instances: MockWebSocket[] = []

  url: string
  readyState = MockWebSocket.CONNECTING
  onopen: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  open() {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.()
  }

  sendMessage(data: string) {
    this.onmessage?.({ data })
  }

  triggerError(event: Event) {
    this.onerror?.(event)
  }

  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.({ code: 1000, reason: '', wasClean: true } as CloseEvent)
  }
}

const originalWebSocket = globalThis.WebSocket
const originalLocation = { ...window.location }

beforeEach(() => {
  vi.clearAllMocks()
  ;(globalThis as unknown as { WebSocket: typeof WebSocket }).WebSocket = MockWebSocket as unknown as typeof WebSocket
  Object.defineProperty(window, 'location', {
    value: { ...originalLocation, protocol: 'http:', host: 'localhost', href: '' },
    writable: true,
  })
})

afterEach(() => {
  ;(globalThis as unknown as { WebSocket: typeof WebSocket }).WebSocket = originalWebSocket
  Object.defineProperty(window, 'location', { value: originalLocation })
  MockWebSocket.instances.length = 0
})

describe('logs api', () => {
  it('lists log files', async () => {
    mockedClient.get.mockResolvedValue({ data: [{ name: 'access.log', size: 10, mod_time: 'now' }] })

    const logs = await getLogs()

    expect(mockedClient.get).toHaveBeenCalledWith('/logs')
    expect(logs[0].name).toBe('access.log')
  })

  it('fetches log content with filters applied', async () => {
    mockedClient.get.mockResolvedValue({ data: { filename: 'access.log', logs: [], total: 0, limit: 50, offset: 0 } })

    await getLogContent('access.log', {
      search: 'error',
      host: 'example.com',
      status: '500',
      level: 'error',
      limit: 50,
      offset: 10,
      sort: 'asc',
    })

    expect(mockedClient.get).toHaveBeenCalledWith(
      '/logs/access.log?search=error&host=example.com&status=500&level=error&limit=50&offset=10&sort=asc'
    )
  })

  it('sets window location when downloading logs', () => {
    downloadLog('access.log')
    expect(window.location.href).toBe('/api/v1/logs/access.log/download')
  })

  it('connects to live logs websocket and handles lifecycle events', () => {
    const received: LiveLogEntry[] = []
    const onOpen = vi.fn()
    const onError = vi.fn()
    const onClose = vi.fn()

    const disconnect = connectLiveLogs({ level: 'error', source: 'cerberus' }, (log) => received.push(log), onOpen, onError, onClose)

  const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    expect(socket.url).toContain('level=error')
    expect(socket.url).toContain('source=cerberus')

    socket.open()
    expect(onOpen).toHaveBeenCalled()

    socket.sendMessage(JSON.stringify({ level: 'info', timestamp: 'now', message: 'hello' }))
    expect(received).toHaveLength(1)

    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    socket.sendMessage('not-json')
    expect(consoleError).toHaveBeenCalled()
    consoleError.mockRestore()

    const errorEvent = new Event('error')
    socket.triggerError(errorEvent)
    expect(onError).toHaveBeenCalledWith(errorEvent)

    socket.close()
    expect(onClose).toHaveBeenCalled()

    disconnect()
  })
})
