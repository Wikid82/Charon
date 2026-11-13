import { AlertTriangle, CheckCircle2, Circle, Copy, XCircle } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { externalProxyBindErrorMessage } from '../../api/errorBody';
import { type OrthrusAgent } from '../../api/orthrus';
import { useAgentProxyStatus, useLocalNetworks, usePatchAgent } from '../../hooks/useOrthrus';
import { Button } from '../ui/Button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../ui/Dialog';
import { NativeSelect } from '../ui/NativeSelect';
import {
  bindChoiceFromSetting,
  bindSettingFromChoice,
  isBindChoiceComplete,
  isLoopbackListener,
  type BindChoice,
} from './externalProxyBind';

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

/** Option value of the listen-on select for a choice. Networks use a `net:` prefix so a CIDR cannot collide. */
function selectValueFor(choice: BindChoice): string {
  switch (choice.kind) {
    case 'network':
      return `net:${choice.cidr}`;
    default:
      return choice.kind;
  }
}

/** Choice picked in the listen-on select. A custom choice keeps any address already typed. */
function choiceFromSelectValue(value: string, current: BindChoice): BindChoice {
  if (value.startsWith('net:')) return { kind: 'network', cidr: value.slice(4), custom: '' };
  if (value === 'custom') {
    return { kind: 'custom', cidr: '', custom: current.kind === 'custom' ? current.custom : '' };
  }
  if (value === 'all') return { kind: 'all', cidr: '', custom: '' };
  return { kind: 'auto', cidr: '', custom: '' };
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
  const [bindChoice, setBindChoice] = useState<BindChoice>(() =>
    bindChoiceFromSetting(agent.external_proxy_bind ?? ''),
  );
  const [saveError, setSaveError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const isOnline = agent.status === 'online';

  const { data: proxyStatus, refetch: refetchStatus } = useAgentProxyStatus(
    agent.uuid,
    open && isOnline,
  );
  const { data: localNetworks } = useLocalNetworks(open);

  // Re-seed on open or when the stored port or listen setting changes. Adjusting during
  // render replaces the former effect; closing alone never reseeds.
  const storedBind = agent.external_proxy_bind ?? '';
  const [seededFor, setSeededFor] = useState({
    open,
    port: agent.external_proxy_port,
    bind: storedBind,
  });
  if (
    seededFor.open !== open ||
    seededFor.port !== agent.external_proxy_port ||
    seededFor.bind !== storedBind
  ) {
    setSeededFor({ open, port: agent.external_proxy_port, bind: storedBind });
    if (open) {
      setPortValue(agent.external_proxy_port ?? 0);
      setPortError(null);
      setBindChoice(bindChoiceFromSetting(storedBind));
      setSaveError(null);
      setCopied(false);
    }
  }

  const handlePortChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const raw = parseInt(e.target.value, 10);
    const num = isNaN(raw) ? 0 : raw;
    setPortValue(num);
    setPortError(validatePort(num));
  };

  const handleBindChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    setBindChoice((current) => choiceFromSelectValue(e.target.value, current));
    setSaveError(null);
  };

  const handleSave = () => {
    const err = validatePort(portValue);
    if (err) {
      setPortError(err);
      return;
    }
    if (!isBindChoiceComplete(bindChoice)) return;
    patch(
      {
        uuid: agent.uuid,
        req: {
          external_proxy_port: portValue,
          external_proxy_bind: bindSettingFromChoice(bindChoice),
        },
      },
      {
        onSuccess: () => {
          void refetchStatus();
          onClose();
        },
        onError: (saveErr) => {
          setSaveError(
            externalProxyBindErrorMessage(saveErr, t) ?? t('hecate.externalProxy.saveError'),
          );
        },
      },
    );
  };

  const handleCopy = async (text: string) => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 3000);
  };

  // The address the saved setting will listen on, used for the passive notes.
  const autoAddress = localNetworks?.auto.address ?? '';
  const autoFellBack = localNetworks?.auto.source === 'loopback_fallback';
  const effectiveBind =
    bindChoice.kind === 'auto' ? autoAddress : bindSettingFromChoice(bindChoice);
  const listensOnAllNetworks = bindChoice.kind === 'all';
  const listensOnLoopback =
    bindChoice.kind === 'auto' ? autoFellBack || isLoopbackListener(autoAddress) : isLoopbackListener(effectiveBind);

  // Networks to offer. A stored subnet that is no longer detected stays selectable, so saving does not rewrite it.
  const networkOptions = localNetworks?.networks ?? [];
  const storedNetworkMissing =
    bindChoice.kind === 'network' && !networkOptions.some((n) => n.cidr === bindChoice.cidr);

  const configuredDiffersFromActive =
    proxyStatus?.active &&
    proxyStatus.active_port !== 0 &&
    proxyStatus.active_port !== portValue;
  const bindDiffersFromActive =
    proxyStatus !== undefined && proxyStatus.configured_bind !== proxyStatus.active_bind;

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

          {/* Listen-on input */}
          <div className="space-y-1.5">
            <label
              htmlFor="external-proxy-bind"
              className="block text-sm font-medium text-content-primary"
            >
              {t('hecate.externalProxy.bindLabel')}
            </label>
            <NativeSelect
              id="external-proxy-bind"
              value={selectValueFor(bindChoice)}
              onChange={handleBindChange}
              aria-describedby="external-proxy-bind-hint"
            >
              <option value="auto">{t('hecate.externalProxy.bindAuto')}</option>
              {networkOptions.map((network) => (
                <option key={network.cidr} value={`net:${network.cidr}`}>
                  {`${network.interface} · ${network.address} (${network.cidr})`}
                </option>
              ))}
              {storedNetworkMissing && (
                <option value={`net:${bindChoice.cidr}`}>{bindChoice.cidr}</option>
              )}
              <option value="all">{t('hecate.externalProxy.bindAll')}</option>
              <option value="custom">{t('hecate.externalProxy.bindCustom')}</option>
            </NativeSelect>
            <p id="external-proxy-bind-hint" className="text-xs text-content-muted">
              {bindChoice.kind === 'auto' && autoAddress
                ? t('hecate.externalProxy.bindAutoResolved', { address: autoAddress })
                : t('hecate.externalProxy.bindHint')}
            </p>
            {bindChoice.kind === 'custom' && (
              <>
                <label
                  htmlFor="external-proxy-bind-custom"
                  className="block text-sm font-medium text-content-primary"
                >
                  {t('hecate.externalProxy.bindCustomLabel')}
                </label>
                <input
                  id="external-proxy-bind-custom"
                  type="text"
                  maxLength={64}
                  value={bindChoice.custom}
                  onChange={(e) => {
                    setBindChoice({ kind: 'custom', cidr: '', custom: e.target.value });
                    setSaveError(null);
                  }}
                  placeholder={t('hecate.externalProxy.bindCustomPlaceholder')}
                  aria-invalid={!isBindChoiceComplete(bindChoice)}
                  className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 font-mono text-sm text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500 aria-[invalid=true]:border-destructive"
                />
              </>
            )}
            {listensOnLoopback && (
              <p role="note" className="text-xs text-content-secondary">
                {t('hecate.externalProxy.loopbackNote')}
              </p>
            )}
          </div>

          {/* Security warning: the all-networks line replaces the generic one */}
          <div
            role="note"
            className="flex gap-2 rounded-lg border border-warning/30 bg-warning/10 p-3"
          >
            <AlertTriangle
              className="h-4 w-4 shrink-0 text-warning mt-0.5"
              aria-hidden="true"
            />
            <p className="text-xs text-content-secondary">
              {listensOnAllNetworks
                ? t('hecate.externalProxy.bindAllNote')
                : t('hecate.externalProxy.securityWarning')}
            </p>
          </div>

          {saveError && (
            <p role="alert" className="text-xs text-destructive">
              {saveError}
            </p>
          )}

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
                  {proxyStatus.bind_address && (
                    <span className="text-xs font-mono text-content-secondary">
                      {t('hecate.externalProxy.bindListening', { address: proxyStatus.bind_address })}
                    </span>
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

              {proxyStatus.bind_fallback && (
                <p className="text-xs text-content-secondary">
                  {t('hecate.externalProxy.fallbackNote')}
                </p>
              )}

              {proxyStatus.active && isLoopbackListener(proxyStatus.bind_address) && (
                <p className="text-xs text-content-secondary">
                  {t('hecate.externalProxy.loopbackNote')}
                </p>
              )}

              {configuredDiffersFromActive && (
                <p className="text-xs text-content-secondary italic">
                  {t('hecate.externalProxy.reconnectNotice')}
                </p>
              )}

              {bindDiffersFromActive && (
                <p className="text-xs text-content-secondary italic">
                  {t('hecate.externalProxy.bindReconnectNotice')}
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
            disabled={isPending || portError !== null || !isBindChoiceComplete(bindChoice)}
          >
            {t('common.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
