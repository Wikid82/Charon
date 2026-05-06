import { useQuery } from '@tanstack/react-query'
import { ChevronLeft, Wifi } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  listZeroTierMembers,
  listZeroTierNetworks,
  type ZeroTierMember,
  type ZeroTierNetwork,
} from '../../api/hecate'
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

interface ZeroTierMemberPickerProps {
  open: boolean
  onClose: () => void
  onSelect: (member: ZeroTierMember, network: ZeroTierNetwork) => void
  selectedId?: string
}

export function ZeroTierMemberPicker({
  open,
  onClose,
  onSelect,
  selectedId,
}: ZeroTierMemberPickerProps) {
  const { t } = useTranslation()
  const [selectedNetwork, setSelectedNetwork] = useState<ZeroTierNetwork | null>(null)

  const { data: networks = [], isLoading: networksLoading } = useQuery({
    queryKey: ['hecate', 'zerotier', 'networks'],
    queryFn: listZeroTierNetworks,
    enabled: open,
    staleTime: 60_000,
  })

  const { data: members = [], isLoading: membersLoading } = useQuery({
    queryKey: ['hecate', 'zerotier', 'members', selectedNetwork?.id],
    queryFn: () => listZeroTierMembers(selectedNetwork!.id),
    enabled: !!selectedNetwork,
    staleTime: 60_000,
  })

  const handleClose = () => {
    setSelectedNetwork(null)
    onClose()
  }

  const isLoading = networksLoading || (!!selectedNetwork && membersLoading)

  return (
    <Dialog open={open} onOpenChange={isOpen => !isOpen && handleClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {selectedNetwork
              ? t('hecate.form.mode.selectMember')
              : t('hecate.form.mode.selectDevice')}
          </DialogTitle>
          <DialogDescription>
            {selectedNetwork ? selectedNetwork.name : t('hecate.form.mode.agentDescription')}
          </DialogDescription>
        </DialogHeader>

        <div className="px-6 pb-6 max-h-96 overflow-y-auto">
          {selectedNetwork && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSelectedNetwork(null)}
              className="mb-3 gap-1"
            >
              <ChevronLeft className="h-4 w-4" aria-hidden="true" />
              {t('common.back')}
            </Button>
          )}

          {isLoading ? (
            <p className="text-sm text-content-muted text-center py-8">{t('common.loading')}</p>
          ) : !selectedNetwork ? (
            networks.length === 0 ? (
              <p className="text-sm text-content-muted text-center py-8">
                {t('hecate.tailscale.noDevices')}
              </p>
            ) : (
              <ul role="listbox" aria-label={t('hecate.form.mode.selectDevice')} className="space-y-1">
                {networks.map(network => (
                  <li key={network.id}>
                    <Button
                      variant="ghost"
                      role="option"
                      aria-selected={false}
                      className="w-full justify-start gap-3 h-auto py-3 px-4 rounded-lg"
                      onClick={() => setSelectedNetwork(network)}
                    >
                      <div className="flex-1 text-left min-w-0">
                        <p className="font-medium text-sm truncate">{network.name}</p>
                        <p className="text-xs text-content-muted truncate">{network.id}</p>
                      </div>
                      <Badge variant="outline" size="sm">
                        {network.total_member_count} members
                      </Badge>
                    </Button>
                  </li>
                ))}
              </ul>
            )
          ) : members.length === 0 ? (
            <p className="text-sm text-content-muted text-center py-8">
              {t('hecate.tailscale.noDevices')}
            </p>
          ) : (
            <ul role="listbox" aria-label={t('hecate.form.mode.selectMember')} className="space-y-1">
              {members.map(member => (
                <li key={member.node_id}>
                  <Button
                    variant="ghost"
                    role="option"
                    aria-selected={member.node_id === selectedId}
                    className={cn(
                      'w-full justify-start gap-3 h-auto py-3 px-4 rounded-lg',
                      member.node_id === selectedId && 'bg-brand-500/10 text-brand-500',
                    )}
                    onClick={() => {
                      onSelect(member, selectedNetwork)
                      handleClose()
                    }}
                  >
                    <div className="flex-1 text-left min-w-0">
                      <p className="font-medium text-sm truncate">{member.name}</p>
                      <p className="text-xs text-content-muted truncate">
                        {member.ip_assignments[0] ?? member.node_id}
                      </p>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <Badge
                        variant={member.online ? 'success' : 'secondary'}
                        size="sm"
                        className="gap-1"
                      >
                        <Wifi className="h-3 w-3" aria-hidden="true" />
                        {member.online ? t('common.online') : t('common.offline')}
                      </Badge>
                      {!member.authorized && (
                        <Badge variant="warning" size="sm">
                          {t('common.unauthorized')}
                        </Badge>
                      )}
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
