import { CheckCircle2, Copy } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { type InstallSnippets } from '../../api/orthrus';
import { Button } from '../ui/Button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '../ui/Dialog';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/Tabs';

interface OrthrusInstallWizardProps {
  agentName: string;
  agentUUID: string;
  authKey: string;
  snippets: InstallSnippets;
  open: boolean;
  onClose: () => void;
}

export const OrthrusInstallWizard = ({
  agentName,
  authKey,
  snippets,
  open,
  onClose,
}: OrthrusInstallWizardProps) => {
  const { t } = useTranslation();

  const platforms = [
    { key: 'docker_compose' as const, label: t('hecate.installWizard.tabs.dockerCompose') },
    { key: 'systemd' as const, label: t('hecate.installWizard.tabs.systemd') },
    { key: 'tarball' as const, label: t('hecate.installWizard.tabs.tarball') },
    { key: 'homebrew' as const, label: t('hecate.installWizard.tabs.homebrew') },
    { key: 'kubernetes_daemonset' as const, label: t('hecate.installWizard.tabs.kubernetes') },
  ] as const;

  type PlatformKey = (typeof platforms)[number]['key'];

  const [copiedKey, setCopiedKey] = useState(false);
  const [copiedSnippet, setCopiedSnippet] = useState<PlatformKey | null>(null);

  useEffect(() => {
    if (!open) {
      // Auth key is cleared from component state when the dialog closes.
      setCopiedKey(false);
      setCopiedSnippet(null);
    }
  }, [open]);

  const handleCopyKey = async () => {
    await navigator.clipboard.writeText(authKey);
    setCopiedKey(true);
    setTimeout(() => setCopiedKey(false), 3000);
  };

  const handleCopySnippet = async (platform: PlatformKey, text: string) => {
    const filled = text.replace('<AUTH_KEY>', authKey);
    await navigator.clipboard.writeText(filled);
    setCopiedSnippet(platform);
    setTimeout(() => setCopiedSnippet(null), 3000);
  };

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t('hecate.wizard.title', { name: agentName })}</DialogTitle>
          <DialogDescription>{t('hecate.wizard.description')}</DialogDescription>
        </DialogHeader>

        <div className="p-6 space-y-6">
          {/* Auth key section */}
          <div id="auth-key-warning" className="rounded-lg border border-warning/30 bg-warning/10 p-4 space-y-3">
            <p className="text-sm font-medium text-content-primary">
              {t('hecate.wizard.authKeyLabel')}
            </p>
            <p className="text-xs text-content-secondary">{t('hecate.wizard.authKeyWarning')}</p>
            <div className="flex items-center gap-2">
              <input
                readOnly
                id="auth-key-input"
                value={authKey}
                aria-label={t('hecate.wizard.authKeyLabel')}
                aria-describedby="auth-key-warning"
                className="font-mono w-full bg-surface-base border border-border rounded px-3 py-2 text-content-primary text-sm select-all focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
              <Button
                variant="outline"
                size="sm"
                onClick={handleCopyKey}
                aria-label={t('hecate.wizard.copyAuthKey')}
              >
                {copiedKey ? (
                  <CheckCircle2 className="h-4 w-4 text-success" aria-hidden="true" />
                ) : (
                  <Copy className="h-4 w-4" aria-hidden="true" />
                )}
              </Button>
            </div>
            <p
              className="text-xs text-success h-4"
              aria-live="polite"
              aria-atomic="true"
            >
              {copiedKey ? t('hecate.wizard.copied') : ''}
            </p>
          </div>

          {/* Platform tabs */}
          <Tabs defaultValue="docker_compose">
            <TabsList className="w-full">
              {platforms.map(({ key, label }) => (
                <TabsTrigger key={key} value={key} className="flex-1 text-xs">
                  {label}
                </TabsTrigger>
              ))}
            </TabsList>

            {platforms.map(({ key }) => {
              const snippet = snippets[key];
              return (
                <TabsContent key={key} value={key}>
                  <div className="relative">
                    <pre className="rounded-lg bg-surface-base border border-border p-4 text-xs font-mono text-content-primary overflow-x-auto whitespace-pre-wrap break-all max-h-64">
                      {snippet}
                    </pre>
                    <Button
                      variant="outline"
                      size="sm"
                      className="absolute top-2 right-2"
                      onClick={() => handleCopySnippet(key, snippet)}
                      aria-label={t('hecate.wizard.copySnippet')}
                    >
                      {copiedSnippet === key ? (
                        <CheckCircle2 className="h-4 w-4 text-success" aria-hidden="true" />
                      ) : (
                        <Copy className="h-4 w-4" aria-hidden="true" />
                      )}
                    </Button>
                  </div>
                  <p
                    className="text-xs text-success h-4 mt-1"
                    aria-live="polite"
                    aria-atomic="true"
                  >
                    {copiedSnippet === key ? t('hecate.wizard.copied') : ''}
                  </p>
                </TabsContent>
              );
            })}
          </Tabs>

          {/* Troubleshooting */}
          <details className="text-sm">
            <summary className="cursor-pointer font-medium text-content-secondary hover:text-content-primary transition-colors">
              {t('hecate.wizard.troubleshooting')}
            </summary>
            <div className="mt-3 space-y-2 text-xs text-content-secondary pl-4 border-l border-border">
              <p>{t('hecate.wizard.troubleshootingHint1')}</p>
              <p>{t('hecate.wizard.troubleshootingHint2')}</p>
              <p>{t('hecate.wizard.troubleshootingHint3')}</p>
            </div>
          </details>
        </div>

        <div className="px-6 pb-6 flex justify-end">
          <Button onClick={onClose}>{t('hecate.wizard.done')}</Button>
        </div>
      </DialogContent>
    </Dialog>
  );
};
