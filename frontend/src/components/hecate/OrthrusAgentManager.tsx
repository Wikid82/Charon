import { Check, Link2, Pencil, Trash2, X } from 'lucide-react';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { type OrthrusAgent } from '../../api/orthrus';
import { useDeleteAgent, useRenameAgent } from '../../hooks/useOrthrus';
import { AgentProviderAssignDialog } from './AgentProviderAssignDialog';
import { Badge } from '../ui/Badge';
import { Button } from '../ui/Button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '../ui/Dialog';
import { Input } from '../ui/Input';

interface OrthrusAgentManagerProps {
  agents: OrthrusAgent[];
}

const statusVariant = (status: OrthrusAgent['status']): 'success' | 'warning' | 'destructive' => {
  switch (status) {
    case 'online':
      return 'success';
    case 'pending':
      return 'warning';
    default:
      return 'destructive';
  }
};

interface AgentRowProps {
  agent: OrthrusAgent;
  onDelete: (uuid: string, name: string) => void;
  onAssignProvider: (agent: OrthrusAgent) => void;
}

const AgentRow = ({ agent, onDelete, onAssignProvider }: AgentRowProps) => {
  const { t } = useTranslation();
  const { mutate: rename, isPending: isRenaming } = useRenameAgent();
  const [editing, setEditing] = useState(false);
  const [draftName, setDraftName] = useState(agent.name);
  const inputRef = useRef<HTMLInputElement>(null);

  const startEdit = () => {
    setDraftName(agent.name);
    setEditing(true);
    // Focus the input on the next paint after it mounts
    setTimeout(() => inputRef.current?.select(), 0);
  };

  const cancelEdit = () => {
    setDraftName(agent.name);
    setEditing(false);
  };

  const commitEdit = () => {
    const trimmed = draftName.trim();
    if (!trimmed || trimmed === agent.name) {
      cancelEdit();
      return;
    }
    rename(
      { uuid: agent.uuid, name: trimmed },
      { onSettled: () => setEditing(false) },
    );
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') commitEdit();
    if (e.key === 'Escape') cancelEdit();
  };

  return (
    <tr className="border-b border-border last:border-0 hover:bg-surface-subtle transition-colors">
      {/* Name cell — inline editable */}
      <td className="py-3 px-4">
        {editing ? (
          <div className="flex items-center gap-1">
            <Input
              ref={inputRef}
              value={draftName}
              onChange={(e) => setDraftName(e.target.value)}
              onKeyDown={handleKeyDown}
              className="h-7 py-0 text-sm w-40"
              aria-label={t('hecate.agentManager.renameInputLabel', { name: agent.name })}
              disabled={isRenaming}
            />
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0"
              onClick={commitEdit}
              disabled={isRenaming}
              aria-label={t('hecate.agentManager.confirmRename')}
            >
              <Check className="h-3.5 w-3.5 text-success" aria-hidden="true" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-7 p-0"
              onClick={cancelEdit}
              disabled={isRenaming}
              aria-label={t('hecate.agentManager.cancelRename')}
            >
              <X className="h-3.5 w-3.5" aria-hidden="true" />
            </Button>
          </div>
        ) : (
          <button
            type="button"
            onClick={startEdit}
            className="group flex items-center gap-1.5 text-sm font-medium text-content-primary hover:text-brand-500 transition-colors text-left"
            aria-label={t('hecate.agentManager.editNameLabel', { name: agent.name })}
          >
            <span>{agent.name}</span>
            <Pencil
              className="h-3 w-3 opacity-0 group-hover:opacity-60 transition-opacity"
              aria-hidden="true"
            />
          </button>
        )}
      </td>

      {/* UUID */}
      <td className="py-3 px-4">
        <span className="font-mono text-xs text-content-secondary select-all">{agent.uuid}</span>
      </td>

      {/* Status */}
      <td className="py-3 px-4">
        <Badge variant={statusVariant(agent.status)} className="capitalize">
          {t(`hecate.agentManager.status.${agent.status}`)}
        </Badge>
      </td>

      {/* Provider */}
      <td className="py-3 px-4 text-xs text-content-secondary">
        {agent.hecate_tunnel_uuid ? (
          <span className="text-content-primary font-mono">
            {agent.resolved_address ?? agent.device_id ?? '—'}
          </span>
        ) : (
          <span className="text-content-muted italic">
            {t('hecate.agentManager.noProviderAssigned')}
          </span>
        )}
      </td>

      {/* Last seen */}
      <td className="py-3 px-4 text-xs text-content-secondary">
        {agent.last_seen
          ? new Date(agent.last_seen).toLocaleString()
          : t('hecate.agentManager.neverSeen')}
      </td>

      {/* Actions */}
      <td className="py-3 px-4">
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            className="h-7 w-7 p-0 text-content-tertiary hover:text-brand-500"
            onClick={() => onAssignProvider(agent)}
            aria-label={t('hecate.agentManager.assignProvider', { name: agent.name })}
          >
            <Link2 className="h-3.5 w-3.5" aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 w-7 p-0 text-content-tertiary hover:text-destructive"
            onClick={() => onDelete(agent.uuid, agent.name)}
            aria-label={t('hecate.agentManager.deleteLabel', { name: agent.name })}
          >
            <Trash2 className="h-3.5 w-3.5" aria-hidden="true" />
          </Button>
        </div>
      </td>
    </tr>
  );
};

