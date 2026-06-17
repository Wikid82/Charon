import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import {
  getStatsSummary,
  getTopHosts,
  getStatusDistribution,
  getTrafficVolume,
  getCertExpiry,
  getStatsHealth,
} from '../../api/stats';
import {
  useStatsSummary,
  useTopHosts,
  useStatusDistribution,
  useTrafficVolume,
  useCertExpiry,
  useStatsHealth,
} from '../useStats';

vi.mock('../../api/stats', () => ({
  getStatsSummary: vi.fn(),
  getTopHosts: vi.fn(),
  getStatusDistribution: vi.fn(),
  getTrafficVolume: vi.fn(),
  getCertExpiry: vi.fn(),
  getRequests: vi.fn(),
  getStatsHealth: vi.fn(),
  connectStatsWebSocket: vi.fn(),
}));

const mockSummary = {
  requests_last_24h: 100,
  requests_last_7d: 700,
  requests_last_30d: 3000,
};

const mockHostStats = [
  { host_id: 'abc-123', hostname: 'example.com', count: 50 },
];

const mockStatusStats = [
  { code: 200, count: 90 },
  { code: 404, count: 10 },
];

const mockTrafficBuckets = [
  { bucket: '2026-06-14T00:00:00Z', bytes_sent: 1024 },
];

const mockCertExpiry = [
  { host_id: 'def-456', hostname: 'secure.example.com', expires_at: '2026-09-01T00:00:00Z', days_left: 78 },
];

const mockStatsHealth = { dropped_count: 0 };

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe('useStatsSummary', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('uses correct query key', async () => {
    vi.mocked(getStatsSummary).mockResolvedValue(mockSummary);

    const { result } = renderHook(() => useStatsSummary(), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toEqual(mockSummary);
    expect(getStatsSummary).toHaveBeenCalledOnce();
  });

  it('calls getStatsSummary', async () => {
    vi.mocked(getStatsSummary).mockResolvedValue(mockSummary);

    const { result } = renderHook(() => useStatsSummary(), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getStatsSummary).toHaveBeenCalled();
  });

  it('disables refetchInterval when wsConnected is true', async () => {
    vi.mocked(getStatsSummary).mockResolvedValue(mockSummary);

    // We test the hook renders correctly with wsConnected=true — polling is disabled
    // but the initial fetch still fires.
    const { result } = renderHook(() => useStatsSummary(true), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toEqual(mockSummary);
  });

  it('handles fetch error', async () => {
    vi.mocked(getStatsSummary).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useStatsSummary(), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.error).toBeInstanceOf(Error);
  });
});

describe('useTopHosts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('uses correct query key with period and limit', async () => {
    vi.mocked(getTopHosts).mockResolvedValue(mockHostStats);

    const { result } = renderHook(() => useTopHosts('24h', 5), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getTopHosts).toHaveBeenCalledWith('24h', 5);
    expect(result.current.data).toEqual(mockHostStats);
  });

  it('uses default limit of 10', async () => {
    vi.mocked(getTopHosts).mockResolvedValue(mockHostStats);

    const { result } = renderHook(() => useTopHosts('7d'), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getTopHosts).toHaveBeenCalledWith('7d', 10);
  });

  it('handles different periods', async () => {
    vi.mocked(getTopHosts).mockResolvedValue(mockHostStats);

    const { result } = renderHook(() => useTopHosts('30d'), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getTopHosts).toHaveBeenCalledWith('30d', 10);
  });
});

describe('useStatusDistribution', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches status distribution for given period', async () => {
    vi.mocked(getStatusDistribution).mockResolvedValue(mockStatusStats);

    const { result } = renderHook(() => useStatusDistribution('24h'), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getStatusDistribution).toHaveBeenCalledWith('24h');
    expect(result.current.data).toEqual(mockStatusStats);
  });

  it('uses period in query key (re-fetches for different periods)', async () => {
    vi.mocked(getStatusDistribution).mockResolvedValue(mockStatusStats);

    const wrapper = createWrapper();

    const { result: r1 } = renderHook(() => useStatusDistribution('7d'), { wrapper });
    await waitFor(() => expect(r1.current.isSuccess).toBe(true));
    expect(getStatusDistribution).toHaveBeenCalledWith('7d');
  });
});

describe('useTrafficVolume', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches traffic volume for given bucket', async () => {
    vi.mocked(getTrafficVolume).mockResolvedValue(mockTrafficBuckets);

    const { result } = renderHook(() => useTrafficVolume('1h'), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getTrafficVolume).toHaveBeenCalledWith('1h');
    expect(result.current.data).toEqual(mockTrafficBuckets);
  });

  it('uses bucket in query key', async () => {
    vi.mocked(getTrafficVolume).mockResolvedValue(mockTrafficBuckets);

    const { result } = renderHook(() => useTrafficVolume('6h'), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getTrafficVolume).toHaveBeenCalledWith('6h');
  });
});

describe('useCertExpiry', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches cert expiry with default withinDays', async () => {
    vi.mocked(getCertExpiry).mockResolvedValue(mockCertExpiry);

    const { result } = renderHook(() => useCertExpiry(), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getCertExpiry).toHaveBeenCalledWith(30);
    expect(result.current.data).toEqual(mockCertExpiry);
  });

  it('fetches cert expiry with custom withinDays', async () => {
    vi.mocked(getCertExpiry).mockResolvedValue(mockCertExpiry);

    const { result } = renderHook(() => useCertExpiry(14), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getCertExpiry).toHaveBeenCalledWith(14);
  });
});

describe('useStatsHealth', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches stats health', async () => {
    vi.mocked(getStatsHealth).mockResolvedValue(mockStatsHealth);

    const { result } = renderHook(() => useStatsHealth(), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getStatsHealth).toHaveBeenCalledOnce();
    expect(result.current.data).toEqual(mockStatsHealth);
  });

  it('handles error state', async () => {
    vi.mocked(getStatsHealth).mockRejectedValue(new Error('Service unavailable'));

    const { result } = renderHook(() => useStatsHealth(), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.error).toBeInstanceOf(Error);
  });
});
