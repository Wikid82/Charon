import { useDroppable } from '@dnd-kit/core'
import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { GroupDropZone } from '../GroupDropZone'

vi.mock('@dnd-kit/core', () => ({
  useDroppable: vi.fn(() => ({ setNodeRef: vi.fn(), isOver: false })),
}))

describe('GroupDropZone', () => {
  beforeEach(() => {
    vi.mocked(useDroppable).mockReturnValue({
      setNodeRef: vi.fn(),
      isOver: false,
      active: null,
      rect: { current: null },
      node: { current: null },
      over: null,
    })
  })

  it('renders children', () => {
    render(
      <GroupDropZone groupId="group-1" isDragActive={false}>
        <span>child content</span>
      </GroupDropZone>,
    )
    expect(screen.getByText('child content')).toBeInTheDocument()
  })

  it('does not apply ring-2 class when isOver is false', () => {
    const { container } = render(
      <GroupDropZone groupId="group-1" isDragActive={false}>
        <span>content</span>
      </GroupDropZone>,
    )
    const div = container.firstChild as HTMLElement
    expect(div.className).not.toContain('ring-2')
  })

  it('applies ring-2 class when isOver is true', () => {
    vi.mocked(useDroppable).mockReturnValue({
      setNodeRef: vi.fn(),
      isOver: true,
      active: null,
      rect: { current: null },
      node: { current: null },
      over: null,
    })

    const { container } = render(
      <GroupDropZone groupId="group-1" isDragActive={false}>
        <span>content</span>
      </GroupDropZone>,
    )
    const div = container.firstChild as HTMLElement
    expect(div.className).toContain('ring-2')
  })

  it('sets aria-dropeffect="move" when isDragActive is true', () => {
    const { container } = render(
      <GroupDropZone groupId="group-1" isDragActive={true}>
        <span>content</span>
      </GroupDropZone>,
    )
    const div = container.firstChild as HTMLElement
    expect(div).toHaveAttribute('aria-dropeffect', 'move')
  })

  it('omits aria-dropeffect when isDragActive is false', () => {
    const { container } = render(
      <GroupDropZone groupId="group-1" isDragActive={false}>
        <span>content</span>
      </GroupDropZone>,
    )
    const div = container.firstChild as HTMLElement
    expect(div).not.toHaveAttribute('aria-dropeffect')
  })

  it('passes groupId to useDroppable', () => {
    render(
      <GroupDropZone groupId="my-group" isDragActive={false}>
        <span>content</span>
      </GroupDropZone>,
    )
    expect(useDroppable).toHaveBeenCalledWith({ id: 'my-group' })
  })
})
