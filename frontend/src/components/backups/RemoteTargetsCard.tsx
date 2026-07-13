import { useQueryClient } from '@tanstack/react-query'
import { CloudOff, Pencil, Plug, Plus, Trash2, Zap } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { RemoteTargetFormDialog } from './RemoteTargetFormDialog'
import {
  useRemoteTargets,
  useDeleteRemoteTarget,
  useTestRemoteTarget,
  useStartRemoteTargetOAuth,
  useDisconnectRemoteTargetOAuth,
  REMOTE_TARGETS_QUERY_KEY,
  type RemoteTarget,
} from '../../hooks/useRemoteTargets'
import { toast } from '../../utils/toast'
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

import type { RemoteTargetOAuthStatus } from '../../api/backups'


const STATUS_VARIANT: Record<RemoteTarget['last_test_status'], 'success' | 'destructive' | 'outline'> = {
  ok: 'success',
  failed: 'destructive',
  never: 'outline',
}

/** Provider types that carry an OAuth connection lifecycle (spec §3.3/§3.4). */
const OAUTH_TYPES = new Set<RemoteTarget['type']>(['dropbox', 'google_drive'])

const OAUTH_STATUS_VARIANT: Record<Exclude<RemoteTargetOAuthStatus, ''>, 'success' | 'destructive' | 'outline'> = {
  connected: 'success',
  revoked: 'destructive',
  not_connected: 'outline',
}

const OAUTH_STATUS_LABEL_KEY: Record<Exclude<RemoteTargetOAuthStatus, ''>, string> = {
  connected: 'connected',
  revoked: 'revoked',
  not_connected: 'notConnected',
}

/** Maps the `provider` query-param value (spec §3.3 OAuth callback redirect) to its type label i18n key. */
const OAUTH_PROVIDER_TYPE_LABEL_KEY: Record<string, string> = {
  dropbox: 'typeDropbox',
  google_drive: 'typeGoogleDrive',
}

/**
 * Remote storage target list with status badges + test-connection button
 * (spec §3.3.2, §3.7, §3.8). Composes `RemoteTargetFormDialog` for
 * create/edit.
 */
export function RemoteTargetsCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { data: targets, isLoading } = useRemoteTargets()
  const deleteMutation = useDeleteRemoteTarget()
  const testMutation = useTestRemoteTarget()
  const startOAuthMutation = useStartRemoteTargetOAuth()
  const disconnectOAuthMutation = useDisconnectRemoteTargetOAuth()

  const [editingTarget, setEditingTarget] = useState<RemoteTarget | null | undefined>(undefined)
  const [deletingTarget, setDeletingTarget] = useState<RemoteTarget | null>(null)

  // Picks up the redirect back from the OAuth callback route (spec §3.3/§3.6):
  // `oauth_result=success&provider=...&target=...` or
  // `oauth_result=error&provider=...&message=...`. Runs once on mount, strips
  // the query string so a page refresh doesn't re-show the toast, and
  // invalidates the target list so the just-connected row's oauth_status
  // badge reflects the new state without a manual refresh.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const oauthResult = params.get('oauth_result')
    if (!oauthResult) return

    const provider = params.get('provider') ?? ''
    if (oauthResult === 'success') {
      const providerLabelKey = OAUTH_PROVIDER_TYPE_LABEL_KEY[provider]
      const providerLabel = providerLabelKey ? t(`backups.remoteTargets.${providerLabelKey}`) : provider
      toast.success(t('backups.remoteTargets.oauthConnectSuccess', { provider: providerLabel }))
    } else {
      const message = params.get('message')
      toast.error(
        message === 'authorization_denied'
          ? t('backups.remoteTargets.oauthConnectDenied')
          : t('backups.remoteTargets.oauthConnectError')
      )
    }

    window.history.replaceState(null, '', window.location.pathname)
    queryClient.invalidateQueries({ queryKey: REMOTE_TARGETS_QUERY_KEY })
    // Runs once on mount only — the query string is a one-time signal from
    // the OAuth redirect, not reactive state to re-derive from.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

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

  const handleConnect = (target: RemoteTarget) => {
    startOAuthMutation.mutate(target.uuid, {
      onSuccess: (result) => {
        // Full-page redirect (spec §3.6) — no popup/postMessage.
        window.location.href = result.authorize_url
      },
      onError: (error: Error) => toast.error(error.message),
    })
  }

  const handleDisconnect = (target: RemoteTarget) => {
    disconnectOAuthMutation.mutate(target.uuid, {
      onSuccess: () => toast.success(t('backups.remoteTargets.disconnectSuccess')),
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
                  {OAUTH_TYPES.has(target.type) && target.oauth_status && (
                    <Badge
                      data-testid="backup-remote-target-oauth-status-badge"
                      variant={OAUTH_STATUS_VARIANT[target.oauth_status]}
                    >
                      {t(`backups.remoteTargets.oauthStatus.${OAUTH_STATUS_LABEL_KEY[target.oauth_status]}`)}
                    </Badge>
                  )}
                  {OAUTH_TYPES.has(target.type) && target.oauth_status !== 'connected' && (
                    <Button
                      variant="secondary"
                      size="sm"
                      data-testid="backup-remote-target-connect-btn"
                      onClick={() => handleConnect(target)}
                      isLoading={startOAuthMutation.isPending && startOAuthMutation.variables === target.uuid}
                    >
                      <Plug className="w-4 h-4 mr-1" />
                      {target.oauth_status === 'revoked'
                        ? t('backups.remoteTargets.reconnect')
                        : t('backups.remoteTargets.connect')}
                    </Button>
                  )}
                  {OAUTH_TYPES.has(target.type) && target.oauth_status === 'connected' && (
                    <Button
                      variant="ghost"
                      size="sm"
                      data-testid="backup-remote-target-disconnect-btn"
                      onClick={() => handleDisconnect(target)}
                      isLoading={disconnectOAuthMutation.isPending && disconnectOAuthMutation.variables === target.uuid}
                      title={t('backups.remoteTargets.disconnect')}
                    >
                      {t('backups.remoteTargets.disconnect')}
                    </Button>
                  )}
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
