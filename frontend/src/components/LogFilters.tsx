import React from 'react';
import { Search, Download, RefreshCw } from 'lucide-react';
import { Button } from './ui/Button';

interface LogFiltersProps {
  search: string;
  onSearchChange: (value: string) => void;
  status: string;
  onStatusChange: (value: string) => void;
  level: string;
  onLevelChange: (value: string) => void;
  host: string;
  onHostChange: (value: string) => void;
  onRefresh: () => void;
  onDownload: () => void;
  isLoading: boolean;
}

export const LogFilters: React.FC<LogFiltersProps> = ({
  search,
  onSearchChange,
  status,
  onStatusChange,
  level,
  onLevelChange,
  host,
  onHostChange,
  onRefresh,
  onDownload,
  isLoading
}) => {
  return (
    <div className="flex flex-col md:flex-row gap-4 p-4 bg-white dark:bg-gray-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
      <div className="flex-1 relative">
        <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
          <Search className="h-4 w-4 text-gray-400" />
        </div>
        <input
          type="text"
          placeholder="Search logs..."
          value={search}
          onChange={(e) => onSearchChange(e.target.value)}
          className="block w-full pl-10 rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm dark:bg-gray-700 dark:text-white"
        />
      </div>

      <div className="w-full md:w-48">
        <input
          type="text"
          placeholder="Filter by Host"
          value={host}
          onChange={(e) => onHostChange(e.target.value)}
          className="block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm dark:bg-gray-700 dark:text-white"
        />
      </div>

      <div className="w-full md:w-32">
        <select
          value={level}
          onChange={(e) => onLevelChange(e.target.value)}
          className="block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm dark:bg-gray-700 dark:text-white"
        >
          <option value="">All Levels</option>
          <option value="DEBUG">Debug</option>
          <option value="INFO">Info</option>
          <option value="WARN">Warn</option>
          <option value="ERROR">Error</option>
        </select>
      </div>

      <div className="w-full md:w-32">
        <select
          value={status}
          onChange={(e) => onStatusChange(e.target.value)}
          className="block w-full rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm dark:bg-gray-700 dark:text-white"
        >
          <option value="">All Status</option>
          <option value="2xx">2xx Success</option>
          <option value="3xx">3xx Redirect</option>
          <option value="4xx">4xx Client Error</option>
          <option value="5xx">5xx Server Error</option>
        </select>
      </div>

      <div className="flex gap-2">
        <Button onClick={onRefresh} variant="secondary" size="sm" isLoading={isLoading}>
          <RefreshCw className="w-4 h-4 mr-2" />
          Refresh
        </Button>
        <Button onClick={onDownload} variant="secondary" size="sm">
          <Download className="w-4 h-4 mr-2" />
          Download
        </Button>
      </div>
    </div>
  );
};
