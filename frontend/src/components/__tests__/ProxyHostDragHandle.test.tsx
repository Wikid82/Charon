import { useDraggable } from '@dnd-kit/core'
import { render } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { ProxyHostDragHandle } from '../ProxyHostDragHandle'

type DraggableReturn = ReturnType<typeof useDraggable>

const makeDraggableReturn = (overrides: Partial<DraggableReturn> = {}): DraggableReturn =>
  ({
    attributes: {
      role: 'button' as const,
      tabIndex: 0,
      'aria-disabled': false,
      'aria-pressed': undefined,
      'aria-roledescription': 'draggable',
      'aria-describedby': 'dnd-description',
    },
    listeners: {},
    setNodeRef: vi.fn(),
    setActivatorNodeRef: vi.fn(),
    isDragging: false,
    active: null,
    activatorEvent: null,
    activeNodeRect: null,
    over: null,
    transform: null,
    node: { current: null },
    ...overrides,
  }) as DraggableReturn

vi.mock('@dnd-kit/core', () => ({
  useDraggable: vi.fn(() => makeDraggableReturn()),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

describe('ProxyHostDragHandle', () => {
  beforeEach(() => {
    vi.mocked(useDraggable).mockReturnValue(makeDraggableReturn())
  })

  it('uses single-host aria-label when dragCount is 1', () => {
    const { container } = render(
      <ProxyHostDragHandle hostUuid="h1" dragCount={1} />,
    )
    const span = container.querySelector('span')
    expect(span).toHaveAttribute('aria-label', 'proxyGroups.dnd.dragHandleSingle')
  })

  it('uses multi-host aria-label when dragCount is greater than 1', () => {
    const { container } = render(
      <ProxyHostDragHandle hostUuid="h1" dragCount={3} />,
    )
    const span = container.querySelector('span')
    expect(span).toHaveAttribute('aria-label', 'proxyGroups.dnd.dragHandleMultiple')
  })

  it('does not apply opacity-30 when isDragging is false', () => {
    const { container } = render(
      <ProxyHostDragHandle hostUuid="h1" dragCount={1} />,
    )
    const span = container.querySelector('span')
    expect(span?.className).not.toContain('opacity-30')
  })

  it('applies opacity-30 class when isDragging is true', () => {
    vi.mocked(useDraggable).mockReturnValue(makeDraggableReturn({ isDragging: true }))

    const { container } = render(
      <ProxyHostDragHandle hostUuid="h1" dragCount={1} />,
    )
    const span = container.querySelector('span')
    expect(span?.className).toContain('opacity-30')
  })

  it('passes hostUuid as id to useDraggable', () => {
    render(<ProxyHostDragHandle hostUuid="my-host-uuid" dragCount={1} />)
    expect(useDraggable).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'my-host-uuid' }),
    )
  })
})

