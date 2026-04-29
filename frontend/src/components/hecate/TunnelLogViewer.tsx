import { Pause, Play, Trash2 } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { connectTunnelLogs } from '../../api/hecate';
import { cn } from '../../utils/cn';
import { Button } from '../ui/Button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '../ui/Dialog';

interface LogLine {
  id: number;
  raw: string;
  level: 'error' | 'warn' | 'info' | 'debug' | 'unknown';
  timestamp: string;
}

const MAX_LINES = 500;

function detectLevel(line: string): LogLine['level'] {
  const lower = line.toLowerCase();
  if (lower.includes('error') || lower.includes('fatal') || lower.includes('crit')) return 'error';
  if (lower.includes('warn')) return 'warn';
  if (lower.includes('info')) return 'info';
  if (lower.includes('debug') || lower.includes('trace')) return 'debug';
  return 'unknown';
}

const levelClass: Record<LogLine['level'], string> = {
  error: 'text-red-400',
  warn: 'text-yellow-400',
  info: 'text-content-primary',
  debug: 'text-content-muted',
  unknown: 'text-content-secondary',
};

interface TunnelLogViewerProps {
  serverName: string;
  serverUUID: string;
  open: boolean;
  onClose: () => void;
}

export const TunnelLogViewer = ({
  serverName,
  serverUUID,
  open,
  onClose,
}: TunnelLogViewerProps) => {
  const { t } = useTranslation();
  const [lines, setLines] = useState<LogLine[]>([]);
  const [paused, setPaused] = useState(false);
  const [reconnecting, setReconnecting] = useState(false);
  const [reconnectCount, setReconnectCount] = useState(0);
  const pausedRef = useRef(false);
  const receivedErrorRef = useRef(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const autoScrollRef = useRef(true);
  const counterRef = useRef(0);
  const reconnectTimerRef = useRef<number | null>(null);

  const handleScroll = useCallback(() => {
    if (!containerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    autoScrollRef.current = scrollHeight - scrollTop - clientHeight < 50;
  }, []);

  useEffect(() => {
    if (!open) return;

    receivedErrorRef.current = false;

    const ws = connectTunnelLogs(serverUUID, (raw: string) => {
      if (raw.startsWith('error:')) {
        receivedErrorRef.current = true;
      }
      if (pausedRef.current) return;
      const id = ++counterRef.current;
      const line: LogLine = {
        id,
        raw,
        level: detectLevel(raw),
        timestamp: new Date().toISOString(),
      };
      setLines((prev) => {
        const next = [...prev, line];
        return next.length > MAX_LINES ? next.slice(next.length - MAX_LINES) : next;
      });
    });

    ws.onclose = () => {
      if (!pausedRef.current && open && !receivedErrorRef.current) {
        setReconnecting(true);
        reconnectTimerRef.current = window.setTimeout(() => {
          setReconnecting(false);
          setReconnectCount((c) => c + 1);
        }, 3000);
      }
    };

    ws.onerror = () => {
      ws.close();
    };

    return () => {
      ws.close();
      if (reconnectTimerRef.current) {
        window.clearTimeout(reconnectTimerRef.current);
      }
    };
  }, [open, serverUUID, reconnectCount]);

  useEffect(() => {
    if (paused || !containerRef.current || !autoScrollRef.current) return;
    containerRef.current.scrollTop = containerRef.current.scrollHeight;
  }, [lines, paused]);

  const togglePause = () => {
    const next = !paused;
    pausedRef.current = next;
    setPaused(next);
  };

  const handleClear = () => {
    setLines([]);
  };

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="max-w-3xl h-[80vh] flex flex-col">
        <DialogHeader className="shrink-0">
          <DialogTitle>
            {t('hecate.logs.title', { name: serverName })}
          </DialogTitle>
          <DialogDescription className="sr-only">
            {t('hecate.logs.description', { name: serverName })}
          </DialogDescription>
        </DialogHeader>

        <div className="flex items-center gap-2 px-6 pb-2 shrink-0">
          <Button
            variant="outline"
            size="sm"
            onClick={togglePause}
            aria-pressed={paused}
            aria-label={paused ? t('hecate.logs.resume') : t('hecate.logs.pause')}
          >
            {paused ? (
              <Play className="h-4 w-4 mr-1.5" aria-hidden="true" />
            ) : (
              <Pause className="h-4 w-4 mr-1.5" aria-hidden="true" />
            )}
            {paused ? t('hecate.logs.resume') : t('hecate.logs.pause')}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={handleClear}
            aria-label={t('hecate.logs.clear')}
          >
            <Trash2 className="h-4 w-4 mr-1.5" aria-hidden="true" />
            {t('hecate.logs.clear')}
          </Button>
          <span className="ml-auto text-xs text-content-muted" aria-live="polite">
            {lines.length > 0
              ? t('hecate.logs.lineCount', { count: lines.length })
              : t('hecate.logs.noLogs')}
          </span>
          {reconnecting && (
            <span aria-live="polite" className="text-yellow-500 text-xs">
              {t('hecate.logs.reconnecting')}
            </span>
          )}
        </div>

        <div
          ref={containerRef}
          onScroll={handleScroll}
          role="log"
          aria-live="polite"
          aria-label={t('hecate.logs.title', { name: serverName })}
          className="flex-1 overflow-y-auto bg-gray-950 rounded-lg mx-6 mb-6 p-4 font-mono text-xs space-y-0.5"
        >
          {lines.length === 0 ? (
            <p className="text-content-muted text-center py-8">{t('hecate.logs.noLogs')}</p>
          ) : (
            lines.map((line) => (
              <div
                key={line.id}
                className={cn('whitespace-pre-wrap break-all leading-5', levelClass[line.level])}
              >
                {line.raw}
              </div>
            ))
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
};
