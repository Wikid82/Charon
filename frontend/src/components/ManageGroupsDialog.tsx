import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { ProxyGroupForm } from './ProxyGroupForm';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Button,
} from './ui';
import { useProxyGroups, useDeleteProxyGroup } from '../hooks/useProxyGroups';

import type { ProxyGroup } from '../api/proxyGroups';

interface ManageGroupsDialogProps {
  open: boolean;
  onClose: () => void;
}

export function ManageGroupsDialog({ open, onClose }: ManageGroupsDialogProps) {
  const { t } = useTranslation();
  const { data: groups = [] } = useProxyGroups();
  const deleteGroup = useDeleteProxyGroup();

  const [showForm, setShowForm] = useState(false);
  const [editingGroup, setEditingGroup] = useState<ProxyGroup | undefined>();
  const [groupToDelete, setGroupToDelete] = useState<ProxyGroup | null>(null);

  return (
    <>
      <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('proxyGroups.manageGroups')}</DialogTitle>
            <DialogDescription>
              {t('proxyGroups.manageGroupsDescription')}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-2 py-2 max-h-80 overflow-y-auto">
            {groups.length === 0 ? (
              <p className="text-sm text-content-muted text-center py-4">
                {t('proxyGroups.noGroups')}
              </p>
            ) : (
              groups.map((group) => (
                <div
                  key={group.uuid}
                  className="flex items-center justify-between p-2 rounded-md border border-border"
                >
                  <div className="flex items-center gap-2">
                    <span
                      className="w-3 h-3 rounded-full shrink-0"
                      style={{ backgroundColor: group.color }}
                      aria-hidden="true"
                    />
                    <span className="text-sm font-medium">{group.name}</span>
                  </div>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label={`${t('common.edit')} ${group.name}`}
                      onClick={() => {
                        setEditingGroup(group);
                        setShowForm(true);
                      }}
                    >
                      {t('common.edit')}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-error hover:text-error hover:bg-error/10"
                      aria-label={`${t('common.delete')} ${group.name}`}
                      onClick={() => setGroupToDelete(group)}
                    >
                      {t('common.delete')}
                    </Button>
                  </div>
                </div>
              ))
            )}
          </div>

          <DialogFooter>
            <Button
              type="button"
              onClick={() => {
                setEditingGroup(undefined);
                setShowForm(true);
              }}
            >
              {t('proxyGroups.createGroup')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ProxyGroupForm
        open={showForm}
        onClose={() => {
          setShowForm(false);
          setEditingGroup(undefined);
        }}
        group={editingGroup}
      />

      <Dialog open={!!groupToDelete} onOpenChange={(o) => !o && setGroupToDelete(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('proxyGroups.deleteGroup')}</DialogTitle>
            <DialogDescription>{t('proxyGroups.deleteGroupConfirm')}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setGroupToDelete(null)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="danger"
              isLoading={deleteGroup.isPending}
              onClick={async () => {
                if (!groupToDelete) return;
                try {
                  await deleteGroup.mutateAsync(groupToDelete.uuid);
                } catch {
                  // errors handled by hook toast
                }
                setGroupToDelete(null);
              }}
            >
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
