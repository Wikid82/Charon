import React from 'react';
import { CaddyAccessLog } from '../api/logs';
import { format } from 'date-fns';

interface LogTableProps {
  logs: CaddyAccessLog[];
  isLoading: boolean;
}

export const LogTable: React.FC<LogTableProps> = ({ logs, isLoading }) => {
  if (isLoading) {
    return (
      <div className="w-full h-64 flex items-center justify-center text-gray-500">
        Loading logs...
      </div>
    );
  }

  if (!logs || logs.length === 0) {
    return (
      <div className="w-full h-64 flex items-center justify-center text-gray-500">
        No logs found matching criteria.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
        <thead className="bg-gray-50 dark:bg-gray-800">
          <tr>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Time</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Status</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Method</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Host</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Path</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">IP</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Latency</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Message</th>
          </tr>
        </thead>
        <tbody className="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
          {logs.map((log, idx) => (
            <tr key={idx} className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors">
              <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                {format(new Date(log.ts * 1000), 'MMM d HH:mm:ss')}
              </td>
              <td className="px-6 py-4 whitespace-nowrap text-sm">
                {log.status > 0 && (
                  <span className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full
                    ${log.status >= 500 ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' :
                      log.status >= 400 ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200' :
                      log.status >= 300 ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' :
                      'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'}`}>
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
          ))}
        </tbody>
      </table>
    </div>
  );
};
