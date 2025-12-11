import { useEffect, useRef, useState } from 'react';
import { connectLiveLogs, LiveLogEntry, LiveLogFilter } from '../api/logs';
import { Button } from './ui/Button';
import { Pause, Play, Trash2, Filter } from 'lucide-react';

interface LiveLogViewerProps {
  filters?: LiveLogFilter;
  maxLogs?: number;
  className?: string;
}

export function LiveLogViewer({ filters = {}, maxLogs = 500, className = '' }: LiveLogViewerProps) {
  const [logs, setLogs] = useState<LiveLogEntry[]>([]);
  const [isPaused, setIsPaused] = useState(false);
  const [isConnected, setIsConnected] = useState(false);
  const [textFilter, setTextFilter] = useState('');
  const [levelFilter, setLevelFilter] = useState('');
  const logContainerRef = useRef<HTMLDivElement>(null);
  const closeConnectionRef = useRef<(() => void) | null>(null);

  // Auto-scroll when new logs arrive (only if not paused and user hasn't scrolled up)
  const shouldAutoScroll = useRef(true);

  useEffect(() => {
    // Connect to WebSocket
    const closeConnection = connectLiveLogs(
      filters,
      (log: LiveLogEntry) => {
        if (!isPaused) {
          setLogs((prev) => {
            const updated = [...prev, log];
            // Keep only last maxLogs entries
            if (updated.length > maxLogs) {
              return updated.slice(updated.length - maxLogs);
            }
            return updated;
          });
        }
      },
      () => {
        // onOpen callback - connection established
        console.log('Live log viewer connected');
        setIsConnected(true);
      },
      (error) => {
        console.error('WebSocket error:', error);
        setIsConnected(false);
      },
      () => {
        console.log('Live log viewer disconnected');
        setIsConnected(false);
      }
    );

    closeConnectionRef.current = closeConnection;
    // Don't set isConnected here - wait for onOpen callback

    return () => {
      closeConnection();
      setIsConnected(false);
    };
  }, [filters, isPaused, maxLogs]);

  // Handle auto-scroll
  useEffect(() => {
    if (shouldAutoScroll.current && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [logs]);

  // Track if user has manually scrolled
  const handleScroll = () => {
    if (logContainerRef.current) {
      const { scrollTop, scrollHeight, clientHeight } = logContainerRef.current;
      // If scrolled to bottom (within 50px), enable auto-scroll
      shouldAutoScroll.current = scrollHeight - scrollTop - clientHeight < 50;
    }
  };

  const handleClear = () => {
    setLogs([]);
  };

  const handleTogglePause = () => {
    setIsPaused(!isPaused);
  };

  // Filter logs based on text and level
  const filteredLogs = logs.filter((log) => {
    if (textFilter && !log.message.toLowerCase().includes(textFilter.toLowerCase())) {
      return false;
    }
    if (levelFilter && log.level.toLowerCase() !== levelFilter.toLowerCase()) {
      return false;
    }
    return true;
  });

  // Color coding based on log level
  const getLevelColor = (level: string) => {
    const normalized = level.toLowerCase();
    if (normalized.includes('error') || normalized.includes('fatal')) return 'text-red-400';
    if (normalized.includes('warn')) return 'text-yellow-400';
    if (normalized.includes('info')) return 'text-blue-400';
    if (normalized.includes('debug')) return 'text-gray-400';
    return 'text-gray-300';
  };

  const formatTimestamp = (timestamp: string) => {
    try {
      const date = new Date(timestamp);
      return date.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
    } catch {
      return timestamp;
    }
  };

  return (
    <div className={`bg-gray-900 rounded-lg border border-gray-700 ${className}`}>
      {/* Header with controls */}
      <div className="flex items-center justify-between p-3 border-b border-gray-700">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold text-white">Live Security Logs</h3>
          <span
            className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
              isConnected ? 'bg-green-900 text-green-300' : 'bg-red-900 text-red-300'
            }`}
          >
            {isConnected ? 'Connected' : 'Disconnected'}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={handleTogglePause}
            className="flex items-center gap-1"
            title={isPaused ? 'Resume' : 'Pause'}
          >
            {isPaused ? <Play className="w-4 h-4" /> : <Pause className="w-4 h-4" />}
          </Button>
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
      <div className="flex items-center gap-2 p-2 border-b border-gray-700 bg-gray-800">
        <Filter className="w-4 h-4 text-gray-400" />
        <input
          type="text"
          placeholder="Filter by text..."
          value={textFilter}
          onChange={(e) => setTextFilter(e.target.value)}
          className="flex-1 px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:border-blue-500"
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
          <div key={index} className="mb-1 hover:bg-gray-900 px-1 -mx-1 rounded">
            <span className="text-gray-500">{formatTimestamp(log.timestamp)}</span>
            <span className={`ml-2 font-semibold ${getLevelColor(log.level)}`}>{log.level.toUpperCase()}</span>
            {log.source && <span className="ml-2 text-purple-400">[{log.source}]</span>}
            <span className="ml-2 text-gray-200">{log.message}</span>
            {log.data && Object.keys(log.data).length > 0 && (
              <div className="ml-8 text-gray-400 text-xs">
                {JSON.stringify(log.data, null, 2)}
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Footer with log count */}
      <div className="p-2 border-t border-gray-700 bg-gray-800 text-xs text-gray-400 flex items-center justify-between">
        <span>
          Showing {filteredLogs.length} of {logs.length} logs
        </span>
        {isPaused && <span className="text-yellow-400">⏸ Paused</span>}
      </div>
    </div>
  );
}
