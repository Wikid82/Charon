import { CloudOff, Pencil, Plus, Trash2, Zap } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useRemoteTargets, useDeleteRemoteTarget, useTestRemoteTarget, type RemoteTarget } from '../../hooks/useRemoteTargets'
import { RemoteTargetFormDialog } from './RemoteTargetFormDialog'
import {
  Badge,
  Button,
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  EmptyState,
} from '../ui'
import { toast } from '../../utils/toast'

const STATUS_VARIANT: Record<RemoteTarget['last_test_status'], 'success' | 'destructive' | 'outline'> = {
  ok: 'success',
  failed: 'destructive',
  never: 'outline',
}

/**
 * Remote storage target list with status badges + test-connection button
 * (spec §3.3.2, §3.7, §3.8). Composes `RemoteTargetFormDialog` for
 * create/edit.
 */
export function RemoteTargetsCard() {
  const { t } = useTranslation()
  const { data: targets, isLoading } = useRemoteTargets()
  const deleteMutation = useDeleteRemoteTarget()
  const testMutation = useTestRemoteTarget()

  const [editingTarget, setEditingTarget] = useState<RemoteTarget | null | undefined>(undefined)
  const [deletingTarget, setDeletingTarget] = useState<RemoteTarget | null>(null)

  const handleTest = (target: RemoteTarget) => {
    testMutation.mutate(target.uuid, {
      onSuccess: (result) => {
        if (result.success) {
          toast.success(result.message || t('backups.remoteTargets.testSuccess'))
        } else {
          toast.error(result.message || t('backups.remoteTargets.testFailed'))
        }
      },
      onError: (error: Error) => toast.error(error.message),
    })
  }

  const handleDelete = () => {
    if (!deletingTarget) return
    deleteMutation.mutate(deletingTarget.uuid, {
      onSuccess: () => setDeletingTarget(null),
      onError: (error: Error) => toast.error(error.message),
    })
  }

  return (
    <Card data-testid="backup-remote-targets-card">
      <CardHeader className="flex flex-row items-center justify-between space-y-0">
        <CardTitle>{t('backups.remoteTargets.title')}</CardTitle>
        <Button size="sm" onClick={() => setEditingTarget(null)}>
          <Plus className="w-4 h-4 mr-2" />
          {t('backups.remoteTargets.add')}
        </Button>
      </CardHeader>
      <CardContent>
        {!isLoading && (!targets || targets.length === 0) ? (
          <EmptyState
            icon={<CloudOff className="h-10 w-10" />}
            title={t('backups.remoteTargets.emptyTitle')}
            description={t('backups.remoteTargets.emptyDescription')}
            data-testid="backup-remote-targets-empty-state"
            action={{ label: t('backups.remoteTargets.add'), onClick: () => setEditingTarget(null) }}
          />
        ) : (
          <ul className="divide-y divide-border">
            {(targets ?? []).map((target) => (
              <li
                key={target.uuid}
                data-testid="backup-remote-target-row"
                className="flex items-center justify-between gap-4 py-3"
              >
                <div>
                  <p className="font-medium text-content-primary">{target.name}</p>
                  <p className="text-sm text-content-secondary">{target.type.toUpperCase()}</p>
                </div>
                <div className="flex items-center gap-2">
                  <Badge data-testid="backup-remote-target-status-badge" variant={STATUS_VARIANT[target.last_test_status]}>
                    {t(`backups.remoteTargets.status.${target.last_test_status}`)}
                  </Badge>
                  <Button
                    variant="ghost"
                    size="sm"
                    data-testid="backup-remote-target-test-btn"
                    onClick={() => handleTest(target)}
                    isLoading={testMutation.isPending && testMutation.variables === target.uuid}
                    title={t('backups.remoteTargets.test')}
                  >
                    <Zap className="w-4 h-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    data-testid="backup-remote-target-edit-btn"
                    onClick={() => setEditingTarget(target)}
                    title={t('common.edit')}
                  >
                    <Pencil className="w-4 h-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    data-testid="backup-remote-target-delete-btn"
                    onClick={() => setDeletingTarget(target)}
                    title={t('common.delete')}
                  >
                    <Trash2 className="w-4 h-4 text-error" />
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>

      <RemoteTargetFormDialog
        open={editingTarget !== undefined}
        target={editingTarget}
        onClose={() => setEditingTarget(undefined)}
      />

      <Dialog open={deletingTarget !== null} onOpenChange={(open) => !open && setDeletingTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('backups.remoteTargets.deleteTitle')}</DialogTitle>
          </DialogHeader>
          <p className="px-6 text-content-secondary">
            {t('backups.remoteTargets.deleteConfirm', { name: deletingTarget?.name })}
          </p>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setDeletingTarget(null)} disabled={deleteMutation.isPending}>
              {t('common.cancel')}
            </Button>
            <Button variant="danger" onClick={handleDelete} isLoading={deleteMutation.isPending}>
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}
