import { format } from 'date-fns';
import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react';
import React from 'react';
import { useTranslation } from 'react-i18next';

import { type CaddyAccessLog, type LogSortField } from '../api/logs';

type SortDirection = 'asc' | 'desc';

interface LogTableProps {
  logs: CaddyAccessLog[];
  isLoading: boolean;
  /** True while a background refetch is in flight (previous page still shown). */
  isFetching?: boolean;
  sortBy: LogSortField;
  sortDir: SortDirection;
  onSortChange: (field: LogSortField) => void;
}

const HEADER_CELL_CLASS =
  'px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 tracking-wider';

/** Badge color classes per log level (unknown levels fall back to gray). */
const LEVEL_BADGE_CLASSES: Record<string, string> = {
  error: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
  warn: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
  info: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  debug: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200',
};

interface SortableHeaderProps {
  field: LogSortField;
  label: string;
  sortBy: LogSortField;
  sortDir: SortDirection;
  onSortChange: (field: LogSortField) => void;
}

/**
 * Clickable column header for server-side sorting. The header keeps the
 * plain label as its accessible name (button text only), exposes the active
 * sort via aria-sort on the th, and shows a direction icon.
 */
const SortableHeader: React.FC<SortableHeaderProps> = ({ field, label, sortBy, sortDir, onSortChange }) => {
  const isActive = sortBy === field;
  const ariaSort = isActive ? (sortDir === 'asc' ? 'ascending' : 'descending') : undefined;
  const Icon = isActive ? (sortDir === 'asc' ? ArrowUp : ArrowDown) : ArrowUpDown;

  return (
    <th scope="col" className={HEADER_CELL_CLASS} aria-sort={ariaSort}>
      <button
        type="button"
        onClick={() => onSortChange(field)}
        data-testid={`sort-header-${field}`}
        className={`inline-flex items-center gap-1 hover:text-gray-700 dark:hover:text-gray-200 transition-colors ${
          isActive ? 'text-gray-900 dark:text-white' : ''
        }`}
      >
        {label}
        <Icon className="w-3.5 h-3.5" aria-hidden="true" />
      </button>
    </th>
  );
};

export const LogTable: React.FC<LogTableProps> = ({
  logs,
  isLoading,
  isFetching = false,
  sortBy,
  sortDir,
  onSortChange,
}) => {
  const { t } = useTranslation();

  if (isLoading) {
    return (
      <div className="w-full h-64 flex items-center justify-center text-gray-500">
        {t('logs.loading')}
      </div>
    );
  }

  if (!logs || logs.length === 0) {
    return (
      <div className="w-full h-64 flex items-center justify-center text-gray-500">
        {t('logs.noLogsFound')}
      </div>
    );
  }

  const sortProps = { sortBy, sortDir, onSortChange };

  return (
    <div className="overflow-x-auto">
      <table
        className="min-w-full divide-y divide-gray-200 dark:divide-gray-700"
        aria-busy={isFetching}
      >
        <thead className="bg-gray-50 dark:bg-gray-800">
          <tr>
            <SortableHeader field="ts" label={t('logs.columnTime')} {...sortProps} />
            <SortableHeader field="level" label={t('logs.columnLevel')} {...sortProps} />
            <SortableHeader field="status" label={t('logs.columnStatus')} {...sortProps} />
            <SortableHeader field="method" label={t('logs.columnMethod')} {...sortProps} />
            <th scope="col" className={HEADER_CELL_CLASS}>{t('logs.columnHost')}</th>
            <SortableHeader field="uri" label={t('logs.columnPath')} {...sortProps} />
            <th scope="col" className={HEADER_CELL_CLASS}>{t('logs.columnIp')}</th>
            <th scope="col" className={HEADER_CELL_CLASS}>{t('logs.columnLatency')}</th>
            <th scope="col" className={HEADER_CELL_CLASS}>{t('logs.columnMessage')}</th>
          </tr>
        </thead>
        <tbody
          className={`bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700 transition-opacity ${
            isFetching ? 'opacity-50' : ''
          }`}
        >
          {logs.map((log, idx) => {
            // Check if this is a structured access log or a plain text system log
            const isAccessLog = log.status > 0 || (log.request && log.request.method);

            if (!isAccessLog) {
              return (
                <tr key={idx} className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    {format(new Date(log.ts * 1000), 'MMM d HH:mm:ss')}
                  </td>
                  <td colSpan={8} className="px-6 py-4 text-sm text-gray-900 dark:text-white font-mono whitespace-pre-wrap break-all">
                    {log.msg}
                  </td>
                </tr>
              );
            }

            return (
            <tr key={idx} className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
              <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                {format(new Date(log.ts * 1000), 'MMM d HH:mm:ss')}
              </td>
              <td className="px-6 py-4 whitespace-nowrap text-sm">
                {log.level && (
                  <span
                    className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full uppercase ${
                      LEVEL_BADGE_CLASSES[log.level.toLowerCase()] ?? LEVEL_BADGE_CLASSES.debug
                    }`}
                    data-testid={`level-${log.level.toLowerCase()}`}
                  >
                    {log.level}
                  </span>
                )}
              </td>
              <td className="px-6 py-4 whitespace-nowrap text-sm">
                {log.status > 0 && (
                  <span
                    className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full
                    ${log.status >= 500 ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' :
                      log.status >= 400 ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200' :
                      log.status >= 300 ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' :
                      'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'}`}
                    data-testid={`status-${log.status}`}
                  >
                    {log.status}
                  </span>
                )}
              </td>
              <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-white">
                {log.request?.method}
              </td>
              <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                {log.request?.host}
              </td>
              <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400 max-w-xs truncate" title={log.request?.uri}>
                {log.request?.uri}
              </td>
              <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                {log.request?.remote_ip}
              </td>
              <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                {log.duration > 0 ? (log.duration * 1000).toFixed(2) + 'ms' : ''}
              </td>
              <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400 max-w-xs truncate" title={log.msg}>
                {log.msg}
              </td>
            </tr>
          )})}
        </tbody>
      </table>
    </div>
  );
};
