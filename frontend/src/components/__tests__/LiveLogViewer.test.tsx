import { render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import * as logsApi from '../../api/logs';
import { LiveLogViewer } from '../LiveLogViewer';

// Mock the connectLiveLogs and connectSecurityLogs functions
vi.mock('../../api/logs', async () => {
  const actual = await vi.importActual('../../api/logs');
  return {
    ...actual,
    connectLiveLogs: vi.fn(),
    connectSecurityLogs: vi.fn(),
  };
});

describe('LiveLogViewer', () => {
  let mockCloseConnection: ReturnType<typeof vi.fn>;
  let mockOnMessage: ((log: logsApi.LiveLogEntry) => void) | null;
  let mockOnSecurityMessage: ((log: logsApi.SecurityLogEntry) => void) | null;
  let mockOnClose: (() => void) | null;

  beforeEach(() => {
    mockCloseConnection = vi.fn();
    mockOnMessage = null;
    mockOnSecurityMessage = null;
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

    vi.mocked(logsApi.connectSecurityLogs).mockImplementation((_filters, onMessage, onOpen, _onError, onClose) => {
      mockOnSecurityMessage = onMessage;
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

    // Default mode is now 'security'
    expect(screen.getByText('Security Access Logs')).toBeTruthy();
    // Initially disconnected until WebSocket opens
    expect(screen.getByText('Disconnected')).toBeTruthy();

    // Wait for onOpen callback to be called
    await waitFor(() => {
      expect(screen.getByText('Connected')).toBeTruthy();
    });

    expect(screen.getByText('No logs yet. Waiting for events...')).toBeTruthy();
  });

  it('displays incoming log messages', async () => {
    // Explicitly use application mode for this test
    render(<LiveLogViewer mode="application" />);

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
    // Explicitly use application mode for this test
    render(<LiveLogViewer mode="application" />);

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
    // Explicitly use application mode for this test
    render(<LiveLogViewer mode="application" />);

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
    const levelSelect = screen.getAllByRole('combobox')[0];
    await user.selectOptions(levelSelect, 'error');

    await waitFor(() => {
      expect(screen.queryByText('Info message')).toBeFalsy();
      expect(screen.getByText('Error message')).toBeTruthy();
    });
  });

  it('pauses and resumes log streaming', async () => {
    const user = userEvent.setup();
    // Explicitly use application mode for this test
    render(<LiveLogViewer mode="application" />);

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
    // Explicitly use application mode for this test
    render(<LiveLogViewer mode="application" />);

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
    // Explicitly use application mode for this test
    render(<LiveLogViewer maxLogs={2} mode="application" />);

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
    // Explicitly use application mode for this test
    render(<LiveLogViewer mode="application" />);

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

    // Default mode is security
    expect(logsApi.connectSecurityLogs).toHaveBeenCalled();

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

    // Use security logs mock since default mode is security
    vi.mocked(logsApi.connectSecurityLogs).mockImplementation((_filters, _onMessage, onOpen, onError) => {
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
      // Should show error message
      expect(screen.getByText('Failed to connect to log stream. Check your authentication or try refreshing.')).toBeTruthy();
    });
  });

  it('shows no-match message when filters exclude all logs', async () => {
    const user = userEvent.setup();
    // Explicitly use application mode for this test
    render(<LiveLogViewer mode="application" />);

    if (mockOnMessage) {
      mockOnMessage({ level: 'info', timestamp: '2025-12-09T10:30:00Z', message: 'Visible' });
      mockOnMessage({ level: 'error', timestamp: '2025-12-09T10:30:01Z', message: 'Hidden' });
    }

    expect(await screen.findByText('Visible')).toBeTruthy();

    await user.type(screen.getByPlaceholderText('Filter by text...'), 'nomatch');

    await waitFor(() => {
      expect(screen.getByText('No logs match the current filters.')).toBeTruthy();
    });
  });

  it('marks connection as disconnected when WebSocket closes', async () => {
    render(<LiveLogViewer />);

    expect(await screen.findByText('Connected')).toBeTruthy();

    act(() => {
      mockOnClose?.();
    });

    expect(await screen.findByText('Disconnected')).toBeTruthy();
  });

  // ============================================================
  // Security Mode Tests
  // ============================================================

  describe('Security Mode', () => {
    it('renders in security mode when mode="security"', async () => {
      render(<LiveLogViewer mode="security" />);

      expect(screen.getByText('Security Access Logs')).toBeTruthy();
      expect(logsApi.connectSecurityLogs).toHaveBeenCalled();
    });

    it('displays security log entries with source badges', async () => {
      render(<LiveLogViewer mode="security" />);

      // Wait for connection to establish
      expect(await screen.findByText('Connected')).toBeTruthy();

      const securityLog: logsApi.SecurityLogEntry = {
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
      };

      if (mockOnSecurityMessage) {
        mockOnSecurityMessage(securityLog);
      }

      await waitFor(() => {
        expect(screen.getByText('NORMAL')).toBeTruthy();
        expect(screen.getByText('192.168.1.100')).toBeTruthy();
        expect(screen.getByText(/GET \/api\/test → 200/)).toBeTruthy();
      });
    });

    it('displays blocked requests with special styling', async () => {
      render(<LiveLogViewer mode="security" />);

      // Wait for connection to establish
      expect(await screen.findByText('Connected')).toBeTruthy();

      const blockedLog: logsApi.SecurityLogEntry = {
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
      };

      // Send message inside act to properly handle state updates
      await act(async () => {
        if (mockOnSecurityMessage) {
          mockOnSecurityMessage(blockedLog);
        }
      });

      // Use findBy queries (built-in waiting) instead of single waitFor with multiple assertions
      // This avoids race conditions where one failing assertion causes the entire block to retry
      await screen.findByText('10.0.0.1');
      await screen.findByText(/🚫 BLOCKED: SQL injection detected/);
      await screen.findByText(/\[SQL injection detected\]/);

      // For getAllByText, keep in waitFor but separate from other assertions
      await waitFor(() => {
        // Use getAllByText since 'WAF' appears both in dropdown option and source badge
        const wafElements = screen.getAllByText('WAF');
        expect(wafElements.length).toBeGreaterThanOrEqual(2); // Option + badge
      });
    }, 15000); // 15 second timeout as safeguard

    it('shows source filter dropdown in security mode', async () => {
      render(<LiveLogViewer mode="security" />);

      // Should have source filter options
      expect(screen.getByText('All Sources')).toBeTruthy();
      expect(screen.getByRole('option', { name: 'WAF' })).toBeTruthy();
      expect(screen.getByRole('option', { name: 'CrowdSec' })).toBeTruthy();
      expect(screen.getByRole('option', { name: 'Rate Limit' })).toBeTruthy();
      expect(screen.getByRole('option', { name: 'ACL' })).toBeTruthy();
    });

    it('filters by source in security mode', async () => {
      const user = userEvent.setup();
      render(<LiveLogViewer mode="security" />);

      // Wait for connection
      expect(await screen.findByText('Connected')).toBeTruthy();

      // Add logs from different sources
      if (mockOnSecurityMessage) {
        mockOnSecurityMessage({
          timestamp: '2025-12-12T10:30:00Z',
          level: 'info',
          logger: 'http.log.access',
          client_ip: '192.168.1.1',
          method: 'GET',
          uri: '/normal-request',
          status: 200,
          duration: 0.01,
          size: 100,
          user_agent: 'Test/1.0',
          host: 'example.com',
          source: 'normal',
          blocked: false,
        });
        mockOnSecurityMessage({
          timestamp: '2025-12-12T10:30:01Z',
          level: 'warn',
          logger: 'http.handlers.waf',
          client_ip: '10.0.0.1',
          method: 'POST',
          uri: '/waf-blocked',
          status: 403,
          duration: 0.001,
          size: 0,
          user_agent: 'Attack/1.0',
          host: 'example.com',
          source: 'waf',
          blocked: true,
          block_reason: 'WAF block',
        });
      }

      // Wait for logs to appear - normal shows URI, blocked shows block message
      await waitFor(() => {
        expect(screen.getByText(/GET \/normal-request/)).toBeTruthy();
        expect(screen.getByText(/BLOCKED: WAF block/)).toBeTruthy();
      });

      // Filter by WAF using the source dropdown (second combobox after level)
      const sourceSelects = screen.getAllByRole('combobox');
      const sourceFilterSelect = sourceSelects[1]; // Second combobox is source filter

      await user.selectOptions(sourceFilterSelect, 'waf');

      await waitFor(() => {
        expect(screen.queryByText(/GET \/normal-request/)).toBeFalsy();
        expect(screen.getByText(/BLOCKED: WAF block/)).toBeTruthy();
      });
    });

    it('shows blocked only checkbox in security mode', async () => {
      render(<LiveLogViewer mode="security" />);

      expect(screen.getByText('Blocked only')).toBeTruthy();
      expect(screen.getByRole('checkbox')).toBeTruthy();
    });

    it('toggles blocked only filter', async () => {
      const user = userEvent.setup();
      render(<LiveLogViewer mode="security" />);

      const checkbox = screen.getByRole('checkbox');
      await user.click(checkbox);

      // Verify checkbox is checked
      expect(checkbox).toBeChecked();
    });

    it('displays duration for security logs', async () => {
      render(<LiveLogViewer mode="security" />);

      // Wait for connection to establish
      expect(await screen.findByText('Connected')).toBeTruthy();

      const securityLog: logsApi.SecurityLogEntry = {
        timestamp: '2025-12-12T10:30:00Z',
        level: 'info',
        logger: 'http.log.access',
        client_ip: '192.168.1.100',
        method: 'GET',
        uri: '/api/test',
        status: 200,
        duration: 0.123,
        size: 1024,
        user_agent: 'TestAgent/1.0',
        host: 'example.com',
        source: 'normal',
        blocked: false,
      };

      if (mockOnSecurityMessage) {
        mockOnSecurityMessage(securityLog);
      }

      await waitFor(() => {
        expect(screen.getByText('123.0ms')).toBeTruthy();
      });
    });

    it('displays status code with appropriate color for security logs', async () => {
      render(<LiveLogViewer mode="security" />);

      // Wait for connection to establish
      expect(await screen.findByText('Connected')).toBeTruthy();

      if (mockOnSecurityMessage) {
        mockOnSecurityMessage({
          timestamp: '2025-12-12T10:30:00Z',
          level: 'info',
          logger: 'http.log.access',
          client_ip: '192.168.1.100',
          method: 'GET',
          uri: '/ok',
          status: 200,
          duration: 0.01,
          size: 100,
          user_agent: 'Test/1.0',
          host: 'example.com',
          source: 'normal',
          blocked: false,
        });
      }

      await waitFor(() => {
        expect(screen.getByText('[200]')).toBeTruthy();
      });
    });
  });

  // ============================================================
  // Mode Toggle Tests
  // ============================================================

  describe('Mode Toggle', () => {
    it('switches from application to security mode', async () => {
      const user = userEvent.setup();
      render(<LiveLogViewer mode="application" />);

      expect(screen.getByText('Live Security Logs')).toBeTruthy();
      expect(logsApi.connectLiveLogs).toHaveBeenCalled();

      // Click security mode button
      const securityButton = screen.getByTitle('Security access logs');
      await user.click(securityButton);

      await waitFor(() => {
        expect(screen.getByText('Security Access Logs')).toBeTruthy();
        expect(logsApi.connectSecurityLogs).toHaveBeenCalled();
      });
    });

    it('switches from security to application mode', async () => {
      const user = userEvent.setup();
      render(<LiveLogViewer mode="security" />);

      expect(screen.getByText('Security Access Logs')).toBeTruthy();

      // Click application mode button
      const appButton = screen.getByTitle('Application logs');
      await user.click(appButton);

      await waitFor(() => {
        expect(screen.getByText('Live Security Logs')).toBeTruthy();
      });
    });

    it('clears logs when switching modes', async () => {
      const user = userEvent.setup();
      render(<LiveLogViewer mode="application" />);

      // Add a log in application mode
      if (mockOnMessage) {
        mockOnMessage({ level: 'info', timestamp: '2025-12-12T10:30:00Z', message: 'App log' });
      }

      await waitFor(() => {
        expect(screen.getByText('App log')).toBeTruthy();
      });

      // Switch to security mode
      const securityButton = screen.getByTitle('Security access logs');
      await user.click(securityButton);

      await waitFor(() => {
        expect(screen.queryByText('App log')).toBeFalsy();
        expect(screen.getByText('No logs yet. Waiting for events...')).toBeTruthy();
      });
    });

    it('resets filters when switching modes', async () => {
      const user = userEvent.setup();
      render(<LiveLogViewer mode="application" />);

      // Set a filter
      const filterInput = screen.getByPlaceholderText('Filter by text...');
      await user.type(filterInput, 'test');

      // Switch to security mode
      const securityButton = screen.getByTitle('Security access logs');
      await user.click(securityButton);

      await waitFor(() => {
        // Filter should be cleared
        expect(screen.getByPlaceholderText('Filter by text...')).toHaveValue('');
      });
    });
  });
});
