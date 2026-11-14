import { CheckCircle2, Copy } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { Button } from '../ui/Button';

/** How long the copy confirmation stays visible. */
const COPY_FEEDBACK_MS = 3000;

interface AuthKeyPanelProps {
  authKey: string;
}

/**
 * Shows an agent auth key that Charon reveals only once, with a copy button and
 * a warning. Used by the install wizard and the key rotation dialog. Copy
 * feedback lives in this component, so it resets when the parent unmounts.
 */
export const AuthKeyPanel = ({ authKey }: AuthKeyPanelProps) => {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(authKey);
    setCopied(true);
    setTimeout(() => setCopied(false), COPY_FEEDBACK_MS);
  };

  return (
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
          onClick={handleCopy}
          aria-label={t('hecate.wizard.copyAuthKey')}
        >
          {copied ? (
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
        {copied ? t('hecate.wizard.copied') : ''}
      </p>
    </div>
  );
};
