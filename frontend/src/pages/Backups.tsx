import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { toast } from '../utils/toast'
import { getBackups, createBackup, restoreBackup, deleteBackup, BackupFile } from '../api/backups'
import { getSettings, updateSetting } from '../api/settings'
import { Loader2, Download, RotateCcw, Plus, Archive, Trash2, Save } from 'lucide-react'

const formatSize = (bytes: number): string => {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

export default function Backups() {
  const queryClient = useQueryClient()
  const [interval, setInterval] = useState('7')
  const [retention, setRetention] = useState('30')

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

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
        <Archive className="w-8 h-8" />
        Backups
      </h1>

      {/* Settings Section */}
      <Card className="p-6">
        <h2 className="text-lg font-semibold mb-4 text-gray-900 dark:text-white">Configuration</h2>
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
            className="mb-0.5"
          >
            <Save className="w-4 h-4 mr-2" />
            Save Settings
          </Button>
        </div>
      </Card>

      {/* Actions */}
      <div className="flex justify-end">
        <Button onClick={() => createMutation.mutate()} isLoading={createMutation.isPending}>
          <Plus className="w-4 h-4 mr-2" />
          Create Backup
        </Button>
      </div>

      {/* List */}
      <Card className="overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="bg-gray-50 dark:bg-gray-800 text-gray-500 dark:text-gray-400">
              <tr>
                <th className="px-6 py-3 font-medium">Filename</th>
                <th className="px-6 py-3 font-medium">Size</th>
                <th className="px-6 py-3 font-medium">Created At</th>
                <th className="px-6 py-3 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {isLoadingBackups ? (
                <tr>
                  <td colSpan={4} className="px-6 py-8 text-center">
                    <Loader2 className="w-6 h-6 animate-spin mx-auto text-blue-500" />
                  </td>
                </tr>
              ) : backups?.length === 0 ? (
                <tr>
                  <td colSpan={4} className="px-6 py-8 text-center text-gray-500">
                    No backups found
                  </td>
                </tr>
              ) : (
                backups?.map((backup: BackupFile) => (
                  <tr key={backup.filename} className="hover:bg-gray-50 dark:hover:bg-gray-800/50">
                    <td className="px-6 py-4 font-medium text-gray-900 dark:text-white">
                      {backup.filename}
                    </td>
                    <td className="px-6 py-4 text-gray-500 dark:text-gray-400">
                      {formatSize(backup.size)}
                    </td>
                    <td className="px-6 py-4 text-gray-500 dark:text-gray-400">
                      {new Date(backup.time).toLocaleString()}
                    </td>
                    <td className="px-6 py-4 text-right space-x-2">
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => handleDownload(backup.filename)}
                        title="Download"
                      >
                        <Download className="w-4 h-4" />
                      </Button>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => {
                          if (confirm('Are you sure you want to restore this backup? Current data will be overwritten.')) {
                            restoreMutation.mutate(backup.filename)
                          }
                        }}
                        isLoading={restoreMutation.isPending}
                        title="Restore"
                      >
                        <RotateCcw className="w-4 h-4" />
                      </Button>
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => {
                          if (confirm('Are you sure you want to delete this backup?')) {
                            deleteMutation.mutate(backup.filename)
                          }
                        }}
                        isLoading={deleteMutation.isPending}
                        title="Delete"
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  )
}
