import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { LiveLogViewer } from '../LiveLogViewer';
import * as logsApi from '../../api/logs';

// Mock the connectLiveLogs function
vi.mock('../../api/logs', async () => {
  const actual = await vi.importActual('../../api/logs');
  return {
    ...actual,
    connectLiveLogs: vi.fn(),
  };
});

describe('LiveLogViewer', () => {
  let mockCloseConnection: ReturnType<typeof vi.fn>;
  let mockOnMessage: ((log: logsApi.LiveLogEntry) => void) | null;
  let mockOnClose: (() => void) | null;

  beforeEach(() => {
    mockCloseConnection = vi.fn();
    mockOnMessage = null;
    mockOnClose = null;

    vi.mocked(logsApi.connectLiveLogs).mockImplementation((_filters, onMessage, onOpen, _onError, onClose) => {
      mockOnMessage = onMessage;
      mockOnClose = onClose ?? null;
      // Simulate connection success
      if (onOpen) {
        setTimeout(() => onOpen(), 0);
      }
      return mockCloseConnection as () => void;
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders the component with initial state', async () => {
    render(<LiveLogViewer />);

    expect(screen.getByText('Live Security Logs')).toBeTruthy();
    // Initially disconnected until WebSocket opens
    expect(screen.getByText('Disconnected')).toBeTruthy();

    // Wait for onOpen callback to be called
    await waitFor(() => {
      expect(screen.getByText('Connected')).toBeTruthy();
    });

    expect(screen.getByText('No logs yet. Waiting for events...')).toBeTruthy();
  });

  it('displays incoming log messages', async () => {
    render(<LiveLogViewer />);

    // Simulate receiving a log
    const logEntry: logsApi.LiveLogEntry = {
      level: 'info',
      timestamp: '2025-12-09T10:30:00Z',
      message: 'Test log message',
      source: 'test',
    };

    if (mockOnMessage) {
      mockOnMessage(logEntry);
    }

    await waitFor(() => {
      expect(screen.getByText('Test log message')).toBeTruthy();
      expect(screen.getByText('INFO')).toBeTruthy();
      expect(screen.getByText('[test]')).toBeTruthy();
    });
  });

  it('filters logs by text', async () => {
    const user = userEvent.setup();
    render(<LiveLogViewer />);

    // Add multiple logs
    if (mockOnMessage) {
      mockOnMessage({ level: 'info', timestamp: '2025-12-09T10:30:00Z', message: 'First message' });
      mockOnMessage({ level: 'error', timestamp: '2025-12-09T10:30:01Z', message: 'Second message' });
    }

    await waitFor(() => {
      expect(screen.getByText('First message')).toBeTruthy();
      expect(screen.getByText('Second message')).toBeTruthy();
    });

    // Apply text filter
    const filterInput = screen.getByPlaceholderText('Filter by text...');
    await user.type(filterInput, 'First');

    await waitFor(() => {
      expect(screen.getByText('First message')).toBeTruthy();
      expect(screen.queryByText('Second message')).toBeFalsy();
    });
  });

  it('filters logs by level', async () => {
    const user = userEvent.setup();
    render(<LiveLogViewer />);

    // Add multiple logs
    if (mockOnMessage) {
      mockOnMessage({ level: 'info', timestamp: '2025-12-09T10:30:00Z', message: 'Info message' });
      mockOnMessage({ level: 'error', timestamp: '2025-12-09T10:30:01Z', message: 'Error message' });
    }

    await waitFor(() => {
      expect(screen.getByText('Info message')).toBeTruthy();
      expect(screen.getByText('Error message')).toBeTruthy();
    });

    // Apply level filter
    const levelSelect = screen.getByRole('combobox');
    await user.selectOptions(levelSelect, 'error');

    await waitFor(() => {
      expect(screen.queryByText('Info message')).toBeFalsy();
      expect(screen.getByText('Error message')).toBeTruthy();
    });
  });

  it('pauses and resumes log streaming', async () => {
    const user = userEvent.setup();
    render(<LiveLogViewer />);

    // Add initial log
    if (mockOnMessage) {
      mockOnMessage({ level: 'info', timestamp: '2025-12-09T10:30:00Z', message: 'Before pause' });
    }

    await waitFor(() => {
      expect(screen.getByText('Before pause')).toBeTruthy();
    });

    // Click pause button
    const pauseButton = screen.getByTitle('Pause');
    await user.click(pauseButton);

    // Verify paused state
    await waitFor(() => {
      expect(screen.getByText('⏸ Paused')).toBeTruthy();
    });

    // Try to add log while paused (should not appear)
    if (mockOnMessage) {
      mockOnMessage({ level: 'info', timestamp: '2025-12-09T10:30:01Z', message: 'During pause' });
    }

    // Log should not appear
    expect(screen.queryByText('During pause')).toBeFalsy();

    // Resume
    const resumeButton = screen.getByTitle('Resume');
    await user.click(resumeButton);

    // Add log after resume
    if (mockOnMessage) {
      mockOnMessage({ level: 'info', timestamp: '2025-12-09T10:30:02Z', message: 'After resume' });
    }

    await waitFor(() => {
      expect(screen.getByText('After resume')).toBeTruthy();
    });
  });

  it('clears all logs', async () => {
    const user = userEvent.setup();
    render(<LiveLogViewer />);

    // Add logs
    if (mockOnMessage) {
      mockOnMessage({ level: 'info', timestamp: '2025-12-09T10:30:00Z', message: 'Log 1' });
      mockOnMessage({ level: 'info', timestamp: '2025-12-09T10:30:01Z', message: 'Log 2' });
    }

    await waitFor(() => {
      expect(screen.getByText('Log 1')).toBeTruthy();
      expect(screen.getByText('Log 2')).toBeTruthy();
    });

    // Click clear button
    const clearButton = screen.getByTitle('Clear logs');
    await user.click(clearButton);

    await waitFor(() => {
      expect(screen.queryByText('Log 1')).toBeFalsy();
      expect(screen.queryByText('Log 2')).toBeFalsy();
      expect(screen.getByText('No logs yet. Waiting for events...')).toBeTruthy();
    });
  });

  it('limits the number of stored logs', async () => {
    render(<LiveLogViewer maxLogs={2} />);

    // Add 3 logs (exceeding maxLogs)
    if (mockOnMessage) {
      mockOnMessage({ level: 'info', timestamp: '2025-12-09T10:30:00Z', message: 'Log 1' });
      mockOnMessage({ level: 'info', timestamp: '2025-12-09T10:30:01Z', message: 'Log 2' });
      mockOnMessage({ level: 'info', timestamp: '2025-12-09T10:30:02Z', message: 'Log 3' });
    }

    await waitFor(() => {
      // First log should be removed, only last 2 should remain
      expect(screen.queryByText('Log 1')).toBeFalsy();
      expect(screen.getByText('Log 2')).toBeTruthy();
      expect(screen.getByText('Log 3')).toBeTruthy();
    });
  });

  it('displays log data when available', async () => {
    render(<LiveLogViewer />);

    const logWithData: logsApi.LiveLogEntry = {
      level: 'error',
      timestamp: '2025-12-09T10:30:00Z',
      message: 'Error occurred',
      data: { error_code: 500, details: 'Internal server error' },
    };

    if (mockOnMessage) {
      mockOnMessage(logWithData);
    }

    await waitFor(() => {
      expect(screen.getByText('Error occurred')).toBeTruthy();
      // Check that data is rendered as JSON
      expect(screen.getByText(/"error_code"/)).toBeTruthy();
    });
  });

  it('closes WebSocket connection on unmount', () => {
    const { unmount } = render(<LiveLogViewer />);

    expect(logsApi.connectLiveLogs).toHaveBeenCalled();

    unmount();

    expect(mockCloseConnection).toHaveBeenCalled();
  });

  it('applies custom className', () => {
    const { container } = render(<LiveLogViewer className="custom-class" />);

    const element = container.querySelector('.custom-class');
    expect(element).toBeTruthy();
  });

  it('shows correct connection status', async () => {
    let mockOnOpen: (() => void) | undefined;
    let mockOnError: ((error: Event) => void) | undefined;

    vi.mocked(logsApi.connectLiveLogs).mockImplementation((_filters, _onMessage, onOpen, onError) => {
      mockOnOpen = onOpen;
      mockOnError = onError;
      return mockCloseConnection as () => void;
    });

    render(<LiveLogViewer />);

    // Initially disconnected until onOpen is called
    expect(screen.getByText('Disconnected')).toBeTruthy();

    // Simulate connection opened
    if (mockOnOpen) {
      mockOnOpen();
    }

    await waitFor(() => {
      expect(screen.getByText('Connected')).toBeTruthy();
    });

    // Simulate connection error
    if (mockOnError) {
      mockOnError(new Event('error'));
    }

    await waitFor(() => {
      expect(screen.getByText('Disconnected')).toBeTruthy();
    });
  });

  it('shows no-match message when filters exclude all logs', async () => {
    const user = userEvent.setup();
    render(<LiveLogViewer />);

    if (mockOnMessage) {
      mockOnMessage({ level: 'info', timestamp: '2025-12-09T10:30:00Z', message: 'Visible' });
      mockOnMessage({ level: 'error', timestamp: '2025-12-09T10:30:01Z', message: 'Hidden' });
    }

    await waitFor(() => expect(screen.getByText('Visible')).toBeTruthy());

    await user.type(screen.getByPlaceholderText('Filter by text...'), 'nomatch');

    await waitFor(() => {
      expect(screen.getByText('No logs match the current filters.')).toBeTruthy();
    });
  });

  it('marks connection as disconnected when WebSocket closes', async () => {
    render(<LiveLogViewer />);

    await waitFor(() => expect(screen.getByText('Connected')).toBeTruthy());

    mockOnClose?.();

    await waitFor(() => expect(screen.getByText('Disconnected')).toBeTruthy());
  });
});
