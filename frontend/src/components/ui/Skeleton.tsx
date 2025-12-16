import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../../utils/cn'

const skeletonVariants = cva(
  'animate-pulse bg-surface-muted',
  {
    variants: {
      variant: {
        default: 'rounded-md',
        circular: 'rounded-full',
        text: 'rounded h-4',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)

export interface SkeletonProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof skeletonVariants> {}

export function Skeleton({ className, variant, ...props }: SkeletonProps) {
  return (
    <div
      className={cn(skeletonVariants({ variant }), className)}
      {...props}
    />
  )
}

// Pre-built patterns

export interface SkeletonCardProps extends React.HTMLAttributes<HTMLDivElement> {
  showImage?: boolean
  lines?: number
}

export function SkeletonCard({
  className,
  showImage = true,
  lines = 3,
  ...props
}: SkeletonCardProps) {
  return (
    <div
      className={cn(
        'rounded-lg border border-border bg-surface-elevated p-4 space-y-4',
        className
      )}
      {...props}
    >
      {showImage && (
        <Skeleton className="h-32 w-full rounded-md" />
      )}
      <div className="space-y-2">
        <Skeleton className="h-5 w-3/4" />
        {Array.from({ length: lines }).map((_, i) => (
          <Skeleton
            key={i}
            variant="text"
            className={cn(
              'h-4',
              i === lines - 1 ? 'w-1/2' : 'w-full'
            )}
          />
        ))}
      </div>
    </div>
  )
}

export interface SkeletonTableProps extends React.HTMLAttributes<HTMLDivElement> {
  rows?: number
  columns?: number
}

export function SkeletonTable({
  className,
  rows = 5,
  columns = 4,
  ...props
}: SkeletonTableProps) {
  return (
    <div
      className={cn('rounded-lg border border-border overflow-hidden', className)}
      {...props}
    >
      {/* Header */}
      <div className="flex gap-4 p-4 bg-surface-subtle border-b border-border">
        {Array.from({ length: columns }).map((_, i) => (
          <Skeleton key={i} className="h-4 flex-1" />
        ))}
      </div>
      {/* Rows */}
      <div className="divide-y divide-border">
        {Array.from({ length: rows }).map((_, rowIndex) => (
          <div key={rowIndex} className="flex gap-4 p-4">
            {Array.from({ length: columns }).map((_, colIndex) => (
              <Skeleton
                key={colIndex}
                className={cn(
                  'h-4 flex-1',
                  colIndex === 0 && 'w-1/4 flex-none'
                )}
              />
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}

export interface SkeletonListProps extends React.HTMLAttributes<HTMLDivElement> {
  items?: number
  showAvatar?: boolean
}

export function SkeletonList({
  className,
  items = 3,
  showAvatar = true,
  ...props
}: SkeletonListProps) {
  return (
    <div className={cn('space-y-4', className)} {...props}>
      {Array.from({ length: items }).map((_, i) => (
        <div key={i} className="flex items-center gap-4">
          {showAvatar && (
            <Skeleton variant="circular" className="h-10 w-10 flex-shrink-0" />
          )}
          <div className="flex-1 space-y-2">
            <Skeleton className="h-4 w-1/3" />
            <Skeleton className="h-3 w-2/3" />
          </div>
        </div>
      ))}
    </div>
  )
}
