import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from '../utils/toast'
import { getBackups, createBackup, restoreBackup, deleteBackup, BackupFile } from '../api/backups'
import { getSettings, updateSetting } from '../api/settings'
import { Download, RotateCcw, Plus, Archive, Trash2, Save } from 'lucide-react'
import { useAuth } from '../hooks/useAuth'
import { PageShell } from '../components/layout/PageShell'
import {
  Button,
  Input,
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  Badge,
  DataTable,
  EmptyState,
  SkeletonTable,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  type Column,
} from '../components/ui'

const formatSize = (bytes: number): string => {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

export default function Backups() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const [interval, setInterval] = useState('7')
  const [retention, setRetention] = useState('30')
  const [restoreConfirm, setRestoreConfirm] = useState<BackupFile | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState<BackupFile | null>(null)

  // Fetch Backups
  const { data: backups, isLoading: isLoadingBackups } = useQuery({
    queryKey: ['backups'],
    queryFn: getBackups,
  })

  // Fetch Settings
  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
  })

  // Update local state when settings load
  useState(() => {
    if (settings) {
      if (settings['backup.interval']) setInterval(settings['backup.interval'])
      if (settings['backup.retention']) setRetention(settings['backup.retention'])
    }
  })

  const createMutation = useMutation({
    mutationFn: createBackup,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['backups'] })
      toast.success(t('backups.createSuccess'))
    },
    onError: (error: Error) => {
      toast.error(t('backups.createFailed', { error: error.message }))
    },
  })

  const restoreMutation = useMutation({
    mutationFn: restoreBackup,
    onSuccess: () => {
      setRestoreConfirm(null)
      toast.success(t('backups.restoreSuccess'))
    },
    onError: (error: Error) => {
      toast.error(t('backups.restoreFailed', { error: error.message }))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteBackup,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['backups'] })
      setDeleteConfirm(null)
      toast.success(t('backups.deleteSuccess'))
    },
    onError: (error: Error) => {
      toast.error(t('backups.deleteFailed', { error: error.message }))
    },
  })

  const saveSettingsMutation = useMutation({
    mutationFn: async () => {
      await updateSetting('backup.interval', interval, 'system', 'int')
      await updateSetting('backup.retention', retention, 'system', 'int')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      toast.success(t('backups.settingsSaved'))
    },
    onError: (error: Error) => {
      toast.error(t('backups.settingsFailed', { error: error.message }))
    },
  })

  const handleDownload = (filename: string) => {
    // Trigger download via browser navigation
    // The browser will send the auth cookie automatically
    window.location.href = `/api/v1/backups/${filename}/download`
  }

  const columns: Column<BackupFile>[] = [
    {
      key: 'filename',
      header: t('backups.filename'),
      sortable: true,
      cell: (backup) => (
        <span className="font-medium text-content-primary">{backup.filename}</span>
      ),
    },
    {
      key: 'size',
      header: t('backups.size'),
      sortable: true,
      cell: (backup) => (
        <Badge variant="outline" size="sm">{formatSize(backup.size)}</Badge>
      ),
    },
    {
      key: 'time',
      header: t('backups.createdAt'),
      sortable: true,
      cell: (backup) => (
        <span className="text-content-secondary">
          {new Date(backup.time).toLocaleString()}
        </span>
      ),
    },
    {
      key: 'type',
      header: t('common.type'),
      cell: (backup) => {
        const isAuto = backup.filename.includes('auto')
        return (
          <Badge variant={isAuto ? 'default' : 'primary'} size="sm">
            {isAuto ? t('backups.auto') : t('backups.manual')}
          </Badge>
        )
      },
    },
    {
      key: 'actions',
      header: t('common.actions'),
      cell: (backup) => (
        <div className="flex items-center justify-end gap-2" data-testid="backup-row">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => handleDownload(backup.filename)}
            title={t('backups.download')}
            data-testid="backup-download-btn"
          >
            <Download className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setRestoreConfirm(backup)}
            title={t('backups.restore')}
            disabled={restoreMutation.isPending}
            data-testid="backup-restore-btn"
          >
            <RotateCcw className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDeleteConfirm(backup)}
            title={t('common.delete')}
            disabled={deleteMutation.isPending}
            data-testid="backup-delete-btn"
          >
            <Trash2 className="w-4 h-4 text-error" />
          </Button>
        </div>
      ),
    },
  ]

  // Header actions (visible only to admin users)
  const headerActions = user?.role === 'admin' ? (
    <Button onClick={() => createMutation.mutate()} isLoading={createMutation.isPending}>
      <Plus className="w-4 h-4 mr-2" />
      {t('backups.createBackup')}
    </Button>
  ) : null

  return (
    <PageShell
      title={t('backups.title')}
      description={t('backups.description')}
      actions={headerActions}
    >
      {/* Settings Section */}
      <Card>
        <CardHeader>
          <CardTitle>{t('backups.configuration')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 items-end">
            <Input
              label={t('backups.intervalDays')}
              type="number"
              value={interval}
              onChange={(e) => setInterval(e.target.value)}
              min="1"
            />
            <Input
              label={t('backups.retentionDays')}
              type="number"
              value={retention}
              onChange={(e) => setRetention(e.target.value)}
              min="1"
            />
            <Button
              onClick={() => saveSettingsMutation.mutate()}
              isLoading={saveSettingsMutation.isPending}
            >
              <Save className="w-4 h-4 mr-2" />
              {t('backups.saveSettings')}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Backup List */}
      {isLoadingBackups ? (
        <SkeletonTable rows={5} columns={5} data-testid="loading-skeleton" />
      ) : !backups || backups.length === 0 ? (
        <EmptyState
          icon={<Archive className="h-12 w-12" />}
          title={t('backups.noBackups')}
          description={t('backups.noBackupsDescription')}
          action={{
            label: t('backups.createBackup'),
            onClick: () => createMutation.mutate(),
          }}
          data-testid="empty-state"
        />
      ) : (
        <DataTable
          data={backups}
          columns={columns}
          rowKey={(backup) => backup.filename}
          data-testid="backup-table"
          emptyState={
            <EmptyState
              icon={<Archive className="h-12 w-12" />}
              title={t('backups.noBackups')}
              description={t('backups.noBackupsDescription')}
              action={{
                label: t('backups.createBackup'),
                onClick: () => createMutation.mutate(),
              }}
            />
          }
        />
      )}

      {/* Restore Confirmation Dialog */}
      <Dialog open={restoreConfirm !== null} onOpenChange={() => setRestoreConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('backups.restoreBackup')}</DialogTitle>
          </DialogHeader>
          <p className="text-content-secondary py-4">
            {t('backups.restoreConfirmMessage')}
          </p>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setRestoreConfirm(null)} disabled={restoreMutation.isPending}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="primary"
              onClick={() => restoreConfirm && restoreMutation.mutate(restoreConfirm.filename)}
              isLoading={restoreMutation.isPending}
            >
              {t('backups.restore')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteConfirm !== null} onOpenChange={() => setDeleteConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('backups.deleteBackup')}</DialogTitle>
          </DialogHeader>
          <p className="text-content-secondary py-4">
            {t('backups.deleteConfirmMessage', { filename: deleteConfirm?.filename })}
          </p>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setDeleteConfirm(null)} disabled={deleteMutation.isPending}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="danger"
              onClick={() => deleteConfirm && deleteMutation.mutate(deleteConfirm.filename)}
              isLoading={deleteMutation.isPending}
            >
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  )
}
