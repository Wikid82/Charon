import { useDraggable } from '@dnd-kit/core'
import { GripVertical } from 'lucide-react'
import { useTranslation } from 'react-i18next'

interface ProxyHostDragHandleProps {
  hostUuid: string
  /** Number of hosts that will move (≥1 when host is part of selection) */
  dragCount: number
}

export function ProxyHostDragHandle({ hostUuid, dragCount }: ProxyHostDragHandleProps) {
  const { t } = useTranslation()
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: hostUuid,
    data: { type: 'proxy-host', hostUuid },
  })

  return (
    <span
      ref={setNodeRef}
      {...attributes}
      {...listeners}
      className={[
        'inline-flex items-center justify-center w-6 h-6 rounded',
        'cursor-grab active:cursor-grabbing',
        'text-content-muted hover:text-content-secondary',
        'focus-visible:outline-none focus-visible:ring-2',
        'focus-visible:ring-brand-500 focus-visible:ring-offset-1',
        isDragging ? 'opacity-30' : '',
      ].join(' ')}
      aria-label={
        dragCount > 1
          ? t('proxyGroups.dnd.dragHandleMultiple', { count: dragCount })
          : t('proxyGroups.dnd.dragHandleSingle')
      }
      aria-roledescription={t('proxyGroups.dnd.roleDescription')}
    >
      <GripVertical size={16} aria-hidden="true" />
    </span>
  )
}
