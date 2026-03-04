import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import {
  Skeleton,
  SkeletonCard,
  SkeletonTable,
  SkeletonList,
} from '../Skeleton'

describe('Skeleton', () => {
  it('renders with default variant', () => {
    render(<Skeleton data-testid="skeleton" />)

    const skeleton = screen.getByTestId('skeleton')
    expect(skeleton).toBeInTheDocument()
    expect(skeleton).toHaveClass('animate-pulse')
    expect(skeleton).toHaveClass('rounded-md')
  })

  it('renders with circular variant', () => {
    render(<Skeleton variant="circular" data-testid="skeleton" />)

    const skeleton = screen.getByTestId('skeleton')
    expect(skeleton).toHaveClass('rounded-full')
  })

  it('renders with text variant', () => {
    render(<Skeleton variant="text" data-testid="skeleton" />)

    const skeleton = screen.getByTestId('skeleton')
    expect(skeleton).toHaveClass('rounded')
    expect(skeleton).toHaveClass('h-4')
  })

  it('applies custom className', () => {
    render(<Skeleton className="custom-class" data-testid="skeleton" />)

    const skeleton = screen.getByTestId('skeleton')
    expect(skeleton).toHaveClass('custom-class')
  })

  it('passes through HTML attributes', () => {
    render(<Skeleton data-testid="skeleton" style={{ width: '100px' }} />)

    const skeleton = screen.getByTestId('skeleton')
    expect(skeleton).toHaveStyle({ width: '100px' })
  })
})

describe('SkeletonCard', () => {
  it('renders with default props (image and 3 lines)', () => {
    render(<SkeletonCard data-testid="skeleton-card" />)

    const card = screen.getByTestId('skeleton-card')
    expect(card).toBeInTheDocument()

    // Should have image skeleton (h-32)
    const skeletons = card.querySelectorAll('.animate-pulse')
    // 1 image + 1 title + 3 text lines = 5 total
    expect(skeletons.length).toBe(5)
  })

  it('renders without image when showImage is false', () => {
    render(<SkeletonCard showImage={false} data-testid="skeleton-card" />)

    const card = screen.getByTestId('skeleton-card')
    const skeletons = card.querySelectorAll('.animate-pulse')
    // 1 title + 3 text lines = 4 total (no image)
    expect(skeletons.length).toBe(4)
  })

  it('renders with custom number of lines', () => {
    render(<SkeletonCard lines={5} showImage={false} data-testid="skeleton-card" />)

    const card = screen.getByTestId('skeleton-card')
    const skeletons = card.querySelectorAll('.animate-pulse')
    // 1 title + 5 text lines = 6 total
    expect(skeletons.length).toBe(6)
  })

  it('applies custom className', () => {
    render(<SkeletonCard className="custom-class" data-testid="skeleton-card" />)

    const card = screen.getByTestId('skeleton-card')
    expect(card).toHaveClass('custom-class')
  })
})

describe('SkeletonTable', () => {
  it('renders with default rows and columns (5 rows, 4 columns)', () => {
    render(<SkeletonTable data-testid="skeleton-table" />)

    const table = screen.getByTestId('skeleton-table')
    expect(table).toBeInTheDocument()

    // Header row + 5 data rows
    const rows = table.querySelectorAll('.flex.gap-4')
    expect(rows.length).toBe(6) // 1 header + 5 rows
  })

  it('renders with custom rows', () => {
    render(<SkeletonTable rows={3} data-testid="skeleton-table" />)

    const table = screen.getByTestId('skeleton-table')
    // Header row + 3 data rows
    const rows = table.querySelectorAll('.flex.gap-4')
    expect(rows.length).toBe(4) // 1 header + 3 rows
  })

  it('renders with custom columns', () => {
    render(<SkeletonTable columns={6} rows={1} data-testid="skeleton-table" />)

    const table = screen.getByTestId('skeleton-table')
    // Check header has 6 skeletons
    const headerRow = table.querySelector('.bg-surface-subtle')
    const headerSkeletons = headerRow?.querySelectorAll('.animate-pulse')
    expect(headerSkeletons?.length).toBe(6)
  })

  it('applies custom className', () => {
    render(<SkeletonTable className="custom-class" data-testid="skeleton-table" />)

    const table = screen.getByTestId('skeleton-table')
    expect(table).toHaveClass('custom-class')
  })
})

describe('SkeletonList', () => {
  it('renders with default props (3 items with avatars)', () => {
    render(<SkeletonList data-testid="skeleton-list" />)

    const list = screen.getByTestId('skeleton-list')
    expect(list).toBeInTheDocument()

    // Each item has: 1 avatar (circular) + 2 text lines = 3 skeletons per item
    // 3 items * 3 = 9 total skeletons
    const items = list.querySelectorAll('.flex.items-center.gap-4')
    expect(items.length).toBe(3)
  })

  it('renders with custom number of items', () => {
    render(<SkeletonList items={5} data-testid="skeleton-list" />)

    const list = screen.getByTestId('skeleton-list')
    const items = list.querySelectorAll('.flex.items-center.gap-4')
    expect(items.length).toBe(5)
  })

  it('renders without avatars when showAvatar is false', () => {
    render(<SkeletonList showAvatar={false} items={2} data-testid="skeleton-list" />)

    const list = screen.getByTestId('skeleton-list')
    // No circular skeletons
    const circularSkeletons = list.querySelectorAll('.rounded-full')
    expect(circularSkeletons.length).toBe(0)
  })

  it('renders with avatars when showAvatar is true', () => {
    render(<SkeletonList showAvatar={true} items={2} data-testid="skeleton-list" />)

    const list = screen.getByTestId('skeleton-list')
    // Should have circular skeletons for avatars
    const circularSkeletons = list.querySelectorAll('.rounded-full')
    expect(circularSkeletons.length).toBe(2)
  })

  it('applies custom className', () => {
    render(<SkeletonList className="custom-class" data-testid="skeleton-list" />)

    const list = screen.getByTestId('skeleton-list')
    expect(list).toHaveClass('custom-class')
  })
})
