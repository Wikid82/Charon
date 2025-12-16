import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from '../utils/toast'
import { getBackups, createBackup, restoreBackup, deleteBackup, BackupFile } from '../api/backups'
import { getSettings, updateSetting } from '../api/settings'
import { Download, RotateCcw, Plus, Archive, Trash2, Save } from 'lucide-react'
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
      toast.success('Backup created successfully')
    },
    onError: (error: Error) => {
      toast.error(`Failed to create backup: ${error.message}`)
    },
  })

  const restoreMutation = useMutation({
    mutationFn: restoreBackup,
    onSuccess: () => {
      setRestoreConfirm(null)
      toast.success('Backup restored successfully. Please restart the container.')
    },
    onError: (error: Error) => {
      toast.error(`Failed to restore backup: ${error.message}`)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteBackup,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['backups'] })
      setDeleteConfirm(null)
      toast.success('Backup deleted successfully')
    },
    onError: (error: Error) => {
      toast.error(`Failed to delete backup: ${error.message}`)
    },
  })

  const saveSettingsMutation = useMutation({
    mutationFn: async () => {
      await updateSetting('backup.interval', interval, 'system', 'int')
      await updateSetting('backup.retention', retention, 'system', 'int')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      toast.success('Backup settings saved')
    },
    onError: (error: Error) => {
      toast.error(`Failed to save settings: ${error.message}`)
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
      header: 'Filename',
      sortable: true,
      cell: (backup) => (
        <span className="font-medium text-content-primary">{backup.filename}</span>
      ),
    },
    {
      key: 'size',
      header: 'Size',
      sortable: true,
      cell: (backup) => (
        <Badge variant="outline" size="sm">{formatSize(backup.size)}</Badge>
      ),
    },
    {
      key: 'time',
      header: 'Created At',
      sortable: true,
      cell: (backup) => (
        <span className="text-content-secondary">
          {new Date(backup.time).toLocaleString()}
        </span>
      ),
    },
    {
      key: 'type',
      header: 'Type',
      cell: (backup) => {
        const isAuto = backup.filename.includes('auto')
        return (
          <Badge variant={isAuto ? 'default' : 'primary'} size="sm">
            {isAuto ? 'Auto' : 'Manual'}
          </Badge>
        )
      },
    },
    {
      key: 'actions',
      header: 'Actions',
      cell: (backup) => (
        <div className="flex items-center justify-end gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => handleDownload(backup.filename)}
            title="Download"
          >
            <Download className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setRestoreConfirm(backup)}
            title="Restore"
            disabled={restoreMutation.isPending}
          >
            <RotateCcw className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDeleteConfirm(backup)}
            title="Delete"
            disabled={deleteMutation.isPending}
          >
            <Trash2 className="w-4 h-4 text-error" />
          </Button>
        </div>
      ),
    },
  ]

  // Header actions
  const headerActions = (
    <Button onClick={() => createMutation.mutate()} isLoading={createMutation.isPending}>
      <Plus className="w-4 h-4 mr-2" />
      Create Backup
    </Button>
  )

  return (
    <PageShell
      title="Backups"
      description="Manage database backups"
      actions={headerActions}
    >
      {/* Settings Section */}
      <Card>
        <CardHeader>
          <CardTitle>Configuration</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 items-end">
            <Input
              label="Backup Interval (Days)"
              type="number"
              value={interval}
              onChange={(e) => setInterval(e.target.value)}
              min="1"
            />
            <Input
              label="Retention Period (Days)"
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
              Save Settings
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Backup List */}
      {isLoadingBackups ? (
        <SkeletonTable rows={5} columns={5} />
      ) : !backups || backups.length === 0 ? (
        <EmptyState
          icon={<Archive className="h-12 w-12" />}
          title="No Backups"
          description="Create your first backup to protect your configuration"
          action={{
            label: 'Create Backup',
            onClick: () => createMutation.mutate(),
          }}
        />
      ) : (
        <DataTable
          data={backups}
          columns={columns}
          rowKey={(backup) => backup.filename}
          emptyState={
            <EmptyState
              icon={<Archive className="h-12 w-12" />}
              title="No Backups"
              description="Create your first backup to protect your configuration"
              action={{
                label: 'Create Backup',
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
            <DialogTitle>Restore Backup</DialogTitle>
          </DialogHeader>
          <p className="text-content-secondary py-4">
            Are you sure you want to restore this backup? Current data will be overwritten.
            You will need to restart the container after restoration.
          </p>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setRestoreConfirm(null)} disabled={restoreMutation.isPending}>
              Cancel
            </Button>
            <Button
              variant="primary"
              onClick={() => restoreConfirm && restoreMutation.mutate(restoreConfirm.filename)}
              isLoading={restoreMutation.isPending}
            >
              Restore
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteConfirm !== null} onOpenChange={() => setDeleteConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Backup</DialogTitle>
          </DialogHeader>
          <p className="text-content-secondary py-4">
            Are you sure you want to delete &quot;{deleteConfirm?.filename}&quot;? This action cannot be undone.
          </p>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setDeleteConfirm(null)} disabled={deleteMutation.isPending}>
              Cancel
            </Button>
            <Button
              variant="danger"
              onClick={() => deleteConfirm && deleteMutation.mutate(deleteConfirm.filename)}
              isLoading={deleteMutation.isPending}
            >
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  )
}
