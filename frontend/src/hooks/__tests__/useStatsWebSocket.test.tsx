import { renderHook, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import { useStatsWebSocket } from '../useStatsWebSocket';

// Minimal mock WebSocket implementation
interface MockWsInstance {
  onopen: (() => void) | null;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  close: ReturnType<typeof vi.fn>;
  readyState: number;
}

let mockWsInstance: MockWsInstance;

function setupMockWebSocket(initialReadyState = WebSocket.CONNECTING) {
  mockWsInstance = {
    onopen: null,
    onmessage: null,
    onerror: null,
    onclose: null,
    close: vi.fn(),
    readyState: initialReadyState,
  };

  const MockWebSocket = vi.fn().mockImplementation(function () {
    return mockWsInstance;
  }) as ReturnType<typeof vi.fn> & { OPEN: number; CONNECTING: number; CLOSED: number };

  MockWebSocket.OPEN = WebSocket.OPEN;
  MockWebSocket.CONNECTING = WebSocket.CONNECTING;
  MockWebSocket.CLOSED = WebSocket.CLOSED;

  vi.stubGlobal('WebSocket', MockWebSocket);
}

describe('useStatsWebSocket', () => {
  beforeEach(() => {
    vi.stubGlobal('location', { protocol: 'http:', host: 'localhost:3000' });
    setupMockWebSocket();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it('starts with connected=false, lastMessage=null, latestSummary=null', () => {
    const { result } = renderHook(() => useStatsWebSocket());

    expect(result.current.connected).toBe(false);
    expect(result.current.lastMessage).toBeNull();
    expect(result.current.latestSummary).toBeNull();
  });

  it('sets connected=true when WebSocket opens', () => {
    const { result } = renderHook(() => useStatsWebSocket());

    act(() => {
      mockWsInstance.onopen?.();
    });

    expect(result.current.connected).toBe(true);
  });

  it('sets connected=false when WebSocket closes', () => {
    const { result } = renderHook(() => useStatsWebSocket());

    act(() => {
      mockWsInstance.onopen?.();
    });
    expect(result.current.connected).toBe(true);

    act(() => {
      const closeEvent = new CloseEvent('close', { code: 1000, wasClean: true });
      mockWsInstance.onclose?.(closeEvent);
    });

    expect(result.current.connected).toBe(false);
  });

  it('updates lastMessage and latestSummary on stats_update message', () => {
    const { result } = renderHook(() => useStatsWebSocket());

    const summary = {
      requests_last_24h: 42,
      requests_last_7d: 294,
      requests_last_30d: 1260,
    };

    act(() => {
      const event = {
        data: JSON.stringify({ type: 'stats_update', data: summary }),
      } as MessageEvent;
      mockWsInstance.onmessage?.(event);
    });

    expect(result.current.lastMessage).toEqual({ type: 'stats_update', data: summary });
    expect(result.current.latestSummary).toEqual(summary);
  });

  it('updates lastMessage on each new message', () => {
    const { result } = renderHook(() => useStatsWebSocket());

    const summary1 = { requests_last_24h: 1, requests_last_7d: 7, requests_last_30d: 30 };
    const summary2 = { requests_last_24h: 2, requests_last_7d: 14, requests_last_30d: 60 };

    act(() => {
      mockWsInstance.onmessage?.({ data: JSON.stringify({ type: 'stats_update', data: summary1 }) } as MessageEvent);
    });
    expect(result.current.latestSummary).toEqual(summary1);

    act(() => {
      mockWsInstance.onmessage?.({ data: JSON.stringify({ type: 'stats_update', data: summary2 }) } as MessageEvent);
    });
    expect(result.current.latestSummary).toEqual(summary2);
    expect(result.current.lastMessage).toEqual({ type: 'stats_update', data: summary2 });
  });

  it('calls the cleanup function returned by connectStatsWebSocket on unmount', () => {
    mockWsInstance.readyState = WebSocket.OPEN;

    const { unmount } = renderHook(() => useStatsWebSocket());

    unmount();

    // The cleanup closure calls ws.close() when readyState is OPEN
    expect(mockWsInstance.close).toHaveBeenCalledOnce();
  });

  it('does not call ws.close() on unmount when connection is already closed', () => {
    mockWsInstance.readyState = WebSocket.CLOSED;

    const { unmount } = renderHook(() => useStatsWebSocket());

    unmount();

    expect(mockWsInstance.close).not.toHaveBeenCalled();
  });
});
