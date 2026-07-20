import { AlertTriangle } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';

import { type OrthrusAgent } from '../../api/orthrus';
import { useAgentProxyStatus, usePatchAgent } from '../../hooks/useOrthrus';
import { Button } from '../ui/Button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../ui/Dialog';
import { Input } from '../ui/Input';
import { Switch } from '../ui/Switch';

interface AgentWriteModeDialogProps {
  agent: OrthrusAgent;
  open: boolean;
  onClose: () => void;
}

const WRITE_MODE_OPERATION_KEYS = ['pull', 'start', 'stop', 'restart', 'remove', 'recreate'] as const;

/**
 * Write-mode dialog is intentionally separate from AgentExternalProxyDialog
 * rather than a section bolted onto it — the port dialog governs a
 * transport-layer setting (is there a TCP port bound at all), this dialog
 * governs a permissions-escalation setting (what that tunnel is allowed to
 * do). Keeping them as two components with minimal, independent local state
 * is more testable than one component tracking two loosely-related
 * "configured vs. active" pairs. See docs/plans/current_spec.md Section 3.5.1.
 */
export function AgentWriteModeDialog({ agent, open, onClose }: AgentWriteModeDialogProps) {
  const { t } = useTranslation();
  const { mutate: patch, isPending } = usePatchAgent();

  const isOnline = agent.status === 'online';

  const { data: proxyStatus } = useAgentProxyStatus(agent.uuid, open && isOnline);

  const [desiredEnabled, setDesiredEnabled] = useState(agent.write_enabled);
  const [confirmText, setConfirmText] = useState('');

  useEffect(() => {
    if (open) {
      setDesiredEnabled(agent.write_enabled);
      setConfirmText('');
    }
  }, [open, agent.write_enabled]);

  // Only turning the toggle ON (from an off starting point) requires the
  // typed-name confirmation gate — disabling is strictly safety-increasing
  // and needs no extra friction. This is why the check compares against
  // agent.write_enabled (the value the dialog opened with), not merely
  // "is desiredEnabled true": flipping on then back off within the same
  // dialog session must not require typing anything either.
  const requiresConfirmation = desiredEnabled && !agent.write_enabled;
  const confirmMatches = confirmText.trim() === agent.name;
  const canSave = !requiresConfirmation || confirmMatches;

  const handleSave = () => {
    if (!canSave) return;
    patch(
      { uuid: agent.uuid, req: { write_enabled: desiredEnabled } },
      { onSuccess: onClose },
    );
  };

  const configuredDiffersFromActive =
    proxyStatus !== undefined && proxyStatus.active_write_enabled !== agent.write_enabled;

  const auditLogHref = `/audit-logs?resource_uuid=${encodeURIComponent(agent.uuid)}&event_category=orthrus_write`;

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('hecate.writeMode.title', { name: agent.name })}</DialogTitle>
          <DialogDescription>{t('hecate.writeMode.description')}</DialogDescription>
        </DialogHeader>

        <div className="space-y-5 py-2">
          {/* Toggle */}
          <div className="flex items-center justify-between">
            <span id="write-mode-toggle-label" className="text-sm font-medium text-content-primary">
              {t('hecate.writeMode.toggleLabel')}
            </span>
            <Switch
              role="switch"
              aria-labelledby="write-mode-toggle-label"
              checked={desiredEnabled}
              onCheckedChange={(checked) => {
                setDesiredEnabled(checked);
                if (!checked) setConfirmText('');
              }}
              disabled={isPending}
            />
          </div>

          {/* Typed-name confirmation — only shown when enabling from off */}
          {requiresConfirmation && (
            <div className="space-y-1.5">
              <label
                htmlFor="write-mode-confirm"
                className="block text-sm font-medium text-content-primary"
              >
                {t('hecate.writeMode.confirmPrompt', { name: agent.name })}
              </label>
              <Input
                id="write-mode-confirm"
                type="text"
                value={confirmText}
                onChange={(e) => setConfirmText(e.target.value)}
                placeholder={t('hecate.writeMode.confirmPlaceholder')}
                disabled={isPending}
                aria-describedby="write-mode-confirm-hint"
              />
              <p id="write-mode-confirm-hint" className="text-xs text-content-muted">
                {t('hecate.writeMode.confirmHint')}
              </p>
            </div>
          )}

          {/* Security warning — distinct from AgentExternalProxyDialog's
              network-exposure warning: different DOM node, different copy,
              describes a permissions escalation rather than a network
              reachability change. */}
          <div
            role="note"
            className="flex gap-2 rounded-lg border border-warning/30 bg-warning/10 p-3"
          >
            <AlertTriangle className="h-4 w-4 shrink-0 text-warning mt-0.5" aria-hidden="true" />
            <p className="text-xs text-content-secondary">{t('hecate.writeMode.securityWarning')}</p>
          </div>

          {/* Fixed, non-editable list of permitted operations — shown only
              while the toggle is (or is about to be) on. */}
          {desiredEnabled && (
            <div className="space-y-1.5">
              <p className="text-sm font-medium text-content-primary">
                {t('hecate.writeMode.permittedOperationsHeading')}
              </p>
              <ul className="list-disc list-inside text-xs text-content-secondary space-y-0.5">
                {WRITE_MODE_OPERATION_KEYS.map((key) => (
                  <li key={key}>{t(`hecate.writeMode.operations.${key}`)}</li>
                ))}
              </ul>
            </div>
          )}

          {configuredDiffersFromActive && (
            <p className="text-xs text-content-secondary italic">
              {t('hecate.writeMode.reconnectNotice')}
            </p>
          )}

          <Link
            to={auditLogHref}
            className="text-xs text-brand-500 hover:underline inline-block"
          >
            {t('hecate.writeMode.viewAuditLog')}
          </Link>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isPending}>
            {t('common.cancel')}
          </Button>
          <Button onClick={handleSave} disabled={isPending || !canSave}>
            {desiredEnabled ? t('common.save') : t('hecate.writeMode.disableConfirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
