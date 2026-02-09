import { useEffect, useRef, useState, useCallback } from 'react';
import {
  connectLiveLogs,
  connectSecurityLogs,
  LiveLogEntry,
  LiveLogFilter,
  SecurityLogEntry,
  SecurityLogFilter,
} from '../api/logs';
import { Button } from './ui/Button';
import { Pause, Play, Trash2, Filter, Shield, Globe } from 'lucide-react';

/**
 * Log viewing mode: application logs vs security access logs
 */
export type LogMode = 'application' | 'security';

interface LiveLogViewerProps {
  /** Filters for application log mode */
  filters?: LiveLogFilter;
  /** Filters for security log mode */
  securityFilters?: SecurityLogFilter;
  /** Initial log viewing mode */
  mode?: LogMode;
  /** Maximum number of log entries to retain */
  maxLogs?: number;
  /** Additional CSS classes */
  className?: string;
}

/**
 * Unified display entry for both application and security logs
 */
interface DisplayLogEntry {
  timestamp: string;
  level: string;
  source: string;
  message: string;
  blocked?: boolean;
  blockReason?: string;
  clientIP?: string;
  method?: string;
  host?: string;
  uri?: string;
  status?: number;
  duration?: number;
  userAgent?: string;
  details?: Record<string, unknown>;
}

/**
 * Convert a LiveLogEntry to unified display format
 */
const toDisplayFromLive = (entry: LiveLogEntry): DisplayLogEntry => ({
  timestamp: entry.timestamp,
  level: entry.level,
  source: entry.source || 'app',
  message: entry.message,
  details: entry.data,
});

/**
 * Convert a SecurityLogEntry to unified display format
 */
const toDisplayFromSecurity = (entry: SecurityLogEntry): DisplayLogEntry => ({
  timestamp: entry.timestamp,
  level: entry.level,
  source: entry.source,
  message: entry.blocked
    ? `🚫 BLOCKED: ${entry.block_reason || 'Access denied'}`
    : `${entry.method} ${entry.uri} → ${entry.status}`,
  blocked: entry.blocked,
  blockReason: entry.block_reason,
  clientIP: entry.client_ip,
  method: entry.method,
  host: entry.host,
  uri: entry.uri,
  status: entry.status,
  duration: entry.duration,
  userAgent: entry.user_agent,
  details: entry.details,
});

/**
 * Get background/text styling based on log entry properties
 */
const getEntryStyle = (log: DisplayLogEntry): string => {
  if (log.blocked) {
    return 'bg-red-900/30 border-l-2 border-red-500';
  }
  const level = log.level.toLowerCase();
  if (level.includes('error') || level.includes('fatal')) return 'text-red-400';
  if (level.includes('warn')) return 'text-yellow-400';
  return '';
};

/**
 * Get badge color for security source
 */
const getSourceBadgeColor = (source: string): string => {
  const colors: Record<string, string> = {
    waf: 'bg-orange-600',
    crowdsec: 'bg-purple-600',
    ratelimit: 'bg-blue-600',
    acl: 'bg-green-600',
    normal: 'bg-gray-600',
    cerberus: 'bg-indigo-600',
    app: 'bg-gray-500',
  };
  return colors[source.toLowerCase()] || 'bg-gray-500';
};

/**
 * Format timestamp for display
 */
const formatTimestamp = (timestamp: string): string => {
  try {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
  } catch {
    return timestamp;
  }
};

/**
 * Get level color for application logs
 */
const getLevelColor = (level: string): string => {
  const normalized = level.toLowerCase();
  if (normalized.includes('error') || normalized.includes('fatal')) return 'text-red-400';
  if (normalized.includes('warn')) return 'text-yellow-400';
  if (normalized.includes('info')) return 'text-blue-400';
  if (normalized.includes('debug')) return 'text-gray-400';
  return 'text-gray-300';
};

// Stable default filter objects to prevent useEffect re-triggers on parent re-render
const EMPTY_LIVE_FILTER: LiveLogFilter = {};
const EMPTY_SECURITY_FILTER: SecurityLogFilter = {};

