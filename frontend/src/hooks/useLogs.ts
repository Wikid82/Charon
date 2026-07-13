import { keepPreviousData, useMutation, useQuery } from '@tanstack/react-query';
import toast from 'react-hot-toast';
import { useTranslation } from 'react-i18next';

import {
  downloadLog,
  getLogContent,
  getLogs,
  type LogFile,
  type LogFilter,
  type LogResponse,
  type LogSortField,
} from '../api/logs';

/** Query key factory for log queries. */
export const logKeys = {
  all: ['logs'] as const,
  content: (filename: string, filter: LogFilter) => ['logs', filename, filter] as const,
};

/**
 * Hook for fetching the list of available log files.
 * @returns Query result with the LogFile array
 */
export function useLogFiles() {
  return useQuery({
    queryKey: logKeys.all,
    queryFn: getLogs,
  });
}

/**
 * Hook for fetching paginated, filtered, sorted log content.
 * Keeps the previous page rendered while the next one loads so page
 * flips and sort changes don't flash the loading skeleton.
 * @param filename - Selected log file, or null when none selected
 * @param filter - Server-side filter, pagination, and sort options
 * @returns Query result with the LogResponse
 */
export function useLogContent(filename: string | null, filter: LogFilter) {
  return useQuery<LogResponse>({
    queryKey: logKeys.content(filename ?? '', filter),
    queryFn: () => getLogContent(filename!, filter),
    enabled: !!filename,
    placeholderData: keepPreviousData,
  });
}

/**
 * Hook for downloading a log file as a blob.
 * Shows an error toast and keeps the user on the page when the download fails.
 * @returns Mutation taking the log filename
 */
export function useDownloadLog() {
  const { t } = useTranslation();

  return useMutation<void, Error, string>({
    mutationFn: (filename: string) => downloadLog(filename),
    onError: () => {
      toast.error(t('logs.downloadFailed'));
    },
  });
}

export type { LogFile, LogFilter, LogResponse, LogSortField };
