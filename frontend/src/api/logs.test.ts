import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import client from './client'
import { getLogs, getLogContent, downloadLog, connectLiveLogs, connectSecurityLogs, type LiveLogEntry, type SecurityLogEntry  } from './logs'


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

describe('connectSecurityLogs', () => {
  it('connects to cerberus logs websocket endpoint', () => {
    const received: SecurityLogEntry[] = []
    const onOpen = vi.fn()

    connectSecurityLogs({}, (log) => received.push(log), onOpen)

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    expect(socket.url).toContain('/api/v1/cerberus/logs/ws')
  })

  it('passes source filter to websocket url', () => {
    connectSecurityLogs({ source: 'waf' }, () => {})

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    expect(socket.url).toContain('source=waf')
  })

  it('passes level filter to websocket url', () => {
    connectSecurityLogs({ level: 'error' }, () => {})

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    expect(socket.url).toContain('level=error')
  })

  it('passes ip filter to websocket url', () => {
    connectSecurityLogs({ ip: '192.168' }, () => {})

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    expect(socket.url).toContain('ip=192.168')
  })

  it('passes host filter to websocket url', () => {
    connectSecurityLogs({ host: 'example.com' }, () => {})

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    expect(socket.url).toContain('host=example.com')
  })

  it('passes blocked_only filter to websocket url', () => {
    connectSecurityLogs({ blocked_only: true }, () => {})

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    expect(socket.url).toContain('blocked_only=true')
  })

  it('receives and parses security log entries', () => {
    const received: SecurityLogEntry[] = []
    connectSecurityLogs({}, (log) => received.push(log))

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    socket.open()

    const securityLogEntry: SecurityLogEntry = {
      timestamp: '2025-12-12T10:30:00Z',
      level: 'info',
      logger: 'http.log.access',
      client_ip: '192.168.1.100',
      method: 'GET',
      uri: '/api/test',
      status: 200,
      duration: 0.05,
      size: 1024,
      user_agent: 'TestAgent/1.0',
      host: 'example.com',
      source: 'normal',
      blocked: false,
    }

    socket.sendMessage(JSON.stringify(securityLogEntry))

    expect(received).toHaveLength(1)
    expect(received[0].client_ip).toBe('192.168.1.100')
    expect(received[0].source).toBe('normal')
    expect(received[0].blocked).toBe(false)
  })

  it('receives blocked security log entries', () => {
    const received: SecurityLogEntry[] = []
    connectSecurityLogs({}, (log) => received.push(log))

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    socket.open()

    const blockedEntry: SecurityLogEntry = {
      timestamp: '2025-12-12T10:30:00Z',
      level: 'warn',
      logger: 'http.handlers.waf',
      client_ip: '10.0.0.1',
      method: 'POST',
      uri: '/admin',
      status: 403,
      duration: 0.001,
      size: 0,
      user_agent: 'Attack/1.0',
      host: 'example.com',
      source: 'waf',
      blocked: true,
      block_reason: 'SQL injection detected',
    }

    socket.sendMessage(JSON.stringify(blockedEntry))

    expect(received).toHaveLength(1)
    expect(received[0].blocked).toBe(true)
    expect(received[0].block_reason).toBe('SQL injection detected')
    expect(received[0].source).toBe('waf')
  })

  it('handles onOpen callback', () => {
    const onOpen = vi.fn()
    connectSecurityLogs({}, () => {}, onOpen)

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    socket.open()

    expect(onOpen).toHaveBeenCalled()
  })

  it('handles onError callback', () => {
    const onError = vi.fn()
    connectSecurityLogs({}, () => {}, undefined, onError)

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    const errorEvent = new Event('error')
    socket.triggerError(errorEvent)

    expect(onError).toHaveBeenCalledWith(errorEvent)
  })

  it('handles onClose callback', () => {
    const onClose = vi.fn()
    connectSecurityLogs({}, () => {}, undefined, undefined, onClose)

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    socket.close()

    expect(onClose).toHaveBeenCalled()
  })

  it('returns disconnect function that closes websocket', () => {
    const disconnect = connectSecurityLogs({}, () => {})

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    socket.open()

    expect(socket.readyState).toBe(MockWebSocket.OPEN)

    disconnect()

    expect(socket.readyState).toBe(MockWebSocket.CLOSED)
  })

  it('handles JSON parse errors gracefully', () => {
    const received: SecurityLogEntry[] = []
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})

    connectSecurityLogs({}, (log) => received.push(log))

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    socket.open()
    socket.sendMessage('invalid-json')

    expect(received).toHaveLength(0)
    expect(consoleError).toHaveBeenCalled()

    consoleError.mockRestore()
  })

  it('uses wss protocol when on https', () => {
    Object.defineProperty(window, 'location', {
      value: { protocol: 'https:', host: 'secure.example.com', href: '' },
      writable: true,
    })

    connectSecurityLogs({}, () => {})

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    expect(socket.url).toContain('wss://')
    expect(socket.url).toContain('secure.example.com')
  })

  it('combines multiple filters in websocket url', () => {
    connectSecurityLogs(
      {
        source: 'waf',
        level: 'warn',
        ip: '10.0.0',
        host: 'example.com',
        blocked_only: true,
      },
      () => {}
    )

    const socket = MockWebSocket.instances[MockWebSocket.instances.length - 1]!
    expect(socket.url).toContain('source=waf')
    expect(socket.url).toContain('level=warn')
    expect(socket.url).toContain('ip=10.0.0')
    expect(socket.url).toContain('host=example.com')
    expect(socket.url).toContain('blocked_only=true')
  })
})
