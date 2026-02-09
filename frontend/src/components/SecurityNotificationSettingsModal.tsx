import { useEffect, useState } from 'react';
import { X } from 'lucide-react';
import { Button } from './ui/Button';
import { Switch } from './ui/Switch';
import {
  useSecurityNotificationSettings,
  useUpdateSecurityNotificationSettings,
} from '../hooks/useNotifications';

interface SecurityNotificationSettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export function SecurityNotificationSettingsModal({
  isOpen,
  onClose,
}: SecurityNotificationSettingsModalProps) {
  const { data: settings, isLoading } = useSecurityNotificationSettings();
  const updateMutation = useUpdateSecurityNotificationSettings();

  const [formData, setFormData] = useState({
    enabled: false,
    min_log_level: 'warn',
    notify_waf_blocks: true,
    notify_acl_denials: true,
    notify_rate_limit_hits: true,
    webhook_url: '',
    email_recipients: '',
  });

  useEffect(() => {
    if (settings) {
      setFormData({
        enabled: settings.enabled,
        min_log_level: settings.min_log_level,
        notify_waf_blocks: settings.notify_waf_blocks,
        notify_acl_denials: settings.notify_acl_denials,
        notify_rate_limit_hits: settings.notify_rate_limit_hits,
        webhook_url: settings.webhook_url || '',
        email_recipients: settings.email_recipients || '',
      });
    }
  }, [settings]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    updateMutation.mutate(formData, {
      onSuccess: () => {
        onClose();
      },
    });
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-gray-700">
          <h2 className="text-xl font-semibold text-white">Security Notification Settings</h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-white transition-colors"
            aria-label="Close"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Body */}
        <form onSubmit={handleSubmit} className="p-6 space-y-6">
          {isLoading && (
            <div className="text-center text-gray-400">Loading settings...</div>
          )}

          {!isLoading && (
            <>
              {/* Master Toggle */}
              <div className="flex items-center justify-between">
                <div>
                  <label htmlFor="enable-notifications" className="text-sm font-medium text-white">Enable Notifications</label>
                  <p className="text-xs text-gray-400 mt-1">
                    Receive alerts when security events occur
                  </p>
                </div>
                <Switch
                  id="enable-notifications"
                  checked={formData.enabled}
                  onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
                />
              </div>

              {/* Minimum Log Level */}
              <div>
                <label htmlFor="min-log-level" className="block text-sm font-medium text-white mb-2">
                  Minimum Log Level
                </label>
                <select
                  id="min-log-level"
                  value={formData.min_log_level}
                  onChange={(e) => setFormData({ ...formData, min_log_level: e.target.value })}
                  disabled={!formData.enabled}
                  className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500 disabled:opacity-50"
                >
                  <option value="debug">Debug (All logs)</option>
                  <option value="info">Info</option>
                  <option value="warn">Warning</option>
                  <option value="error">Error</option>
                  <option value="fatal">Fatal (Critical only)</option>
                </select>
                <p className="text-xs text-gray-400 mt-1">
                  Only logs at this level or higher will trigger notifications
                </p>
              </div>

              {/* Event Type Filters */}
              <div className="space-y-3">
                <h3 className="text-sm font-semibold text-white">Notify On:</h3>

                <div className="flex items-center justify-between">
                  <div>
                    <label htmlFor="notify-waf" className="text-sm text-white">WAF Blocks</label>
                    <p className="text-xs text-gray-400">
                      When the Web Application Firewall blocks a request
                    </p>
                  </div>
                  <Switch
                    id="notify-waf"
                    checked={formData.notify_waf_blocks}
                    onChange={(e) =>
                      setFormData({ ...formData, notify_waf_blocks: e.target.checked })
                    }
                    disabled={!formData.enabled}
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <label htmlFor="notify-acl" className="text-sm text-white">ACL Denials</label>
                    <p className="text-xs text-gray-400">
                      When an IP is denied by Access Control Lists
                    </p>
                  </div>
                  <Switch
                    id="notify-acl"
                    checked={formData.notify_acl_denials}
                    onChange={(e) =>
                      setFormData({ ...formData, notify_acl_denials: e.target.checked })
                    }
                    disabled={!formData.enabled}
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <label htmlFor="notify-rate-limit" className="text-sm text-white">Rate Limit Hits</label>
                    <p className="text-xs text-gray-400">
                      When a client exceeds rate limiting thresholds
                    </p>
                  </div>
                  <Switch
                    id="notify-rate-limit"
                    checked={formData.notify_rate_limit_hits}
                    onChange={(e) =>
                      setFormData({ ...formData, notify_rate_limit_hits: e.target.checked })
                    }
                    disabled={!formData.enabled}
                  />
                </div>
              </div>

              {/* Webhook URL (optional, for future use) */}
              <div>
                <label className="block text-sm font-medium text-white mb-2">
                  Webhook URL (Optional)
                </label>
                <input
                  type="url"
                  value={formData.webhook_url}
                  onChange={(e) => setFormData({ ...formData, webhook_url: e.target.value })}
                  placeholder="https://your-webhook-endpoint.com/alert"
                  disabled={!formData.enabled}
                  className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:border-blue-500 disabled:opacity-50"
                />
                <p className="text-xs text-gray-400 mt-1">
                  POST requests will be sent to this URL when events occur
                </p>
              </div>

              {/* Email Recipients (optional, for future use) */}
              <div>
                <label className="block text-sm font-medium text-white mb-2">
                  Email Recipients (Optional)
                </label>
                <input
                  type="text"
                  value={formData.email_recipients}
                  onChange={(e) => setFormData({ ...formData, email_recipients: e.target.value })}
                  placeholder="admin@example.com, security@example.com"
                  disabled={!formData.enabled}
                  className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white placeholder-gray-400 focus:outline-none focus:border-blue-500 disabled:opacity-50"
                />
                <p className="text-xs text-gray-400 mt-1">
                  Comma-separated email addresses
                </p>
              </div>
            </>
          )}

          {/* Footer */}
          <div className="flex justify-end gap-3 pt-4 border-t border-gray-700">
            <Button variant="secondary" onClick={onClose} type="button">
              Cancel
            </Button>
            <Button
              variant="primary"
              type="submit"
              isLoading={updateMutation.isPending}
              disabled={isLoading}
            >
              Save Settings
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
