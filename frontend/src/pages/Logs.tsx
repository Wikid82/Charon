import { useState, useEffect, type FC } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { getLogs, getLogContent, downloadLog, LogFilter } from '../api/logs';
import { Card } from '../components/ui/Card';
import { Loader2, FileText, ChevronLeft, ChevronRight } from 'lucide-react';
import { LogTable } from '../components/LogTable';
import { LogFilters } from '../components/LogFilters';
import { Button } from '../components/ui/Button';

const Logs: FC = () => {
  const [searchParams] = useSearchParams();
  const [selectedLog, setSelectedLog] = useState<string | null>(null);

  // Filter State
  const [search, setSearch] = useState(searchParams.get('search') || '');
  const [host, setHost] = useState('');
  const [status, setStatus] = useState('');
  const [level, setLevel] = useState('');
  const [sort, setSort] = useState<'asc' | 'desc'>('desc');
  const [page, setPage] = useState(0);
  const limit = 50;

  const { data: logs, isLoading: isLoadingLogs } = useQuery({
    queryKey: ['logs'],
    queryFn: getLogs,
  });

  // Select first log by default if none selected
  useEffect(() => {
    if (!selectedLog && logs && logs.length > 0) {
      setSelectedLog(logs[0].name);
    }
  }, [logs, selectedLog]);

  const filter: LogFilter = {
    search,
    host,
    status,
    level,
    limit,
    offset: page * limit,
    sort,
  };

  const { data: logData, isLoading: isLoadingContent, refetch: refetchContent } = useQuery({
    queryKey: ['logContent', selectedLog, search, host, status, level, page, sort],
    queryFn: () => (selectedLog ? getLogContent(selectedLog, filter) : Promise.resolve(null)),
    enabled: !!selectedLog,
  });

  const handleDownload = () => {
    if (selectedLog) downloadLog(selectedLog);
  };

  const totalPages = logData ? Math.ceil(logData.total / limit) : 0;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Access Logs</h1>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        {/* Log File List */}
        <div className="md:col-span-1 space-y-4">
          <Card className="p-4">
            <h2 className="text-lg font-semibold mb-4 text-gray-900 dark:text-white">Log Files</h2>
            {isLoadingLogs ? (
              <div className="flex justify-center p-4">
                <Loader2 className="w-6 h-6 animate-spin text-blue-500" />
              </div>
            ) : (
              <div className="space-y-2">
                {logs?.map((log) => (
                  <button
                    key={log.name}
                    onClick={() => {
                      setSelectedLog(log.name);
                      setPage(0);
                    }}
                    className={`w-full text-left px-3 py-2 rounded-md text-sm transition-colors flex items-center ${
                      selectedLog === log.name
                        ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 border border-blue-200 dark:border-blue-800'
                        : 'hover:bg-gray-50 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-300'
                    }`}
                  >
                    <FileText className="w-4 h-4 mr-2" />
                    <div className="flex-1 truncate">
                      <div className="font-medium">{log.name}</div>
                      <div className="text-xs text-gray-500">{(log.size / 1024 / 1024).toFixed(2)} MB</div>
                    </div>
                  </button>
                ))}
                {logs?.length === 0 && (
                  <div className="text-sm text-gray-500 text-center py-4">No log files found</div>
                )}
              </div>
            )}
          </Card>
        </div>

        {/* Log Content */}
        <div className="md:col-span-3 space-y-4">
          {selectedLog ? (
            <>
              <LogFilters
                search={search}
                onSearchChange={(v) => {
                  setSearch(v);
                  setPage(0);
                }}
                host={host}
                onHostChange={(v) => {
                  setHost(v);
                  setPage(0);
                }}
                status={status}
                onStatusChange={(v) => {
                  setStatus(v);
                  setPage(0);
                }}
                level={level}
                onLevelChange={(v) => {
                  setLevel(v);
                  setPage(0);
                }}
                sort={sort}
                onSortChange={(v) => {
                  setSort(v);
                  setPage(0);
                }}
                onRefresh={refetchContent}
                onDownload={handleDownload}
                isLoading={isLoadingContent}
              />

              <Card className="overflow-hidden">
                <LogTable logs={logData?.logs || []} isLoading={isLoadingContent} />

                {/* Pagination */}
                {logData && logData.total > 0 && (
                  <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex flex-col sm:flex-row items-center justify-between gap-4">
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      Showing {logData.offset + 1} to {Math.min(logData.offset + limit, logData.total)} of {logData.total} entries
                    </div>

                    <div className="flex items-center gap-4">
                      <div className="flex items-center gap-2">
                        <span className="text-sm text-gray-500 dark:text-gray-400">Page</span>
                        <select
                          value={page}
                          onChange={(e) => setPage(Number(e.target.value))}
                          className="block w-20 rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm dark:bg-gray-700 dark:text-white py-1"
                          disabled={isLoadingContent}
                        >
                          {Array.from({ length: totalPages }, (_, i) => (
                            <option key={i} value={i}>
                              {i + 1}
                            </option>
                          ))}
                        </select>
                        <span className="text-sm text-gray-500 dark:text-gray-400">of {totalPages}</span>
                      </div>

                      <div className="flex gap-2">
                        <Button variant="secondary" size="sm" onClick={() => setPage((p) => Math.max(0, p - 1))} disabled={page === 0 || isLoadingContent}>
                          <ChevronLeft className="w-4 h-4" />
                        </Button>
                        <Button variant="secondary" size="sm" onClick={() => setPage((p) => p + 1)} disabled={page >= totalPages - 1 || isLoadingContent}>
                          <ChevronRight className="w-4 h-4" />
                        </Button>
                      </div>
                    </div>
                  </div>
                )}
              </Card>
            </>
          ) : (
            <Card className="p-8 flex flex-col items-center justify-center text-gray-500 h-64">
              <FileText className="w-12 h-12 mb-4 opacity-20" />
              <p>Select a log file to view contents</p>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
};

export default Logs;
import { useState, useEffect, type FC } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { getLogs, getLogContent, downloadLog, LogFilter } from '../api/logs';
import { Card } from '../components/ui/Card';
import { Loader2, FileText, ChevronLeft, ChevronRight } from 'lucide-react';
import { LogTable } from '../components/LogTable';
import { LogFilters } from '../components/LogFilters';
import { Button } from '../components/ui/Button';

const Logs: React.FC = () => {
  const [searchParams] = useSearchParams();
  const [selectedLog, setSelectedLog] = useState<string | null>(null);

  // Filter State
  const [search, setSearch] = useState(searchParams.get('search') || '');
  const [host, setHost] = useState('');
  const [status, setStatus] = useState('');
  const [level, setLevel] = useState('');
  const [sort, setSort] = useState<'asc' | 'desc'>('desc');
  const [page, setPage] = useState(0);
  const limit = 50;

  const { data: logs, isLoading: isLoadingLogs } = useQuery({
    queryKey: ['logs'],
    queryFn: getLogs,
  });

  // Select first log by default if none selected
  useEffect(() => {
    if (!selectedLog && logs && logs.length > 0) {
      setSelectedLog(logs[0].name);
    }
  }, [logs, selectedLog]);

  const filter: LogFilter = {
    search,
    host,
    status,
    level,
    limit,
    offset: page * limit,
    sort
  };

  const { data: logData, isLoading: isLoadingContent, refetch: refetchContent } = useQuery({
    queryKey: ['logContent', selectedLog, search, host, status, level, page, sort],
    queryFn: () => selectedLog ? getLogContent(selectedLog, filter) : Promise.resolve(null),
    enabled: !!selectedLog,
  });

  const handleDownload = () => {
    if (selectedLog) {
      downloadLog(selectedLog);
    }
  };
  const Logs: FC = () => {
  const totalPages = logData ? Math.ceil(logData.total / limit) : 0;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Access Logs</h1>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        {/* Log File List */}
        <div className="md:col-span-1 space-y-4">
          <Card className="p-4">
            <h2 className="text-lg font-semibold mb-4 text-gray-900 dark:text-white">Log Files</h2>
            {isLoadingLogs ? (
              <div className="flex justify-center p-4">
                <Loader2 className="w-6 h-6 animate-spin text-blue-500" />
              </div>
            ) : (
              <div className="space-y-2">
                {logs?.map((log) => (
                  <button
                    key={log.name}
                    onClick={() => { setSelectedLog(log.name); setPage(0); }}
                    className={`w-full text-left px-3 py-2 rounded-md text-sm transition-colors flex items-center ${
                      selectedLog === log.name
                        ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 border border-blue-200 dark:border-blue-800'
                        : 'hover:bg-gray-50 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-300'
                    }`}
                  >
                    <FileText className="w-4 h-4 mr-2" />
                    <div className="flex-1 truncate">
                      <div className="font-medium">{log.name}</div>
                      <div className="text-xs text-gray-500">{(log.size / 1024 / 1024).toFixed(2)} MB</div>
                    </div>
                  </button>
                ))}
                {logs?.length === 0 && (
                  <div className="text-sm text-gray-500 text-center py-4">No log files found</div>
                )}
              </div>
            )}
          </Card>
        </div>

        {/* Log Content */}
        <div className="md:col-span-3 space-y-4">
          {selectedLog ? (
            <>
              <LogFilters
                search={search}
                onSearchChange={(v) => { setSearch(v); setPage(0); }}
                host={host}
                onHostChange={(v) => { setHost(v); setPage(0); }}
                status={status}
                onStatusChange={(v) => { setStatus(v); setPage(0); }}
                level={level}
                onLevelChange={(v) => { setLevel(v); setPage(0); }}
                sort={sort}
                onSortChange={(v) => { setSort(v); setPage(0); }}
                onRefresh={refetchContent}
                onDownload={handleDownload}
                isLoading={isLoadingContent}
              />

              <Card className="overflow-hidden">
                <LogTable
                  logs={logData?.logs || []}
                  isLoading={isLoadingContent}
                />

                {/* Pagination */}
                {logData && logData.total > 0 && (
                  <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex flex-col sm:flex-row items-center justify-between gap-4">
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      Showing {logData.offset + 1} to {Math.min(logData.offset + limit, logData.total)} of {logData.total} entries
                    </div>

                    <div className="flex items-center gap-4">
                      <div className="flex items-center gap-2">
                        <span className="text-sm text-gray-500 dark:text-gray-400">Page</span>
                        <select
                          value={page}
                          onChange={(e) => setPage(Number(e.target.value))}
                          className="block w-20 rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm dark:bg-gray-700 dark:text-white py-1"
                          disabled={isLoadingContent}
                        >
                          {Array.from({ length: totalPages }, (_, i) => (
                            <option key={i} value={i}>
                              {i + 1}
                            </option>
                          ))}
                        </select>
                        <span className="text-sm text-gray-500 dark:text-gray-400">of {totalPages}</span>
                      </div>

                      <div className="flex gap-2">
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => setPage(p => Math.max(0, p - 1))}
                          disabled={page === 0 || isLoadingContent}
                        >
                          <ChevronLeft className="w-4 h-4" />
                        </Button>
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => setPage(p => p + 1)}
                          disabled={page >= totalPages - 1 || isLoadingContent}
                        >
                          <ChevronRight className="w-4 h-4" />
                        </Button>
                      </div>
                    </div>
                  </div>
                )}
              </Card>
            </>
          ) : (
            <Card className="p-8 flex flex-col items-center justify-center text-gray-500 h-64">
              <FileText className="w-12 h-12 mb-4 opacity-20" />
              <p>Select a log file to view contents</p>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
};

export default Logs;
