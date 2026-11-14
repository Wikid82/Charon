import { CheckCircle2, Copy } from 'lucide-react';
import { useState } from 'react';
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
import { AuthKeyPanel } from './AuthKeyPanel';

/** Prints the numeric group ID of the host Docker socket, used as PGID. */
const DOCKER_GID_COMMAND = 'stat -c %g /var/run/docker.sock';

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

  // Each platform's note explains the caveat that applies to its install method.
  const platforms = [
    {
      key: 'docker_compose' as const,
      label: t('hecate.installWizard.tabs.dockerCompose'),
      note: t('hecate.installWizard.dockerGid.noteComposeSystemd'),
    },
    {
      key: 'kubernetes_daemonset' as const,
      label: t('hecate.installWizard.tabs.kubernetes'),
      note: t('hecate.installWizard.dockerGid.noteKubernetes'),
    },
  ] as const;

  type PlatformKey = (typeof platforms)[number]['key'];

  const [copiedGidCommand, setCopiedGidCommand] = useState(false);
  const [copiedSnippet, setCopiedSnippet] = useState<PlatformKey | null>(null);

  // Copy feedback is cleared when the dialog closes. Adjusting during render
  // replaces the former effect.
  const [wasOpen, setWasOpen] = useState(open);
  if (wasOpen !== open) {
    setWasOpen(open);
    if (!open) {
      setCopiedGidCommand(false);
      setCopiedSnippet(null);
    }
  }

  const handleCopyGidCommand = async () => {
    await navigator.clipboard.writeText(DOCKER_GID_COMMAND);
    setCopiedGidCommand(true);
    setTimeout(() => setCopiedGidCommand(false), 3000);
  };

  const handleCopySnippet = async (platform: PlatformKey, text: string) => {
    const filled = text.replace('<AUTH_KEY>', authKey);
    await navigator.clipboard.writeText(filled);
    setCopiedSnippet(platform);
    setTimeout(() => setCopiedSnippet(null), 3000);
  };

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      {/* Capped to the viewport so the footer (Done) stays reachable when the snippet content is tall. */}
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t('hecate.wizard.title', { name: agentName })}</DialogTitle>
          <DialogDescription>{t('hecate.wizard.description')}</DialogDescription>
        </DialogHeader>

        <div className="p-6 space-y-6">
          {/* Auth key section */}
          <AuthKeyPanel authKey={authKey} />

          {/* Docker group ID (PGID) lookup, shared by every platform */}
          <div id="docker-gid-help" className="rounded-lg border border-border p-4 space-y-2">
            <p className="text-sm font-medium text-content-primary">
              {t('hecate.installWizard.dockerGid.title')}
            </p>
            <p className="text-xs text-content-secondary">
              {t('hecate.installWizard.dockerGid.instructions')}
            </p>
            <div className="flex items-center gap-2">
              <code className="font-mono flex-1 bg-surface-base border border-border rounded px-3 py-2 text-content-primary text-sm select-all">
                {DOCKER_GID_COMMAND}
              </code>
              <Button
                variant="outline"
                size="sm"
                onClick={handleCopyGidCommand}
                aria-label={t('hecate.installWizard.dockerGid.copyCommand')}
              >
                {copiedGidCommand ? (
                  <CheckCircle2 className="h-4 w-4 text-success" aria-hidden="true" />
                ) : (
                  <Copy className="h-4 w-4" aria-hidden="true" />
                )}
              </Button>
            </div>
            <p className="text-xs text-content-secondary">
              {t('hecate.installWizard.dockerGid.macHint')}
            </p>
            <p className="text-xs text-success h-4" aria-live="polite" aria-atomic="true">
              {copiedGidCommand ? t('hecate.wizard.copied') : ''}
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

            {platforms.map(({ key, note }) => {
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
                  <p className="text-xs text-content-secondary mt-2">{note}</p>
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