export function LiveLogViewer({
  filters = EMPTY_LIVE_FILTER,
  securityFilters = EMPTY_SECURITY_FILTER,
  mode = 'security',
  maxLogs = 500,
  className = '',
}: LiveLogViewerProps) {
  const [logs, setLogs] = useState<DisplayLogEntry[]>([]);
  const [isPaused, setIsPaused] = useState(false);
  const [isConnected, setIsConnected] = useState(false);
  const [connectionError, setConnectionError] = useState<string | null>(null);
  const [currentMode, setCurrentMode] = useState<LogMode>(mode);
  const [textFilter, setTextFilter] = useState('');
  const [levelFilter, setLevelFilter] = useState('');
  const [sourceFilter, setSourceFilter] = useState('');
  const [showBlockedOnly, setShowBlockedOnly] = useState(false);
  const logContainerRef = useRef<HTMLDivElement>(null);
  const closeConnectionRef = useRef<(() => void) | null>(null);
  const shouldAutoScroll = useRef(true);
  const isPausedRef = useRef(isPaused);

  // Keep ref in sync with state for use in WebSocket handlers
  useEffect(() => {
    isPausedRef.current = isPaused;
  }, [isPaused]);

  // Handle mode change - clear logs and update filters
  const handleModeChange = useCallback((newMode: LogMode) => {
    setCurrentMode(newMode);
    setLogs([]);
    setTextFilter('');
    setLevelFilter('');
    setSourceFilter('');
    setShowBlockedOnly(false);
  }, []);

  // Connection effect - reconnects when mode or external filters change
  useEffect(() => {
    // Close existing connection
    if (closeConnectionRef.current) {
      closeConnectionRef.current();
      closeConnectionRef.current = null;
    }

    const handleOpen = () => {
      console.log(`${currentMode} log viewer connected`);
      setIsConnected(true);
      setConnectionError(null);
    };

    const handleError = (error: Event) => {
      console.error(`${currentMode} log viewer error:`, error);
      setIsConnected(false);
      setConnectionError('Failed to connect to log stream. Check your authentication or try refreshing.');
    };

    const handleClose = () => {
      console.log(`${currentMode} log viewer disconnected`);
      setIsConnected(false);
    };

    if (currentMode === 'security') {
      // Connect to security logs endpoint
      const handleSecurityMessage = (entry: SecurityLogEntry) => {
        // Use ref to check paused state - avoids WebSocket reconnection when pausing
        if (isPausedRef.current) return;
        const displayEntry = toDisplayFromSecurity(entry);
        setLogs((prev) => {
          const updated = [...prev, displayEntry];
          return updated.length > maxLogs ? updated.slice(-maxLogs) : updated;
        });
      };

      // Build filters including blocked_only if selected
      const effectiveFilters: SecurityLogFilter = {
        ...securityFilters,
        blocked_only: showBlockedOnly || securityFilters.blocked_only,
      };

      closeConnectionRef.current = connectSecurityLogs(
        effectiveFilters,
        handleSecurityMessage,
        handleOpen,
        handleError,
        handleClose
      );
    } else {
      // Connect to application logs endpoint
      const handleLiveMessage = (entry: LiveLogEntry) => {
        // Use ref to check paused state - avoids WebSocket reconnection when pausing
        if (isPausedRef.current) return;
        const displayEntry = toDisplayFromLive(entry);
        setLogs((prev) => {
          const updated = [...prev, displayEntry];
          return updated.length > maxLogs ? updated.slice(-maxLogs) : updated;
        });
      };

      closeConnectionRef.current = connectLiveLogs(
        filters,
        handleLiveMessage,
        handleOpen,
        handleError,
        handleClose
      );
    }

    return () => {
      if (closeConnectionRef.current) {
        closeConnectionRef.current();
        closeConnectionRef.current = null;
      }
      setIsConnected(false);
    };
    // Note: isPaused is intentionally excluded - we use isPausedRef to avoid reconnecting when pausing
  }, [currentMode, filters, securityFilters, maxLogs, showBlockedOnly]);

  // Auto-scroll effect
  useEffect(() => {
    if (shouldAutoScroll.current && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [logs]);

  // Track manual scrolling
  const handleScroll = () => {
    if (logContainerRef.current) {
      const { scrollTop, scrollHeight, clientHeight } = logContainerRef.current;
      // Enable auto-scroll if scrolled to bottom (within 50px threshold)
      shouldAutoScroll.current = scrollHeight - scrollTop - clientHeight < 50;
    }
  };

  const handleClear = () => {
    setLogs([]);
  };

  const handleTogglePause = () => {
    setIsPaused(!isPaused);
  };

  // Client-side filtering
  const filteredLogs = logs.filter((log) => {
    // Text filter - search in message, URI, host, IP
    if (textFilter) {
      const searchText = textFilter.toLowerCase();
      const matchFields = [
        log.message,
        log.uri,
        log.host,
        log.clientIP,
        log.blockReason,
      ].filter(Boolean).map(s => s!.toLowerCase());

      if (!matchFields.some(field => field.includes(searchText))) {
        return false;
      }
    }

    // Level filter
    if (levelFilter && log.level.toLowerCase() !== levelFilter.toLowerCase()) {
      return false;
    }

    // Source filter (security mode only)
    if (sourceFilter && log.source.toLowerCase() !== sourceFilter.toLowerCase()) {
      return false;
    }

    return true;
  });

  return (
    <div className={`bg-gray-900 rounded-lg border border-gray-700 ${className}`}>
      {/* Header with mode toggle and controls */}
      <div className="flex items-center justify-between p-3 border-b border-gray-700">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold text-white">
            {currentMode === 'security' ? 'Security Access Logs' : 'Live Security Logs'}
          </h3>
          <span
            className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
              isConnected ? 'bg-green-900 text-green-300' : 'bg-red-900 text-red-300'
            }`}
            data-testid="connection-status"
          >
            {isConnected ? 'Connected' : 'Disconnected'}
          </span>
          {connectionError && (
            <div className="text-xs text-red-400 bg-red-900/20 px-2 py-1 rounded" data-testid="connection-error">
              {connectionError}
            </div>
          )}
        </div>
        <div className="flex items-center gap-2">
          {/* Mode toggle */}
          <div className="flex bg-gray-800 rounded-md p-0.5" data-testid="mode-toggle">
            <button
              onClick={() => handleModeChange('application')}
              className={`px-2 py-1 text-xs rounded flex items-center gap-1 transition-colors ${
                currentMode === 'application' ? 'bg-blue-600 text-white' : 'text-gray-400 hover:text-white'
              }`}
              title="Application logs"
            >
              <Globe className="w-4 h-4" />
              <span className="hidden sm:inline">App</span>
            </button>
            <button
              onClick={() => handleModeChange('security')}
              className={`px-2 py-1 text-xs rounded flex items-center gap-1 transition-colors ${
                currentMode === 'security' ? 'bg-blue-600 text-white' : 'text-gray-400 hover:text-white'
              }`}
              title="Security access logs"
            >
              <Shield className="w-4 h-4" />
              <span className="hidden sm:inline">Security</span>
            </button>
          </div>
          {/* Pause/Resume */}
          <Button
            variant="ghost"
            size="sm"
            onClick={handleTogglePause}
            className="flex items-center gap-1"
            title={isPaused ? 'Resume' : 'Pause'}
          >
            {isPaused ? <Play className="w-4 h-4" /> : <Pause className="w-4 h-4" />}
          </Button>
          {/* Clear */}
          <Button
            variant="ghost"
            size="sm"
            onClick={handleClear}
            className="flex items-center gap-1"
            title="Clear logs"
          >
            <Trash2 className="w-4 h-4" />
          </Button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-2 p-2 border-b border-gray-700 bg-gray-800">
        <Filter className="w-4 h-4 text-gray-400" />
        <input
          type="text"
          placeholder="Filter by text..."
          value={textFilter}
          onChange={(e) => setTextFilter(e.target.value)}
          className="flex-1 min-w-32 px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:border-blue-500"
        />
        <select
          value={levelFilter}
          onChange={(e) => setLevelFilter(e.target.value)}
          className="px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
        >
          <option value="">All Levels</option>
          <option value="debug">Debug</option>
          <option value="info">Info</option>
          <option value="warn">Warning</option>
          <option value="error">Error</option>
          <option value="fatal">Fatal</option>
        </select>
        {/* Security mode specific filters */}
        {currentMode === 'security' && (
          <>
            <select
              value={sourceFilter}
              onChange={(e) => setSourceFilter(e.target.value)}
              className="px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
            >
              <option value="">All Sources</option>
              <option value="waf">WAF</option>
              <option value="crowdsec">CrowdSec</option>
              <option value="ratelimit">Rate Limit</option>
              <option value="acl">ACL</option>
              <option value="normal">Normal</option>
            </select>
            <label className="flex items-center gap-1 text-xs text-gray-400 cursor-pointer">
              <input
                type="checkbox"
                checked={showBlockedOnly}
                onChange={(e) => setShowBlockedOnly(e.target.checked)}
                className="rounded border-gray-600 bg-gray-700 text-blue-500 focus:ring-blue-500"
              />
              Blocked only
            </label>
          </>
        )}
      </div>

      {/* Log display */}
      <div
        ref={logContainerRef}
        onScroll={handleScroll}
        className="h-96 overflow-y-auto p-3 font-mono text-xs bg-black"
        style={{ scrollBehavior: 'smooth' }}
      >
        {filteredLogs.length === 0 && (
          <div className="text-gray-500 text-center py-8">
            {logs.length === 0 ? 'No logs yet. Waiting for events...' : 'No logs match the current filters.'}
          </div>
        )}
        {filteredLogs.map((log, index) => (
          <div
            key={index}
            className={`mb-1 hover:bg-gray-900 px-1 -mx-1 rounded ${getEntryStyle(log)}`}
            data-testid="log-entry"
          >
            <span className="text-gray-500">{formatTimestamp(log.timestamp)}</span>

            {/* Source badge for security mode */}
            {currentMode === 'security' && (
              <span className={`ml-2 px-1 rounded text-xs text-white ${getSourceBadgeColor(log.source)}`}>
                {log.source.toUpperCase()}
              </span>
            )}

            {/* Level badge for application mode */}
            {currentMode === 'application' && (
              <span className={`ml-2 font-semibold ${getLevelColor(log.level)}`}>
                {log.level.toUpperCase()}
              </span>
            )}

            {/* Client IP for security logs */}
            {currentMode === 'security' && log.clientIP && (
              <span className="ml-2 text-cyan-400">{log.clientIP}</span>
            )}

            {/* Source tag for application logs */}
            {currentMode === 'application' && log.source && log.source !== 'app' && (
              <span className="ml-2 text-purple-400">[{log.source}]</span>
            )}

            {/* Message */}
            <span className="ml-2 text-gray-200">{log.message}</span>

            {/* Block reason badge */}
            {log.blocked && log.blockReason && (
              <span className="ml-2 text-red-400 text-xs">[{log.blockReason}]</span>
            )}

            {/* Status code for security logs */}
            {currentMode === 'security' && log.status && !log.blocked && (
              <span className={`ml-2 ${log.status >= 400 ? 'text-red-400' : log.status >= 300 ? 'text-yellow-400' : 'text-green-400'}`}>
                [{log.status}]
              </span>
            )}

            {/* Duration for security logs */}
            {currentMode === 'security' && log.duration !== undefined && (
              <span className="ml-1 text-gray-500">
                {log.duration < 1 ? `${(log.duration * 1000).toFixed(1)}ms` : `${log.duration.toFixed(2)}s`}
              </span>
            )}

            {/* Additional data */}
            {log.details && Object.keys(log.details).length > 0 && (
              <div className="ml-8 text-gray-400 text-xs">
                {JSON.stringify(log.details, null, 2)}
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Footer with log count */}
      <div className="p-2 border-t border-gray-700 bg-gray-800 text-xs text-gray-400 flex items-center justify-between" data-testid="log-count">
        <span>
          Showing {filteredLogs.length} of {logs.length} logs
        </span>
        {isPaused && <span className="text-yellow-400">⏸ Paused</span>}
      </div>
    </div>
  );
}
