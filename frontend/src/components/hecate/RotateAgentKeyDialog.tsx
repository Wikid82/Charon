import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { type OrthrusAgent } from '../../api/orthrus';
import { useRotateAgentKey } from '../../hooks/useOrthrus';
import { Button } from '../ui/Button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '../ui/Dialog';
import { AuthKeyPanel } from './AuthKeyPanel';

interface RotateAgentKeyDialogProps {
  agent: OrthrusAgent;
  open: boolean;
  onClose: () => void;
}

/**
 * Two steps, both built into the page: a confirmation that rotating disconnects
 * the agent, then the new key shown once. The key lives only in this component,
 * so closing the dialog discards it.
 */
export const RotateAgentKeyDialog = ({ agent, open, onClose }: RotateAgentKeyDialogProps) => {
  const { t } = useTranslation();
  const { mutateAsync: rotate, isPending } = useRotateAgentKey();
  const [authKey, setAuthKey] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  const handleConfirm = async () => {
    setFailed(false);
    try {
      const response = await rotate(agent.uuid);
      setAuthKey(response.auth_key);
    } catch {
      setFailed(true);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="max-w-lg">
        {authKey === null ? (
          <>
            <DialogHeader>
              <DialogTitle>{t('hecate.agentManager.rotateTitle')}</DialogTitle>
              <DialogDescription>{t('hecate.agentManager.rotateDescription')}</DialogDescription>
            </DialogHeader>
            <div className="p-6 pt-2 space-y-4">
              {failed && (
                <p role="alert" className="text-sm text-destructive">
                  {t('hecate.agentManager.rotateError')}
                </p>
              )}
              <div className="flex justify-end gap-2">
                <Button variant="outline" onClick={onClose} disabled={isPending}>
                  {t('common.cancel')}
                </Button>
                <Button variant="danger" onClick={handleConfirm} disabled={isPending}>
                  {t('hecate.agentManager.rotateConfirm')}
                </Button>
              </div>
            </div>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{t('hecate.agentManager.rotateSuccessTitle', { name: agent.name })}</DialogTitle>
              <DialogDescription>{t('hecate.agentManager.rotateSuccessDescription')}</DialogDescription>
            </DialogHeader>
            <div className="p-6 pt-2 space-y-4">
              <AuthKeyPanel authKey={authKey} />
              <div className="flex justify-end">
                <Button onClick={onClose}>{t('hecate.agentManager.rotateDone')}</Button>
              </div>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
};