export const OrthrusAgentManager = ({ agents }: OrthrusAgentManagerProps) => {
  const { t } = useTranslation();
  const { mutate: deleteAgent, isPending: isDeleting } = useDeleteAgent();
  const [confirmDelete, setConfirmDelete] = useState<{ uuid: string; name: string } | null>(null);
  const [assignProviderAgent, setAssignProviderAgent] = useState<OrthrusAgent | null>(null);

  const handleDeleteRequest = (uuid: string, name: string) => {
    setConfirmDelete({ uuid, name });
  };

  const handleDeleteConfirm = () => {
    if (!confirmDelete) return;
    deleteAgent(confirmDelete.uuid, {
      onSettled: () => setConfirmDelete(null),
    });
  };

  if (agents.length === 0) {
    return (
      <p className="text-sm text-content-secondary py-4 text-center">
        {t('hecate.agentManager.noAgents')}
      </p>
    );
  }

  return (
    <>
      <div className="overflow-x-auto rounded-lg border border-border">
        <table className="w-full text-sm" role="table" aria-label={t('hecate.agentManager.tableLabel')}>
          <thead>
            <tr className="bg-surface-subtle border-b border-border">
              <th scope="col" className="py-2.5 px-4 text-left text-xs font-medium text-content-secondary">
                {t('hecate.agentManager.colName')}
              </th>
              <th scope="col" className="py-2.5 px-4 text-left text-xs font-medium text-content-secondary">
                {t('hecate.agentManager.colUUID')}
              </th>
              <th scope="col" className="py-2.5 px-4 text-left text-xs font-medium text-content-secondary">
                {t('hecate.agentManager.colStatus')}
              </th>
              <th scope="col" className="py-2.5 px-4 text-left text-xs font-medium text-content-secondary">
                {t('hecate.agentManager.colProvider')}
              </th>
              <th scope="col" className="py-2.5 px-4 text-left text-xs font-medium text-content-secondary">
                {t('hecate.agentManager.colLastSeen')}
              </th>
              <th scope="col" className="py-2.5 px-4 text-left text-xs font-medium text-content-secondary sr-only">
                {t('hecate.agentManager.colActions')}
              </th>
            </tr>
          </thead>
          <tbody>
            {agents.map((agent) => (
              <AgentRow
                key={agent.uuid}
                agent={agent}
                onDelete={handleDeleteRequest}
                onAssignProvider={setAssignProviderAgent}
              />
            ))}
          </tbody>
        </table>
      </div>

      {/* Delete confirmation dialog */}
      <Dialog open={!!confirmDelete} onOpenChange={(open) => !open && setConfirmDelete(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t('hecate.agentManager.deleteTitle')}</DialogTitle>
            <DialogDescription>
              {t('hecate.agentManager.deleteDescription', { name: confirmDelete?.name ?? '' })}
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" onClick={() => setConfirmDelete(null)} disabled={isDeleting}>
              {t('common.cancel')}
            </Button>
            <Button variant="danger" onClick={handleDeleteConfirm} disabled={isDeleting}>
              {t('hecate.agentManager.deleteConfirm')}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
      {/* Assign Provider dialog */}
      {assignProviderAgent && (
        <AgentProviderAssignDialog
          agent={assignProviderAgent}
          open={!!assignProviderAgent}
          onClose={() => setAssignProviderAgent(null)}
        />
      )}
    </>
  );
};
