import { Eye, EyeOff } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { TunnelStatusBadge } from './TunnelStatusBadge';
import { getTunnelStatus, type TunnelState } from '../../api/hecate';
import { useHecate } from '../../hooks/useHecate';
import { Button } from '../ui/Button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '../ui/Dialog';

interface Props {
  serverName: string;
  onSuccess: (tunnelUuid: string) => void;
  onCancel: () => void;
}

export const CloudflareTunnelWizard = ({ serverName, onSuccess, onCancel }: Props) => {
  const { t } = useTranslation();
  const { createTunnel, isCreating } = useHecate();

  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [token, setToken] = useState('');
  const [showToken, setShowToken] = useState(false);
  const [tunnelUuid, setTunnelUuid] = useState('');
  const [tunnelName, setTunnelName] = useState('');
  const [tunnelState, setTunnelState] = useState<TunnelState>('connecting');
  const [error, setError] = useState<string | null>(null);
  const pollRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (pollRef.current) window.clearInterval(pollRef.current);
    };
  }, []);

  const handleStep1Next = async () => {
    setError(null);
    try {
      const tunnel = await createTunnel({
        name: serverName,
        provider: 'cloudflare',
        credentials: token,
      });
      setTunnelUuid(tunnel.uuid);
      setTunnelName(tunnel.name);
      setStep(2);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create tunnel');
    }
  };

  const handleStep2Next = () => {
    setStep(3);
    pollRef.current = window.setInterval(async () => {
      try {
        const statuses = await getTunnelStatus();
        const found = statuses.find((s) => s.uuid === tunnelUuid);
        if (found) setTunnelState(found.state);
      } catch {
        // ignore transient poll errors
      }
    }, 3000);
  };

  const handleDone = () => {
    if (pollRef.current) window.clearInterval(pollRef.current);
    onSuccess(tunnelUuid);
  };

  const helpTextId = 'cf-token-help';

  return (
    <Dialog open onOpenChange={(isOpen) => !isOpen && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('hecate.cloudflare.wizardTitle')}</DialogTitle>
          <DialogDescription>{t('hecate.cloudflare.wizardDescription')}</DialogDescription>
        </DialogHeader>

        <div className="p-6 space-y-4">
          {error && (
            <div
              role="alert"
              className="text-sm text-red-400 bg-red-900/20 border border-red-500 rounded px-3 py-2"
            >
              {error}
            </div>
          )}

          {step === 1 && (
            <div className="space-y-3">
              <p className="text-sm font-medium text-content-primary">
                {t('hecate.cloudflare.step1Title')}
              </p>
              <div>
                <label htmlFor="cf-token" className="block text-sm text-content-secondary mb-1">
                  {t('hecate.cloudflare.tokenLabel')}
                </label>
                <div className="flex items-center gap-2">
                  <input
                    id="cf-token"
                    type={showToken ? 'text' : 'password'}
                    value={token}
                    onChange={(e) => setToken(e.target.value)}
                    aria-describedby={helpTextId}
                    className="flex-1 bg-surface-base border border-border-primary rounded px-3 py-2 text-content-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                    autoComplete="off"
                  />
                  <button
                    type="button"
                    onClick={() => setShowToken((s) => !s)}
                    aria-label={
                      showToken
                        ? t('hecate.cloudflare.hideToken')
                        : t('hecate.cloudflare.showToken')
                    }
                    className="p-2 rounded border border-border-primary hover:bg-surface-muted focus:outline-none focus:ring-2 focus:ring-brand-500"
                  >
                    {showToken ? (
                      <EyeOff className="h-4 w-4" aria-hidden="true" />
                    ) : (
                      <Eye className="h-4 w-4" aria-hidden="true" />
                    )}
                  </button>
                </div>
                <p id={helpTextId} className="mt-1 text-xs text-content-muted">
                  {t('hecate.cloudflare.tokenHelp')}
                </p>
              </div>
            </div>
          )}

          {step === 2 && (
            <div className="space-y-3">
              <p className="text-sm font-medium text-content-primary">
                {t('hecate.cloudflare.step2Title')}
              </p>
              <div className="flex items-center gap-3">
                <span className="text-sm text-content-secondary">{tunnelName}</span>
                <TunnelStatusBadge state={tunnelState} />
              </div>
            </div>
          )}

          {step === 3 && (
            <div className="space-y-3">
              <p className="text-sm font-medium text-content-primary">
                {t('hecate.cloudflare.step3Title')}
              </p>
              <TunnelStatusBadge state={tunnelState} />
            </div>
          )}
        </div>

        <div className="px-6 pb-6 flex justify-end gap-3">
          <Button variant="outline" onClick={onCancel}>
            {t('common.cancel')}
          </Button>
          {step === 1 && (
            <Button onClick={handleStep1Next} disabled={!token || isCreating}>
              {t('common.next')}
            </Button>
          )}
          {step === 2 && <Button onClick={handleStep2Next}>{t('common.next')}</Button>}
          {step === 3 && (
            <Button onClick={handleDone} disabled={tunnelState !== 'connected'}>
              {t('common.done')}
            </Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
};
