import { FileText, ChevronLeft, ChevronRight, ScrollText, AlertTriangle } from 'lucide-react';
import { useState, useEffect, type FC } from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';

import { type LogFilter, type LogSortField } from '../api/logs';
import { PageShell } from '../components/layout/PageShell';
import { LogFilters } from '../components/LogFilters';
import { LogTable } from '../components/LogTable';
import {
  Button,
  Card,
  Badge,
  EmptyState,
  Skeleton,
  SkeletonList,
} from '../components/ui';
import { useDebouncedValue } from '../hooks/useDebouncedValue';
import { useDownloadLog, useLogContent, useLogFiles } from '../hooks/useLogs';

const Logs: FC = () => {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const [selectedLog, setSelectedLog] = useState<string | null>(null);

  // Filter State
  const [search, setSearch] = useState(searchParams.get('search') || '');
  const [host, setHost] = useState('');
  const [status, setStatus] = useState('');
  const [level, setLevel] = useState('');
  const [sortBy, setSortBy] = useState<LogSortField>('ts');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc');
  const [page, setPage] = useState(0);
  const limit = 50;

  const debouncedSearch = useDebouncedValue(search);
  const debouncedHost = useDebouncedValue(host);

  // Reset pagination only once the debounced values change, so a keystroke
  // doesn't reset the page before the query actually fires.
  useEffect(() => {
    setPage(0);
  }, [debouncedSearch, debouncedHost]);

  const { data: logs, isLoading: isLoadingLogs } = useLogFiles();

  // Select first log by default if none selected
  useEffect(() => {
    if (!selectedLog && logs && logs.length > 0) {
      setSelectedLog(logs[0].name);
    }
  }, [logs, selectedLog]);

  const filter: LogFilter = {
    search: debouncedSearch,
    host: debouncedHost,
    status,
    level,
    limit,
    offset: page * limit,
    sort: sortDir,
    sortBy,
  };

  const {
    data: logData,
    isLoading: isLoadingContent,
    isFetching: isFetchingContent,
    refetch: refetchContent,
  } = useLogContent(selectedLog, filter);

  const downloadMutation = useDownloadLog();

  const handleDownload = () => {
    if (selectedLog) downloadMutation.mutate(selectedLog);
  };

  const handleSortChange = (field: LogSortField) => {
    if (field === sortBy) {
      setSortDir((d) => (d === 'desc' ? 'asc' : 'desc'));
    } else {
      setSortBy(field);
      setSortDir('desc');
    }
    setPage(0);
  };

  const totalPages = logData ? Math.ceil(logData.total / limit) : 0;

  return (
    <PageShell
      title={t('logs.title')}
      description={t('logs.description')}
    >
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        {/* Log File List */}
        <div className="md:col-span-1 space-y-4">
          <Card className="p-4" data-testid="log-file-list">
            <h2 className="text-lg font-semibold mb-4 text-content-primary">{t('logs.logFiles')}</h2>
            {isLoadingLogs ? (
              <SkeletonList items={4} showAvatar={false} />
            ) : logs?.length === 0 ? (
              <div className="text-sm text-content-muted text-center py-4">{t('logs.noLogFiles')}</div>
            ) : (
              <div className="space-y-2">
                {logs?.map((log) => (
                  <button
                    key={log.name}
                    onClick={() => {
                      setSelectedLog(log.name);
                      setPage(0);
                    }}
                    data-testid={`log-file-${log.name}`}
                    className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors flex items-center ${
                      selectedLog === log.name
                        ? 'bg-brand-500/10 text-brand-500 border border-brand-500/30'
                        : 'hover:bg-surface-muted text-content-secondary'
                    }`}
                  >
                    <FileText className="w-4 h-4 mr-2 shrink-0" />
                    <div className="flex-1 min-w-0">
                      <div className="font-medium truncate">{log.name}</div>
                      <div className="text-xs text-content-muted">{(log.size / 1024 / 1024).toFixed(2)} MB</div>
                    </div>
                  </button>
                ))}
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
                onSearchChange={setSearch}
                host={host}
                onHostChange={setHost}
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
                onRefresh={refetchContent}
                onDownload={handleDownload}
                isLoading={isFetchingContent}
              />

              {logData && logData.skipped_lines > 0 && (
                <div
                  className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm bg-yellow-50 text-yellow-800 border border-yellow-200 dark:bg-yellow-900/30 dark:text-yellow-200 dark:border-yellow-800"
                  data-testid="skipped-lines-warning"
                  role="status"
                >
                  <AlertTriangle className="w-4 h-4 shrink-0" aria-hidden="true" />
                  {t('logs.skippedLines', { count: logData.skipped_lines })}
                </div>
              )}

              <Card className="overflow-hidden">
                {isLoadingContent ? (
                  <div className="p-6 space-y-3">
                    {Array.from({ length: 10 }).map((_, i) => (
                      <Skeleton key={i} className="h-8 w-full" />
                    ))}
                  </div>
                ) : (
                  <div data-testid="log-table">
                    <LogTable
                      logs={logData?.logs || []}
                      isLoading={isLoadingContent}
                      isFetching={isFetchingContent}
                      sortBy={sortBy}
                      sortDir={sortDir}
                      onSortChange={handleSortChange}
                    />
                  </div>
                )}

                {/* Pagination */}
                {logData && logData.total > 0 && (
                  <div className="px-6 py-4 border-t border-border flex flex-col sm:flex-row items-center justify-between gap-4">
                    <div className="text-sm text-content-muted" data-testid="page-info">
                      {t('logs.showingEntries', { from: logData.offset + 1, to: Math.min(logData.offset + limit, logData.total), total: logData.total })}
                    </div>

                    <div className="flex items-center gap-4">
                      <div className="flex items-center gap-2">
                        <Badge variant="outline" size="sm">
                          {t('logs.pageOf', { current: page + 1, total: totalPages })}
                        </Badge>
                      </div>

                      <div className="flex gap-2">
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => setPage((p) => Math.max(0, p - 1))}
                          disabled={page === 0 || isFetchingContent}
                          data-testid="prev-page-button"
                          aria-label="Previous page"
                        >
                          <ChevronLeft className="w-4 h-4" />
                        </Button>
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => setPage((p) => p + 1)}
                          disabled={page >= totalPages - 1 || isFetchingContent}
                          data-testid="next-page-button"
                          aria-label="Next page"
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
            <EmptyState
              icon={<ScrollText className="h-12 w-12" />}
              title={t('logs.noLogSelected')}
              description={t('logs.selectLogDescription')}
            />
          )}
        </div>
      </div>
    </PageShell>
  );
};

export default Logs;
