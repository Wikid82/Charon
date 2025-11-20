import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getLogs, getLogContent } from '../api/logs';
import { getSettings, updateSetting } from '../api/settings';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { toast } from '../components/Toast';
import { Loader2, RefreshCw, FileText, Save } from 'lucide-react';

const Logs: React.FC = () => {
  const [selectedLog, setSelectedLog] = useState<string | null>(null);
  const [lineCount, setLineCount] = useState(100);
  const [logLevel, setLogLevel] = useState('INFO');
  const queryClient = useQueryClient();

  const { data: logs, isLoading: isLoadingLogs, refetch: refetchLogs } = useQuery({
    queryKey: ['logs'],
    queryFn: getLogs,
  });

  const { data: logContent, isLoading: isLoadingContent, refetch: refetchContent } = useQuery({
    queryKey: ['logContent', selectedLog, lineCount],
    queryFn: () => selectedLog ? getLogContent(selectedLog, lineCount) : Promise.resolve({ lines: [] }),
    enabled: !!selectedLog,
  });

  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
  });

  // Update local state when settings load
  React.useEffect(() => {
    if (settings && settings['logging.level']) {
      setLogLevel(settings['logging.level']);
    }
  }, [settings]);

  const saveSettingsMutation = useMutation({
    mutationFn: async () => {
      await updateSetting('logging.level', logLevel, 'caddy', 'string');
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] });
      toast.success('Log level saved');
    },
    onError: (error: any) => {
      toast.error(`Failed to save log level: ${error.message}`);
    },
  });

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">System Logs</h1>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <select
              value={logLevel}
              onChange={(e) => setLogLevel(e.target.value)}
              className="block w-32 rounded-md border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm dark:bg-gray-700 dark:text-white"
            >
              <option value="DEBUG">DEBUG</option>
              <option value="INFO">INFO</option>
              <option value="WARN">WARN</option>
              <option value="ERROR">ERROR</option>
            </select>
            <Button
              onClick={() => saveSettingsMutation.mutate()}
              isLoading={saveSettingsMutation.isPending}
              size="sm"
            >
              <Save className="w-4 h-4" />
            </Button>
          </div>
          <Button onClick={() => { refetchLogs(); if (selectedLog) refetchContent(); }} variant="secondary" size="sm">
            <RefreshCw className="w-4 h-4 mr-2" />
            Refresh
          </Button>
        </div>
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
                    onClick={() => setSelectedLog(log.name)}
                    className={`w-full text-left px-3 py-2 rounded-md text-sm transition-colors flex items-center ${
                      selectedLog === log.name
                        ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 border border-blue-200 dark:border-blue-800'
                        : 'hover:bg-gray-50 dark:hover:bg-gray-800 text-gray-700 dark:text-gray-300'
                    }`}
                  >
                    <FileText className="w-4 h-4 mr-2 opacity-70" />
                    <div className="flex-1 truncate">
                      <div className="font-medium">{log.name}</div>
                      <div className="text-xs opacity-70">{(log.size / 1024).toFixed(1)} KB</div>
                    </div>
                  </button>
                ))}
                {logs?.length === 0 && (
                  <p className="text-sm text-gray-500 text-center py-4">No logs found</p>
                )}
              </div>
            )}
          </Card>
        </div>

        {/* Log Viewer */}
        <div className="md:col-span-3">
          <Card className="p-0 overflow-hidden flex flex-col h-[600px]">
            <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center bg-gray-50 dark:bg-gray-800/50">
              <h2 className="font-semibold text-gray-900 dark:text-white">
                {selectedLog ? selectedLog : 'Select a log file'}
              </h2>
              {selectedLog && (
                <div className="flex items-center space-x-2">
                  <span className="text-sm text-gray-500">Lines:</span>
                  <select
                    value={lineCount}
                    onChange={(e) => setLineCount(Number(e.target.value))}
                    className="text-sm border-gray-300 rounded-md shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white"
                  >
                    <option value={50}>50</option>
                    <option value={100}>100</option>
                    <option value={500}>500</option>
                    <option value={1000}>1000</option>
                  </select>
                </div>
              )}
            </div>

            <div className="flex-1 overflow-auto p-4 bg-gray-900 text-gray-100 font-mono text-xs">
              {isLoadingContent ? (
                <div className="flex justify-center items-center h-full">
                  <Loader2 className="w-8 h-8 animate-spin text-gray-500" />
                </div>
              ) : selectedLog ? (
                logContent?.lines && logContent.lines.length > 0 ? (
                  logContent.lines.map((line, i) => (
                    <div key={i} className="whitespace-pre-wrap border-b border-gray-800/50 py-0.5 hover:bg-gray-800/50">
                      {line}
                    </div>
                  ))
                ) : (
                  <div className="text-gray-500 italic text-center mt-10">File is empty</div>
                )
              ) : (
                <div className="flex flex-col items-center justify-center h-full text-gray-500">
                  <FileText className="w-12 h-12 mb-2 opacity-20" />
                  <p>Select a log file from the list to view its contents</p>
                </div>
              )}
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
};

export default Logs;
