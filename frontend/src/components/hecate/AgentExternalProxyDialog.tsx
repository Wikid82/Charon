import { AlertTriangle, CheckCircle2, Circle, Copy, XCircle } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

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

interface AgentExternalProxyDialogProps {
  agent: OrthrusAgent;
  open: boolean;
  onClose: () => void;
}

function validatePort(value: number): string | null {
  if (value === 0) return null;
  if (value >= 1 && value <= 1023) return 'Port must be 0 (disabled) or in range 1024–65535.';
  if (value > 65535) return 'Port must be 0 (disabled) or in range 1024–65535.';
  return null;
}

export function AgentExternalProxyDialog({
  agent,
  open,
  onClose,
}: AgentExternalProxyDialogProps) {
  const { t } = useTranslation();
  const { mutate: patch, isPending } = usePatchAgent();

  const [portValue, setPortValue] = useState(agent.external_proxy_port ?? 0);
  const [portError, setPortError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const isOnline = agent.status === 'online';

  const { data: proxyStatus, refetch: refetchStatus } = useAgentProxyStatus(
    agent.uuid,
    open && isOnline,
  );

  useEffect(() => {
    if (open) {
      setPortValue(agent.external_proxy_port ?? 0);
      setPortError(null);
      setCopied(false);
    }
  }, [open, agent.external_proxy_port]);

  const handlePortChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const raw = parseInt(e.target.value, 10);
    const num = isNaN(raw) ? 0 : raw;
    setPortValue(num);
    setPortError(validatePort(num));
  };

  const handleSave = () => {
    const err = validatePort(portValue);
    if (err) {
      setPortError(err);
      return;
    }
    patch(
      { uuid: agent.uuid, req: { external_proxy_port: portValue } },
      {
        onSuccess: () => {
          void refetchStatus();
          onClose();
        },
      },
    );
  };

  const handleCopy = async (text: string) => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 3000);
  };

  const configuredDiffersFromActive =
    proxyStatus?.active &&
    proxyStatus.active_port !== 0 &&
    proxyStatus.active_port !== portValue;

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {t('hecate.externalProxy.title', { name: agent.name })}
          </DialogTitle>
          <DialogDescription>
            {t('hecate.externalProxy.description')}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-5 py-2">
          {/* Port input */}
          <div className="space-y-1.5">
            <label
              htmlFor="external-proxy-port"
              className="block text-sm font-medium text-content-primary"
            >
              {t('hecate.externalProxy.portLabel')}
            </label>
            <input
              id="external-proxy-port"
              type="number"
              min={0}
              max={65535}
              step={1}
              value={portValue}
              onChange={handlePortChange}
              placeholder="0"
              aria-describedby="external-proxy-port-hint external-proxy-port-error"
              aria-invalid={portError !== null}
              className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500 aria-[invalid=true]:border-destructive"
            />
            <p id="external-proxy-port-hint" className="text-xs text-content-muted">
              {t('hecate.externalProxy.portHint')}
            </p>
            {portError && (
              <p
                id="external-proxy-port-error"
                role="alert"
                className="text-xs text-destructive"
              >
                {portError}
              </p>
            )}
          </div>

          {/* Security warning */}
          <div
            role="note"
            className="flex gap-2 rounded-lg border border-warning/30 bg-warning/10 p-3"
          >
            <AlertTriangle
              className="h-4 w-4 shrink-0 text-warning mt-0.5"
              aria-hidden="true"
            />
            <p className="text-xs text-content-secondary">
              {t('hecate.externalProxy.securityWarning')}
            </p>
          </div>

          {/* Live status */}
          {isOnline && proxyStatus && (
            <div
              role="status"
              aria-live="polite"
              aria-label={t('hecate.externalProxy.statusLabel')}
              className="space-y-2"
            >
              <p className="text-sm font-medium text-content-primary">
                {t('hecate.externalProxy.statusHeading')}
              </p>

              {proxyStatus.error ? (
                <div className="flex items-center gap-2" role="alert">
                  <XCircle className="h-4 w-4 text-destructive" aria-hidden="true" />
                  <span className="text-sm text-destructive">{proxyStatus.error}</span>
                </div>
              ) : !proxyStatus.agent_online ? (
                <div className="flex items-center gap-2">
                  <Circle className="h-4 w-4 text-content-muted" aria-hidden="true" />
                  <span className="text-sm text-content-muted">
                    {t('hecate.externalProxy.agentOffline')}
                  </span>
                </div>
              ) : proxyStatus.active ? (
                <div className="flex items-center gap-2 flex-wrap">
                  <CheckCircle2 className="h-4 w-4 shrink-0 text-success" aria-hidden="true" />
                  <span className="text-sm text-success font-medium">
                    {t('hecate.externalProxy.statusActive')}
                  </span>
                  {proxyStatus.connection_string && (
                    <>
                      <code className="text-sm font-mono text-content-primary bg-surface-subtle px-1.5 py-0.5 rounded">
                        {proxyStatus.connection_string}
                      </code>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 px-2"
                        onClick={() => void handleCopy(proxyStatus.connection_string)}
                        aria-label={t('hecate.externalProxy.copyConnectionString')}
                      >
                        {copied ? (
                          <CheckCircle2 className="h-3.5 w-3.5 text-success" aria-hidden="true" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" aria-hidden="true" />
                        )}
                      </Button>
                    </>
                  )}
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <Circle className="h-4 w-4 text-content-muted" aria-hidden="true" />
                  <span className="text-sm text-content-muted">
                    {t('hecate.externalProxy.statusInactive')}
                  </span>
                </div>
              )}

              {configuredDiffersFromActive && (
                <p className="text-xs text-content-secondary italic">
                  {t('hecate.externalProxy.reconnectNotice')}
                </p>
              )}
            </div>
          )}

          {!isOnline && (
            <div
              role="status"
              aria-live="polite"
              className="flex items-center gap-2"
            >
              <Circle className="h-4 w-4 text-content-muted" aria-hidden="true" />
              <span className="text-sm text-content-muted">
                {t('hecate.externalProxy.agentOffline')}
              </span>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isPending}>
            {t('common.cancel')}
          </Button>
          <Button
            onClick={handleSave}
            disabled={isPending || portError !== null}
          >
            {t('common.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
