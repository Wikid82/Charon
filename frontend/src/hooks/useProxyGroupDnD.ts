import { useState, useCallback } from 'react'
import type { DragStartEvent, DragEndEvent, DragOverEvent } from '@dnd-kit/core'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'react-hot-toast'
import { useTranslation } from 'react-i18next'

import type { ProxyHost } from '../api/proxyHosts'
import type { ProxyGroup } from '../api/proxyGroups'
import { QUERY_KEY } from './useProxyHosts'

interface UseProxyGroupDnDOptions {
  hosts: ProxyHost[]
  groups: ProxyGroup[]
  selectedHosts: Set<string>
  setSelectedHosts: (keys: Set<string>) => void
  bulkUpdateGroup: (hostUUIDs: string[], proxyGroupId: string | null) => Promise<{ updated: number; errors: { uuid: string; error: string }[] }>
}

export function useProxyGroupDnD({
  hosts,
  groups,
  selectedHosts,
  setSelectedHosts,
  bulkUpdateGroup,
}: UseProxyGroupDnDOptions) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [activeDragId, setActiveDragId] = useState<string | null>(null)
  const [overGroupId, setOverGroupId] = useState<string | null>(null)
  const [hostsBeingDragged, setHostsBeingDragged] = useState<string[]>([])

  const handleDragStart = useCallback(
    (event: DragStartEvent) => {
      const uuid = event.active.id as string
      setActiveDragId(uuid)
      if (selectedHosts.has(uuid)) {
        setHostsBeingDragged([...selectedHosts])
      } else {
        setHostsBeingDragged([uuid])
      }
    },
    [selectedHosts],
  )

  const handleDragOver = useCallback((event: DragOverEvent) => {
    setOverGroupId(event.over ? (event.over.id as string) : null)
  }, [])

  const handleDragCancel = useCallback(() => {
    setActiveDragId(null)
    setOverGroupId(null)
    setHostsBeingDragged([])
  }, [])

  const handleDragEnd = useCallback(
    async (event: DragEndEvent) => {
      if (!event.over) {
        handleDragCancel()
        return
      }

      const targetId = event.over.id as string
      const targetGroup = targetId === 'ungrouped' ? null : groups.find((g) => g.uuid === targetId)

      const dragged = hostsBeingDragged
      const targetGroupUuid = targetGroup?.uuid ?? null

      const allAlreadyInTarget = dragged.every((uuid) => {
        const host = hosts.find((h) => h.uuid === uuid)
        if (!host) return false
        const currentGroupUuid = host.proxy_group?.uuid ?? null
        return currentGroupUuid === targetGroupUuid
      })

      if (allAlreadyInTarget) {
        handleDragCancel()
        return
      }

      const snapshot = queryClient.getQueryData<ProxyHost[]>(QUERY_KEY)

      queryClient.setQueryData<ProxyHost[]>(QUERY_KEY, (old) => {
        if (!old) return old
        return old.map((host) => {
          if (!dragged.includes(host.uuid)) return host
          return {
            ...host,
            proxy_group_id: targetGroupUuid,
            proxy_group: targetGroup
              ? { uuid: targetGroup.uuid, name: targetGroup.name, color: targetGroup.color ?? '' }
              : null,
          }
        })
      })

      const draggedSnapshot = [...dragged]
      setActiveDragId(null)
      setOverGroupId(null)
      setHostsBeingDragged([])

      try {
        const result = await bulkUpdateGroup(draggedSnapshot, targetGroupUuid)

        if (result.errors.length > 0) {
          toast.error(t('proxyGroups.dnd.partialError', { count: result.errors.length }))
        } else {
          toast.success(t('proxyGroups.dnd.moveSuccess', { count: draggedSnapshot.length }))
        }

        queryClient.invalidateQueries({ queryKey: QUERY_KEY })
        setSelectedHosts(new Set())
      } catch {
        if (snapshot) {
          queryClient.setQueryData(QUERY_KEY, snapshot)
        }
        toast.error(t('proxyGroups.dnd.moveFailed'))
      }
    },
    [hostsBeingDragged, hosts, groups, queryClient, bulkUpdateGroup, handleDragCancel, t, setSelectedHosts],
  )

  return {
    activeDragId,
    overGroupId,
    hostsBeingDragged,
    handleDragStart,
    handleDragOver,
    handleDragEnd,
    handleDragCancel,
  }
}
