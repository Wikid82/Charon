import { Download, Lock, Plus, RotateCcw, Trash2, Archive } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { BackupEncryptionCard } from '../components/backups/BackupEncryptionCard'
import { BackupScheduleCard } from '../components/backups/BackupScheduleCard'
import { RemoteTargetsCard } from '../components/backups/RemoteTargetsCard'
import { RestoreDialog } from '../components/backups/RestoreDialog'
import { UploadBackupButton } from '../components/backups/UploadBackupButton'
import { PageShell } from '../components/layout/PageShell'
import {
  Button,
  Input,
  Badge,
  DataTable,
  EmptyState,
  SkeletonTable,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  Label,
  Switch,
  type Column,
} from '../components/ui'
import { useBackups, useCreateBackup, useDeleteBackup, useBackupSettingsForm, type BackupFile } from '../hooks/useBackups'
import { useAuth } from '../hooks/useAuth'
import { toast } from '../utils/toast'

const formatSize = (bytes: number): string => {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

const BACKUP_TYPE_KEYS: Record<string, string> = {
  manual: 'manual',
  scheduled: 'scheduled',
  pre_restore: 'preRestore',
  uploaded: 'uploaded',
}

export default function Backups() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'

  const { data: backups, isLoading: isLoadingBackups } = useBackups()
  const settingsForm = useBackupSettingsForm()

  const [restoreTarget, setRestoreTarget] = useState<BackupFile | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState<BackupFile | null>(null)
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [createEncrypt, setCreateEncrypt] = useState(false)
  const [createPassphrase, setCreatePassphrase] = useState('')

  const createMutation = useCreateBackup()
  const deleteMutation = useDeleteBackup()

  const handleOpenCreateDialog = () => {
    setCreateEncrypt(false)
    setCreatePassphrase('')
    setCreateDialogOpen(true)
  }

  const handleCreateConfirm = () => {
    createMutation.mutate(createEncrypt ? { encrypt: true, passphrase: createPassphrase } : undefined, {
      onSuccess: () => {
        toast.success(t('backups.createSuccess'))
        setCreateDialogOpen(false)
      },
      onError: (error: Error) => toast.error(t('backups.createFailed', { error: error.message })),
    })
  }

  const handleDownload = (filename: string) => {
    // Trigger download via browser navigation; the browser sends the auth cookie automatically.
    window.location.href = `/api/v1/backups/${filename}/download`
  }

  const handleDelete = () => {
    if (!deleteConfirm) return
    deleteMutation.mutate(deleteConfirm.filename, {
      onSuccess: () => {
        setDeleteConfirm(null)
        toast.success(t('backups.deleteSuccess'))
      },
      onError: (error: Error) => toast.error(t('backups.deleteFailed', { error: error.message })),
    })
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
        const typeKey = BACKUP_TYPE_KEYS[backup.type ?? 'manual'] ?? 'manual'
        return (
          <Badge data-testid="backup-type-badge" variant={backup.type === 'uploaded' ? 'primary' : 'default'} size="sm">
            {t(`backups.types.${typeKey}`)}
          </Badge>
        )
      },
    },
    {
      key: 'encrypted',
      header: t('backups.encryptedColumn'),
      cell: (backup) =>
        backup.encrypted ? (
          <Lock data-testid="backup-encrypted-icon" className="w-4 h-4 text-content-secondary" aria-label={t('backups.encryption.title')} />
        ) : null,
    },
    {
      key: 'remoteCopies',
      header: t('backups.remoteCopiesColumn'),
      cell: (backup) =>
        backup.remote_copies && backup.remote_copies.length > 0 ? (
          <div className="flex flex-wrap gap-1">
            {backup.remote_copies.map((copy) => (
              <Badge
                key={copy.target_uuid}
                size="sm"
                variant={copy.status === 'uploaded' ? 'success' : copy.status === 'failed' ? 'destructive' : 'outline'}
              >
                {copy.target_name}: {copy.status}
              </Badge>
            ))}
          </div>
        ) : (
          <span className="text-content-muted text-sm">—</span>
        ),
    },
    {
      key: 'actions',
      header: t('common.actions'),
      cell: (backup) =>
        isAdmin ? (
          <div className="flex items-center justify-end gap-2">
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
              onClick={() => setRestoreTarget(backup)}
              title={t('backups.restore')}
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
        ) : null,
    },
  ]

  // Header actions — admin-only (backend gates create/upload/download/restore/delete to admin; spec §3.9).
  const headerActions = isAdmin ? (
    <div className="flex items-center gap-2">
      <UploadBackupButton />
      <Button onClick={handleOpenCreateDialog}>
        <Plus className="w-4 h-4 mr-2" />
        {t('backups.createBackup')}
      </Button>
    </div>
  ) : null

  return (
    <PageShell
      title={t('backups.title')}
      description={t('backups.description')}
      actions={headerActions}
    >
      {isAdmin && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <BackupScheduleCard form={settingsForm} />
          <BackupEncryptionCard form={settingsForm} />
        </div>
      )}

      {isAdmin && <RemoteTargetsCard />}

      {/* Backup List */}
      {isLoadingBackups ? (
        <SkeletonTable rows={5} columns={6} data-testid="loading-skeleton" />
      ) : !backups || backups.length === 0 ? (
        <EmptyState
          icon={<Archive className="h-12 w-12" />}
          title={t('backups.noBackups')}
          description={t('backups.noBackupsDescription')}
          action={isAdmin ? { label: t('backups.createBackup'), onClick: handleOpenCreateDialog } : undefined}
          data-testid="empty-state"
        />
      ) : (
        <DataTable
          data={backups}
          columns={columns}
          rowKey={(backup) => backup.filename}
          rowTestId={() => 'backup-row'}
          data-testid="backup-table"
          emptyState={
            <EmptyState
              icon={<Archive className="h-12 w-12" />}
              title={t('backups.noBackups')}
              description={t('backups.noBackupsDescription')}
              action={isAdmin ? { label: t('backups.createBackup'), onClick: handleOpenCreateDialog } : undefined}
            />
          }
        />
      )}

      {/* Create Backup Dialog */}
      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('backups.createBackup')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 px-6 pb-2">
            <div className="flex items-center gap-3">
              <Switch
                id="backup-create-encrypt"
                data-testid="backup-create-encrypt-toggle"
                checked={createEncrypt}
                onCheckedChange={setCreateEncrypt}
              />
              <Label htmlFor="backup-create-encrypt">{t('backups.encryptThisBackup')}</Label>
            </div>
            {createEncrypt && (
              <Input
                id="backup-create-passphrase"
                data-testid="backup-create-passphrase-input"
                type="password"
                label={t('backups.encryption.passphraseLabel')}
                value={createPassphrase}
                onChange={(e) => setCreatePassphrase(e.target.value)}
                autoComplete="new-password"
              />
            )}
          </div>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setCreateDialogOpen(false)} disabled={createMutation.isPending}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="primary"
              onClick={handleCreateConfirm}
              isLoading={createMutation.isPending}
              disabled={createEncrypt && !createPassphrase}
            >
              {t('common.create')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <RestoreDialog backup={restoreTarget} onClose={() => setRestoreTarget(null)} />

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
              onClick={handleDelete}
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
