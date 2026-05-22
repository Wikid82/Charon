import { useDroppable } from '@dnd-kit/core'
import type React from 'react'

interface GroupDropZoneProps {
  /** Group UUID or the literal string 'ungrouped' */
  groupId: string
  /** Whether any drag is currently active (for ungrouped empty-state visibility) */
  isDragActive: boolean
  children: React.ReactNode
}

export function GroupDropZone({ groupId, isDragActive, children }: GroupDropZoneProps) {
  const { setNodeRef, isOver } = useDroppable({ id: groupId })

  return (
    <div
      ref={setNodeRef}
      data-drop-zone={groupId}
      className={[
        'rounded-xl transition-all duration-150',
        isOver
          ? 'ring-2 ring-brand-400 ring-offset-2 ring-offset-surface-base bg-brand-500/5'
          : '',
      ].join(' ')}
      // aria-dropeffect is deprecated in ARIA 1.1 but retained for backward compatibility with older assistive technologies
      aria-dropeffect={isDragActive ? 'move' : undefined}
    >
      {children}
    </div>
  )
}
