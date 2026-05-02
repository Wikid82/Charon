import {
  KeyRound,
  Pencil,
  Play,
  Plus,
  RotateCcw,
  ScrollText,
  Square,
  Trash2,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { type TunnelConfig } from '../api/hecate'
import { HecateTunnelForm } from '../components/hecate/HecateTunnelForm'
import { TunnelLogViewer } from '../components/hecate/TunnelLogViewer'
import { TunnelStatusBadge } from '../components/hecate/TunnelStatusBadge'
import { PageShell } from '../components/layout/PageShell'
import {
  Alert,
  Badge,
  Button,
  DataTable,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  Label,
  SkeletonTable,
  type Column,
} from '../components/ui'
import { useHecate } from '../hooks/useHecate'

export default function HecateTunnels() {
  const { t } = useTranslation()
  const {
    tunnels,
    loadingTunnels,
    error,
    getStatus,
    startTunnel,
    stopTunnel,
    deleteTunnel,
    rotateCredentials,
    isStarting,
    isStopping,
    isDeleting,
    isRotating,
  } = useHecate()

  const [formOpen, setFormOpen] = useState(false)
  const [editingTunnel, setEditingTunnel] = useState<TunnelConfig | undefined>()
  const [logsTunnel, setLogsTunnel] = useState<TunnelConfig | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<TunnelConfig | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [isConfirmDeleting, setIsConfirmDeleting] = useState(false)
  const [rotateTarget, setRotateTarget] = useState<TunnelConfig | null>(null)
  const [rotateValue, setRotateValue] = useState('')
  const [rotateError, setRotateError] = useState<string | null>(null)
  const [isConfirmRotating, setIsConfirmRotating] = useState(false)

  const handleAddTunnel = () => {
    setEditingTunnel(undefined)
    setFormOpen(true)
  }

  const handleEditTunnel = (tunnel: TunnelConfig) => {
    setEditingTunnel(tunnel)
    setFormOpen(true)
  }

  const handleStart = async (tunnel: TunnelConfig) => {
    await startTunnel(tunnel.uuid)
  }

  const handleStop = async (tunnel: TunnelConfig) => {
    await stopTunnel(tunnel.uuid)
  }

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return
    setIsConfirmDeleting(true)
    setDeleteError(null)
    try {
      await deleteTunnel(deleteTarget.uuid)
      setDeleteTarget(null)
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Failed to delete')
    } finally {
      setIsConfirmDeleting(false)
    }
  }

  const handleRotateOpen = (tunnel: TunnelConfig) => {
    setRotateTarget(tunnel)
    setRotateValue('')
    setRotateError(null)
  }

  const handleRotateConfirm = async () => {
    if (!rotateTarget || !rotateValue.trim()) return
    setIsConfirmRotating(true)
    setRotateError(null)
    try {
      await rotateCredentials({ uuid: rotateTarget.uuid, credentials: rotateValue.trim() })
      setRotateTarget(null)
    } catch (err) {
      setRotateError(err instanceof Error ? err.message : 'Failed to rotate credentials')
    } finally {
      setIsConfirmRotating(false)
    }
  }

  const columns: Column<TunnelConfig>[] = [
    {
      key: 'name',
      header: t('hecate.page.columns.name'),
      sortable: true,
      cell: tunnel => (
        <span className="font-medium text-content-primary">{tunnel.name}</span>
      ),
    },
    {
      key: 'provider',
      header: t('hecate.page.columns.provider'),
      sortable: true,
      cell: tunnel => (
        <Badge variant="outline" size="sm" className="capitalize">
          {tunnel.provider}
        </Badge>
      ),
    },
    {
      key: 'status',
      header: t('hecate.page.columns.status'),
      cell: tunnel => {
        const status = getStatus(tunnel.uuid)
        return status ? <TunnelStatusBadge state={status.state} showLabel /> : (
          <span className="text-content-muted text-sm">—</span>
        )
      },
    },
    {
      key: 'is_active',
      header: t('hecate.page.columns.active'),
      cell: tunnel => (
        <Badge variant={tunnel.is_active ? 'success' : 'default'} size="sm">
          {tunnel.is_active ? t('common.enabled') : t('common.disabled')}
        </Badge>
      ),
    },
    {
      key: 'created_at',
      header: t('hecate.page.columns.created'),
      sortable: true,
      cell: tunnel => (
        <span className="text-sm text-content-secondary">
          {new Date(tunnel.created_at).toLocaleDateString()}
        </span>
      ),
    },
    {
      key: 'actions',
      header: t('hecate.page.columns.actions'),
      cell: tunnel => {
        const status = getStatus(tunnel.uuid)
        const isConnected = status?.state === 'connected' || status?.state === 'connecting'
        return (
          <div className="flex items-center justify-end gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={e => { e.stopPropagation(); void handleStart(tunnel) }}
              disabled={isConnected || isStarting}
              title={t('hecate.page.start')}
              aria-label={`${t('hecate.page.start')} ${tunnel.name}`}
            >
              <Play className="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={e => { e.stopPropagation(); void handleStop(tunnel) }}
              disabled={!isConnected || isStopping}
              title={t('hecate.page.stop')}
              aria-label={`${t('hecate.page.stop')} ${tunnel.name}`}
            >
              <Square className="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={e => { e.stopPropagation(); setLogsTunnel(tunnel) }}
              title={t('hecate.page.viewLogs')}
              aria-label={`${t('hecate.page.viewLogs')} ${tunnel.name}`}
            >
              <ScrollText className="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={e => { e.stopPropagation(); handleEditTunnel(tunnel) }}
              title={t('common.edit')}
              aria-label={`${t('common.edit')} ${tunnel.name}`}
            >
              <Pencil className="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={e => { e.stopPropagation(); handleRotateOpen(tunnel) }}
              disabled={isRotating}
              title={t('hecate.page.rotateCredentials')}
              aria-label={`${t('hecate.page.rotateCredentials')} ${tunnel.name}`}
            >
              <RotateCcw className="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={e => { e.stopPropagation(); setDeleteTarget(tunnel) }}
              disabled={isDeleting}
              title={t('hecate.page.deleteProvider')}
              aria-label={`${t('hecate.page.deleteProvider')} ${tunnel.name}`}
            >
              <Trash2 className="h-4 w-4 text-error" />
            </Button>
          </div>
        )
      },
    },
  ]

  const headerActions = (
    <Button onClick={handleAddTunnel}>
      <Plus className="w-4 h-4 mr-2" />
      {t('hecate.page.addProvider')}
    </Button>
  )

  return (
    <PageShell
      title={t('hecate.tunnels.title')}
      description={t('hecate.tunnels.description')}
      actions={headerActions}
    >
      {error && (
        <Alert variant="error" title={t('common.error')}>
          {error}
        </Alert>
      )}

      {loadingTunnels ? (
        <SkeletonTable rows={3} columns={6} />
      ) : tunnels.length === 0 ? (
        <EmptyState
          icon={<KeyRound className="h-12 w-12" />}
          title={t('hecate.page.emptyState.title')}
          description={t('hecate.page.emptyState.description')}
          action={{ label: t('hecate.page.addProvider'), onClick: handleAddTunnel }}
        />
      ) : (
        <DataTable
          data={tunnels}
          columns={columns}
          rowKey={tunnel => tunnel.uuid}
        />
      )}

      <HecateTunnelForm
        tunnel={editingTunnel}
        open={formOpen}
        onClose={() => {
          setFormOpen(false)
          setEditingTunnel(undefined)
        }}
      />

      {logsTunnel && (
        <TunnelLogViewer
          serverName={logsTunnel.name}
          serverUUID={logsTunnel.uuid}
          open
          onClose={() => setLogsTunnel(null)}
        />
      )}

      <Dialog open={deleteTarget !== null} onOpenChange={() => setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('hecate.page.confirmDeleteTitle')}</DialogTitle>
          </DialogHeader>
          <p className="text-content-secondary py-4">
            {t('hecate.page.confirmDeleteDescription', { name: deleteTarget?.name })}
          </p>
          {deleteError && (
            <p role="alert" className="text-sm text-error">
              {deleteError}
            </p>
          )}
          <DialogFooter>
            <Button
              variant="secondary"
              onClick={() => setDeleteTarget(null)}
              disabled={isConfirmDeleting}
            >
              {t('common.cancel')}
            </Button>
            <Button
              variant="danger"
              onClick={handleDeleteConfirm}
              disabled={isConfirmDeleting}
            >
              {isConfirmDeleting ? t('common.loading') : t('hecate.page.confirmDeleteButton')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={rotateTarget !== null} onOpenChange={() => setRotateTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('hecate.page.rotateTitle')}</DialogTitle>
          </DialogHeader>
          <div className="py-4 space-y-4">
            <p className="text-content-secondary text-sm">
              {t('hecate.page.rotateDescription', { name: rotateTarget?.name })}
            </p>
            <div>
              <Label htmlFor="rotate-creds">
                {t('hecate.page.rotateCredentials')}
                <span aria-hidden="true"> *</span>
              </Label>
              <textarea
                id="rotate-creds"
                required
                aria-required
                value={rotateValue}
                onChange={e => setRotateValue(e.target.value)}
                rows={4}
                className="w-full mt-1 bg-surface-subtle border border-border rounded-lg px-4 py-2 font-mono text-sm text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            {rotateError && (
              <p role="alert" className="text-sm text-error">
                {rotateError}
              </p>
            )}
          </div>
          <DialogFooter>
            <Button
              variant="secondary"
              onClick={() => setRotateTarget(null)}
              disabled={isConfirmRotating}
            >
              {t('common.cancel')}
            </Button>
            <Button
              onClick={handleRotateConfirm}
              disabled={isConfirmRotating || !rotateValue.trim()}
            >
              {isConfirmRotating ? t('common.loading') : t('hecate.page.rotateButton')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  )
}
