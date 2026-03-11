import { formatDistanceToNow } from 'date-fns'
import {
  Edit,
  Trash2,
  TestTube,
  Star,
  CheckCircle,
  XCircle,
  AlertTriangle,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  Button,
  Badge,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from './ui'

import type { DNSProvider } from '../api/dnsProviders'

interface DNSProviderCardProps {
  provider: DNSProvider
  onEdit: (provider: DNSProvider) => void
  onDelete: (id: number) => void
  onTest: (id: number) => void
  isTesting?: boolean
}

export default function DNSProviderCard({
  provider,
  onEdit,
  onDelete,
  onTest,
  isTesting = false,
}: DNSProviderCardProps) {
  const { t } = useTranslation()
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)

  const getStatusBadge = () => {
    if (!provider.has_credentials) {
      return (
        <Badge variant="warning">
          <AlertTriangle className="w-3 h-3 mr-1" />
          {t('dnsProviders.unconfigured')}
        </Badge>
      )
    }
    if (provider.last_error) {
      return (
        <Badge variant="destructive">
          <XCircle className="w-3 h-3 mr-1" />
          {t('dnsProviders.error')}
        </Badge>
      )
    }
    if (provider.enabled) {
      return (
        <Badge variant="success">
          <CheckCircle className="w-3 h-3 mr-1" />
          {t('dnsProviders.active')}
        </Badge>
      )
    }
    return (
      <Badge variant="secondary">
        {t('common.disabled')}
      </Badge>
    )
  }

  const getProviderIcon = (type: string) => {
    const iconMap: Record<string, string> = {
      cloudflare: '☁️',
      route53: '🔶',
      digitalocean: '🐙',
      googleclouddns: '🔵',
      namecheap: '🏢',
      godaddy: '🟢',
      azure: '⚡',
      hetzner: '🟠',
      vultr: '🔷',
      dnsimple: '💎',
    }
    return iconMap[type] || '🌐'
  }

  const handleDeleteConfirm = () => {
    onDelete(provider.id)
    setShowDeleteDialog(false)
  }

  return (
    <>
      <Card className="hover:shadow-lg transition-shadow">
        <CardHeader className="pb-3">
          <div className="flex items-start justify-between">
            <div className="flex items-center gap-3">
              <div className="text-3xl">{getProviderIcon(provider.provider_type)}</div>
              <div>
                <CardTitle className="flex items-center gap-2">
                  {provider.name}
                  {provider.is_default && (
                    <Star className="w-4 h-4 text-yellow-500 fill-yellow-500" aria-label={t('dnsProviders.default')} />
                  )}
                </CardTitle>
                <p className="text-sm text-content-secondary mt-1">
                  {t(`dnsProviders.types.${provider.provider_type}`, provider.provider_type)}
                </p>
              </div>
            </div>
            {getStatusBadge()}
          </div>
        </CardHeader>

        <CardContent className="space-y-4">
          {/* Usage Stats */}
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p className="text-content-muted">{t('dnsProviders.lastUsed')}</p>
              <p className="font-medium text-content-primary">
                {provider.last_used_at
                  ? formatDistanceToNow(new Date(provider.last_used_at), { addSuffix: true })
                  : t('dnsProviders.neverUsed')}
              </p>
            </div>
            <div>
              <p className="text-content-muted">{t('dnsProviders.successRate')}</p>
              <p className="font-medium text-content-primary">
                {provider.success_count} / {provider.failure_count}
              </p>
            </div>
          </div>

          {/* Settings */}
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p className="text-content-muted">{t('dnsProviders.propagationTimeout')}</p>
              <p className="font-medium text-content-primary">{provider.propagation_timeout}s</p>
            </div>
            <div>
              <p className="text-content-muted">{t('dnsProviders.pollingInterval')}</p>
              <p className="font-medium text-content-primary">{provider.polling_interval}s</p>
            </div>
          </div>

          {/* Last Error */}
          {provider.last_error && (
            <div className="bg-error/10 border border-error/20 rounded-lg p-3">
              <p className="text-xs font-medium text-error mb-1">{t('dnsProviders.lastError')}</p>
              <p className="text-xs text-content-secondary">{provider.last_error}</p>
            </div>
          )}

          {/* Action Buttons */}
          <div className="flex gap-2 pt-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => onEdit(provider)}
              className="flex-1"
            >
              <Edit className="w-4 h-4 mr-2" />
              {t('common.edit')}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => onTest(provider.id)}
              isLoading={isTesting}
              disabled={!provider.has_credentials}
              className="flex-1"
            >
              <TestTube className="w-4 h-4 mr-2" />
              {t('common.test')}
            </Button>
            <Button
              variant="danger"
              size="sm"
              onClick={() => setShowDeleteDialog(true)}
            >
              <Trash2 className="w-4 h-4" />
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Delete Confirmation Dialog */}
      <Dialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('dnsProviders.deleteProvider')}</DialogTitle>
            <DialogDescription>
              {t('dnsProviders.deleteConfirmation', { name: provider.name })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setShowDeleteDialog(false)}>
              {t('common.cancel')}
            </Button>
            <Button variant="danger" onClick={handleDeleteConfirm}>
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
