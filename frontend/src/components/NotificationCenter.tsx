import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Bell, X, Info, AlertTriangle, AlertCircle, CheckCircle, ExternalLink } from 'lucide-react';
import { useEffect, useState, type FC } from 'react';
import { useTranslation } from 'react-i18next';

import { getNotifications, markNotificationRead, markAllNotificationsRead, type ActiveAlert } from '../api/system';
import { filterKnownAlerts, getAlertPresentation } from '../constants/alerts';
import { GITHUB_BUG_REPORT_URL } from '../constants/links';
import { useActiveAlerts } from '../hooks/useActiveAlerts';
import { useSystemUpdates } from '../hooks/useSystemUpdates';

const getIcon = (type: string) => {
  switch (type) {
    case 'success': return <CheckCircle className="w-5 h-5 text-green-500" />;
    case 'warning': return <AlertTriangle className="w-5 h-5 text-yellow-500" />;
    case 'error': return <AlertCircle className="w-5 h-5 text-red-500" />;
    default: return <Info className="w-5 h-5 text-blue-500" />;
  }
};

/** One server alert: headline, body with its affected lists, docs link and, when registered, an issue link. */
const ActiveAlertItem: FC<{ alert: ActiveAlert }> = ({ alert }) => {
  const { t, i18n } = useTranslation();
  const presentation = getAlertPresentation(alert.key);
  if (!presentation) return null;

  const listFormatter = new Intl.ListFormat(i18n.language, { style: 'long', type: 'conjunction' });
  const lists = listFormatter.format(alert.affected_lists ?? []);
  const message = t(presentation.messageKey(alert), {
    count: alert.affected_lists_total ?? 0,
    lists,
  });
  const linkKeyPrefix = `notificationCenter.alerts.${alert.key}`;

  return (
    <div className="flex items-start px-4 py-3 border-b dark:border-gray-700 bg-red-50 dark:bg-red-900/10">
      <div className="shrink-0 mt-0.5">{getIcon(alert.severity)}</div>
      <div className="ml-3 w-0 flex-1">
        <p className="text-sm font-medium text-gray-900 dark:text-white">{t(presentation.titleKey)}</p>
        <p className="mt-1 text-sm text-gray-700 dark:text-gray-300">{message}</p>
        <a
          href={presentation.docsUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="mt-1 text-sm text-blue-600 hover:text-blue-500 dark:text-blue-400 flex items-center"
        >
          {t(`${linkKeyPrefix}.docsLink`)} <ExternalLink className="w-3 h-3 ml-1" aria-hidden="true" />
        </a>
        {presentation.showIssueLink && (
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {t(`${linkKeyPrefix}.issueHint`)}{' '}
            <a
              href={GITHUB_BUG_REPORT_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 hover:text-blue-500 dark:text-blue-400 inline-flex items-center"
            >
              {t(`${linkKeyPrefix}.issueLink`)} <ExternalLink className="w-3 h-3 ml-1" aria-hidden="true" />
            </a>
          </p>
        )}
        <p className="mt-1 text-xs text-gray-400">{t('notificationCenter.alerts.clearsItself')}</p>
      </div>
    </div>
  );
};

const NotificationCenter: FC = () => {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = useState(false);
  const queryClient = useQueryClient();

  const { data: notifications = [] } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => getNotifications(true),
    refetchInterval: 30000, // Poll every 30s
  });

  // Alerts are not notifications: "Mark all read" and the per-item dismiss never touch them.
  const { data: serverAlerts = [] } = useActiveAlerts();
  const alerts = filterKnownAlerts(serverAlerts);

  const { data: updateInfo } = useSystemUpdates();

  const markReadMutation = useMutation({
    mutationFn: markNotificationRead,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
  });

  const markAllReadMutation = useMutation({
    mutationFn: markAllNotificationsRead,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
  });

  // Close the dropdown on Escape — the backdrop below is aria-hidden (click-to-dismiss
  // only), so keyboard users need this document-level listener to dismiss it too.
  useEffect(() => {
    if (!isOpen) return;
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsOpen(false);
      }
    };
    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [isOpen]);

  const hasUpdate = Boolean(updateInfo?.available);
  const unreadCount = notifications.length + (hasUpdate ? 1 : 0) + alerts.length;
  const hasCritical =
    notifications.some(n => n.type === 'error') || alerts.some(a => a.severity === 'error');
  const hasWarning =
    notifications.some(n => n.type === 'warning') || alerts.some(a => a.severity === 'warning') || hasUpdate;
  const isEmpty = unreadCount === 0;

  const getBellColor = () => {
    if (hasCritical) return 'text-red-500 hover:text-red-600';
    if (hasWarning) return 'text-yellow-500 hover:text-yellow-600';
    return 'text-green-500 hover:text-green-600';
  };

  const bellLabel = isEmpty
    ? t('notificationCenter.title')
    : t('notificationCenter.bellLabel', { count: unreadCount });

  return (
    <div className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className={`relative p-2 focus:outline-none transition-colors ${getBellColor()}`}
        aria-label={bellLabel}
      >
        <Bell className="w-6 h-6" />
        {unreadCount > 0 && (
          <span className="absolute top-0 right-0 inline-flex items-center justify-center px-2 py-1 text-xs font-bold leading-none text-white transform translate-x-1/4 -translate-y-1/4 bg-red-600 rounded-full">
            {unreadCount}
          </span>
        )}
      </button>

      {isOpen && (
        <>
          <div
            data-testid="notification-backdrop"
            className="fixed inset-0 z-10"
            aria-hidden="true"
            onClick={() => setIsOpen(false)}
          ></div>
          <div className="absolute right-0 z-20 w-80 mt-2 overflow-hidden bg-white rounded-md shadow-lg dark:bg-gray-800 ring-1 ring-black ring-opacity-5">
            <div className="flex items-center justify-between px-4 py-2 border-b dark:border-gray-700">
              <h3 className="text-sm font-medium text-gray-900 dark:text-white">{t('notificationCenter.title')}</h3>
              {notifications.length > 0 && (
                <button
                  onClick={() => markAllReadMutation.mutate()}
                  className="text-xs text-blue-600 hover:text-blue-500 dark:text-blue-400"
                >
                  {t('notificationCenter.markAllRead')}
                </button>
              )}
            </div>
            <div className="max-h-96 overflow-y-auto">
              {alerts.length > 0 && (
                <section aria-label={t('notificationCenter.alerts.label')}>
                  {alerts.map((alert) => (
                    <ActiveAlertItem key={alert.key} alert={alert} />
                  ))}
                </section>
              )}

              {/* Update Notification */}
              {updateInfo?.available && (
                <div className="flex items-start px-4 py-3 border-b dark:border-gray-700 bg-yellow-50 dark:bg-yellow-900/10 hover:bg-yellow-100 dark:hover:bg-yellow-900/20">
                  <div className="shrink-0 mt-0.5">
                    <AlertCircle className="w-5 h-5 text-yellow-500" />
                  </div>
                  <div className="ml-3 w-0 flex-1">
                    <p className="text-sm font-medium text-gray-900 dark:text-white">
                      {t('notificationCenter.updateAvailable', { version: updateInfo.latest_version })}
                    </p>
                    {updateInfo.changelog_url && (
                      <a
                        href={updateInfo.changelog_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="mt-1 text-sm text-blue-600 hover:text-blue-500 dark:text-blue-400 flex items-center"
                      >
                        {t('notificationCenter.viewChangelog')} <ExternalLink className="w-3 h-3 ml-1" />
                      </a>
                    )}
                  </div>
                </div>
              )}

              {isEmpty ? (
                <div className="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
                  {t('notificationCenter.empty')}
                </div>
              ) : (
                notifications.map((notification) => (
                  <div
                    key={notification.id}
                    className="flex items-start px-4 py-3 border-b dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700"
                  >
                    <div className="shrink-0 mt-0.5">
                      {getIcon(notification.type)}
                    </div>
                    <div className="ml-3 w-0 flex-1">
                      <p className="text-sm font-medium text-gray-900 dark:text-white">
                        {notification.title}
                      </p>
                      <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                        {notification.message}
                      </p>
                      <p className="mt-1 text-xs text-gray-400">
                        {new Date(notification.created_at).toLocaleString()}
                      </p>
                    </div>
                    <div className="ml-4 shrink-0 flex">
                      <button
                        onClick={() => markReadMutation.mutate(notification.id)}
                        className="bg-white dark:bg-gray-800 rounded-md inline-flex text-gray-400 hover:text-gray-500 focus:outline-none"
                      >
                        <span className="sr-only">{t('notificationCenter.close')}</span>
                        <X className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        </>
      )}
    </div>
  );
};

export default NotificationCenter;
