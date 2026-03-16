import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import * as websocketApi from '../../api/websocket';
import { WebSocketStatusCard } from '../WebSocketStatusCard';

// Mock the API functions
vi.mock('../../api/websocket');

// Mock date-fns to avoid timezone issues in tests
vi.mock('date-fns', () => ({
  formatDistanceToNow: vi.fn(() => '5 minutes ago'),
}));

describe('WebSocketStatusCard', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });
    vi.clearAllMocks();
  });

  const renderComponent = (props = {}) => {
    return render(
      <QueryClientProvider client={queryClient}>
        <WebSocketStatusCard {...props} />
      </QueryClientProvider>
    );
  };

  it('should render loading state', () => {
    vi.mocked(websocketApi.getWebSocketConnections).mockReturnValue(
      new Promise(() => {}) // Never resolves
    );
    vi.mocked(websocketApi.getWebSocketStats).mockReturnValue(
      new Promise(() => {}) // Never resolves
    );

    renderComponent();

    // Loading state shows skeleton elements
    expect(screen.getAllByRole('generic').length).toBeGreaterThan(0);
  });

  it('should render with no active connections', async () => {
    vi.mocked(websocketApi.getWebSocketConnections).mockResolvedValue({
      connections: [],
      count: 0,
    });
    vi.mocked(websocketApi.getWebSocketStats).mockResolvedValue({
      total_active: 0,
      logs_connections: 0,
      cerberus_connections: 0,
      last_updated: '2024-01-15T10:10:00Z',
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('WebSocket Connections')).toBeInTheDocument();
    });

    expect(screen.getByText('0 Active')).toBeInTheDocument();
    expect(screen.getByText('No active WebSocket connections')).toBeInTheDocument();
  });

  it('should render with active connections', async () => {
    const mockConnections = [
      {
        id: 'conn-1',
        type: 'logs' as const,
        connected_at: '2024-01-15T10:00:00Z',
        last_activity_at: '2024-01-15T10:05:00Z',
        remote_addr: '192.168.1.1:12345',
        filters: 'level=error',
      },
      {
        id: 'conn-2',
        type: 'cerberus' as const,
        connected_at: '2024-01-15T10:02:00Z',
        last_activity_at: '2024-01-15T10:06:00Z',
        remote_addr: '192.168.1.2:54321',
        filters: 'source=waf',
      },
    ];

    vi.mocked(websocketApi.getWebSocketConnections).mockResolvedValue({
      connections: mockConnections,
      count: 2,
    });
    vi.mocked(websocketApi.getWebSocketStats).mockResolvedValue({
      total_active: 2,
      logs_connections: 1,
      cerberus_connections: 1,
      oldest_connection: '2024-01-15T10:00:00Z',
      last_updated: '2024-01-15T10:10:00Z',
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('WebSocket Connections')).toBeInTheDocument();
    });

    expect(screen.getByText('2 Active')).toBeInTheDocument();
    expect(screen.getByText('General Logs')).toBeInTheDocument();
    expect(screen.getByText('Security Logs')).toBeInTheDocument();
    // Use getAllByText since we have two "1" values
    const ones = screen.getAllByText('1');
    expect(ones).toHaveLength(2);
  });

  it('should show details when expanded', async () => {
    const mockConnections = [
      {
        id: 'conn-123',
        type: 'logs' as const,
        connected_at: '2024-01-15T10:00:00Z',
        last_activity_at: '2024-01-15T10:05:00Z',
        remote_addr: '192.168.1.1:12345',
        filters: 'level=error',
      },
    ];

    vi.mocked(websocketApi.getWebSocketConnections).mockResolvedValue({
      connections: mockConnections,
      count: 1,
    });
    vi.mocked(websocketApi.getWebSocketStats).mockResolvedValue({
      total_active: 1,
      logs_connections: 1,
      cerberus_connections: 0,
      last_updated: '2024-01-15T10:10:00Z',
    });

    renderComponent({ showDetails: true });

    await waitFor(() => {
      expect(screen.getByText('WebSocket Connections')).toBeInTheDocument();
    });

    // Check for connection details
    expect(screen.getByText('Active Connections')).toBeInTheDocument();
    expect(screen.getByText(/conn-123/i)).toBeInTheDocument();
    expect(screen.getByText('192.168.1.1:12345')).toBeInTheDocument();
    expect(screen.getByText('level=error')).toBeInTheDocument();
  });

  it('should toggle details on button click', async () => {
    const user = userEvent.setup();
    const mockConnections = [
      {
        id: 'conn-1',
        type: 'logs' as const,
        connected_at: '2024-01-15T10:00:00Z',
        last_activity_at: '2024-01-15T10:05:00Z',
      },
    ];

    vi.mocked(websocketApi.getWebSocketConnections).mockResolvedValue({
      connections: mockConnections,
      count: 1,
    });
    vi.mocked(websocketApi.getWebSocketStats).mockResolvedValue({
      total_active: 1,
      logs_connections: 1,
      cerberus_connections: 0,
      last_updated: '2024-01-15T10:10:00Z',
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('Show Details')).toBeInTheDocument();
    });

    // Initially hidden
    expect(screen.queryByText('Active Connections')).not.toBeInTheDocument();

    // Click to show
    await user.click(screen.getByText('Show Details'));

    await waitFor(() => {
      expect(screen.getByText('Active Connections')).toBeInTheDocument();
    });

    // Click to hide
    await user.click(screen.getByText('Hide Details'));

    await waitFor(() => {
      expect(screen.queryByText('Active Connections')).not.toBeInTheDocument();
    });
  });

  it('should handle API errors gracefully', async () => {
    vi.mocked(websocketApi.getWebSocketConnections).mockRejectedValue(
      new Error('API Error')
    );
    vi.mocked(websocketApi.getWebSocketStats).mockRejectedValue(
      new Error('API Error')
    );

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('Unable to load WebSocket status')).toBeInTheDocument();
    });
  });

  it('should display oldest connection when available', async () => {
    vi.mocked(websocketApi.getWebSocketConnections).mockResolvedValue({
      connections: [],
      count: 1,
    });
    vi.mocked(websocketApi.getWebSocketStats).mockResolvedValue({
      total_active: 1,
      logs_connections: 1,
      cerberus_connections: 0,
      oldest_connection: '2024-01-15T09:55:00Z',
      last_updated: '2024-01-15T10:10:00Z',
    });

    renderComponent();

    await waitFor(() => {
      expect(screen.getByText('Oldest Connection')).toBeInTheDocument();
    });

    expect(screen.getByText('5 minutes ago')).toBeInTheDocument();
  });

  it('should apply custom className', async () => {
    vi.mocked(websocketApi.getWebSocketConnections).mockResolvedValue({
      connections: [],
      count: 0,
    });
    vi.mocked(websocketApi.getWebSocketStats).mockResolvedValue({
      total_active: 0,
      logs_connections: 0,
      cerberus_connections: 0,
      last_updated: '2024-01-15T10:10:00Z',
    });

    const { container } = renderComponent({ className: 'custom-class' });

    await waitFor(() => {
      expect(screen.getByText('WebSocket Connections')).toBeInTheDocument();
    });

    const card = container.querySelector('.custom-class');
    expect(card).toBeInTheDocument();
  });
});
