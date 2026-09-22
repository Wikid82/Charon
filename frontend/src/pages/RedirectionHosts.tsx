import { Globe } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'react-hot-toast'
import { useTranslation } from 'react-i18next'

import RedirectionHostForm from '../components/RedirectionHostForm'
import { PageShell } from '../components/layout/PageShell'
import {
  Badge,
  Button,
  DataTable,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  SkeletonTable,
  Switch,
  type Column,
} from '../components/ui'
import type { RedirectionHost } from '../api/redirectionHosts'
import {
  useCreateRedirectionHost,
  useDeleteRedirectionHost,
  useRedirectionHosts,
  useUpdateRedirectionHost,
} from '../hooks/useRedirectionHosts'

const STATUS_CODE_LABELS: Record<number, string> = {
  301: '301 Permanent',
  302: '302 Temporary',
  307: '307 Temporary',
  308: '308 Permanent',
}

export default function RedirectionHosts() {
  const { t } = useTranslation()
  const { data: hosts = [], isLoading } = useRedirectionHosts()
  const createHost = useCreateRedirectionHost()
  const updateHost = useUpdateRedirectionHost()
  const deleteHost = useDeleteRedirectionHost()

  const [showForm, setShowForm] = useState(false)
  const [editingHost, setEditingHost] = useState<RedirectionHost | undefined>()
  const [hostToDelete, setHostToDelete] = useState<RedirectionHost | null>(null)

  const handleAdd = () => {
    setEditingHost(undefined)
    setShowForm(true)
  }

  const handleEdit = (host: RedirectionHost) => {
    setEditingHost(host)
    setShowForm(true)
  }

  const handleSubmit = async (data: Partial<RedirectionHost>) => {
    if (editingHost) {
      await updateHost.mutateAsync({ uuid: editingHost.uuid, data })
    } else {
      await createHost.mutateAsync(data)
    }
    setShowForm(false)
    setEditingHost(undefined)
  }

  const handleDeleteConfirm = async () => {
    if (!hostToDelete) return
    try {
      await deleteHost.mutateAsync(hostToDelete.uuid)
      setHostToDelete(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete')
    }
  }

  const columns: Column<RedirectionHost>[] = [
    {
      key: 'name',
      header: t('redirectionHosts.columnName'),
      cell: (host) => (
        <div className="text-sm font-medium text-content-primary truncate">
          {host.name || <span className="text-content-muted italic">{t('redirectionHosts.unnamed')}</span>}
        </div>
      ),
    },
    {
      key: 'domain',
      header: t('redirectionHosts.columnDomain'),
      cell: (host) => (
        <div className="text-sm text-content-primary">
          {host.domain_names
            .split(',')
            .map((d) => d.trim())
            .filter(Boolean)
            .join(', ')}
        </div>
      ),
    },
    {
      key: 'target',
      header: t('redirectionHosts.columnTarget'),
      cell: (host) => (
        <div className="text-sm text-content-secondary truncate max-w-[28ch]" title={host.target_url}>
          {host.target_url}
        </div>
      ),
    },
    {
      key: 'statusCode',
      header: t('redirectionHosts.columnStatusCode'),
      cell: (host) => (
        <Badge variant="outline" size="sm">
          {STATUS_CODE_LABELS[host.status_code] ?? host.status_code}
        </Badge>
      ),
    },
    {
      key: 'status',
      header: t('redirectionHosts.columnStatus'),
      cell: (host) => (
        <Switch
          checked={host.enabled}
          aria-label={`${host.enabled ? 'Disable' : 'Enable'} redirection host ${host.name || host.domain_names}`}
          onCheckedChange={(checked) => updateHost.mutateAsync({ uuid: host.uuid, data: { enabled: checked } })}
        />
      ),
    },
    {
      key: 'actions',
      header: t('redirectionHosts.columnActions'),
      cell: (host) => (
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            aria-label={`Edit redirection host ${host.name || host.domain_names}`}
            onClick={() => handleEdit(host)}
          >
            {t('common.edit')}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="text-error hover:text-error hover:bg-error/10"
            aria-label={`Delete redirection host ${host.name || host.domain_names}`}
            onClick={() => setHostToDelete(host)}
          >
            {t('common.delete')}
          </Button>
        </div>
      ),
    },
  ]

  return (
    <PageShell
      title={t('redirectionHosts.title')}
      description={t('redirectionHosts.description')}
      actions={<Button onClick={handleAdd}>{t('redirectionHosts.addHost')}</Button>}
    >
      {isLoading ? (
        <SkeletonTable rows={5} columns={6} />
      ) : (
        <DataTable
          data={hosts}
          columns={columns}
          rowKey={(row) => row.uuid}
          emptyState={
            <EmptyState
              icon={<Globe className="h-12 w-12" />}
              title={t('redirectionHosts.noHosts')}
              description={t('redirectionHosts.noHostsDescription')}
              action={{ label: t('redirectionHosts.addHost'), onClick: handleAdd }}
            />
          }
        />
      )}

      {showForm && (
        <RedirectionHostForm
          host={editingHost}
          onSubmit={handleSubmit}
          onCancel={() => {
            setShowForm(false)
            setEditingHost(undefined)
          }}
        />
      )}

      <Dialog open={!!hostToDelete} onOpenChange={(open) => !open && setHostToDelete(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('redirectionHosts.deleteConfirmTitle')}</DialogTitle>
            <DialogDescription>
              {t('redirectionHosts.deleteConfirmMessage', {
                name: hostToDelete?.name || hostToDelete?.domain_names,
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setHostToDelete(null)}>
              {t('common.cancel')}
            </Button>
            <Button variant="danger" onClick={handleDeleteConfirm} isLoading={deleteHost.isPending}>
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageShell>
  )
}
