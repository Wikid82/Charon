import { useQuery } from '@tanstack/react-query'
import { Wifi } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { listNetBirdPeers, type NetBirdPeer } from '../../api/hecate'
import { cn } from '../../utils/cn'
import { Badge } from '../ui/Badge'
import { Button } from '../ui/Button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '../ui/Dialog'

interface NetBirdPeerPickerProps {
  open: boolean
  onClose: () => void
  onSelect: (peer: NetBirdPeer) => void
  selectedId?: string
}

export function NetBirdPeerPicker({ open, onClose, onSelect, selectedId }: NetBirdPeerPickerProps) {
  const { t } = useTranslation()

  const { data: peers = [], isLoading } = useQuery({
    queryKey: ['hecate', 'netbird', 'peers'],
    queryFn: listNetBirdPeers,
    enabled: open,
    staleTime: 60_000,
  })

  return (
    <Dialog open={open} onOpenChange={isOpen => !isOpen && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('hecate.form.mode.selectPeer')}</DialogTitle>
          <DialogDescription>
            {t('hecate.form.mode.agentDescription')}
          </DialogDescription>
        </DialogHeader>

        <div className="px-6 pb-6 space-y-2 max-h-96 overflow-y-auto">
          {isLoading ? (
            <p className="text-sm text-content-muted text-center py-8">{t('common.loading')}</p>
          ) : peers.length === 0 ? (
            <p className="text-sm text-content-muted text-center py-8">
              {t('hecate.tailscale.noDevices')}
            </p>
          ) : (
            <ul role="listbox" aria-label={t('hecate.form.mode.selectPeer')}>
              {peers.map(peer => (
                <li key={peer.id}>
                  <Button
                    variant="ghost"
                    role="option"
                    aria-selected={peer.id === selectedId}
                    className={cn(
                      'w-full justify-start gap-3 h-auto py-3 px-4 rounded-lg',
                      peer.id === selectedId && 'bg-brand-500/10 text-brand-500',
                    )}
                    onClick={() => {
                      onSelect(peer)
                      onClose()
                    }}
                  >
                    <div className="flex-1 text-left min-w-0">
                      <p className="font-medium text-sm truncate">{peer.name}</p>
                      <p className="text-xs text-content-muted truncate">{peer.ip}</p>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <Badge
                        variant={peer.online ? 'success' : 'secondary'}
                        size="sm"
                        className="gap-1"
                      >
                        <Wifi className="h-3 w-3" aria-hidden="true" />
                        {peer.online ? t('common.online') : t('common.offline')}
                      </Badge>
                      <span className="text-xs text-content-muted">{peer.os}</span>
                    </div>
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
