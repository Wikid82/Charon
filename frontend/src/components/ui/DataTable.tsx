import { ChevronUp, ChevronDown, ChevronsUpDown } from 'lucide-react'
import * as React from 'react'

import { Checkbox } from './Checkbox'
import { cn } from '../../utils/cn'

export interface Column<T> {
  key: string
  header: string
  cell: (row: T) => React.ReactNode
  sortable?: boolean
  width?: string
}

export interface DataTableProps<T> extends Omit<React.HTMLAttributes<HTMLDivElement>, 'children'> {
  data: T[]
  columns: Column<T>[]
  rowKey: (row: T) => string
  selectable?: boolean
  selectedKeys?: Set<string>
  onSelectionChange?: (keys: Set<string>) => void
  onRowClick?: (row: T) => void
  emptyState?: React.ReactNode
  isLoading?: boolean
  stickyHeader?: boolean
}

/**
 * DataTable - Reusable data table component
 *
 * Features:
 * - Generic type <T> for row data
 * - Sortable columns with chevron icons
 * - Row selection with Checkbox component
 * - Sticky header support
 * - Row hover states
 * - Selected row highlighting
 * - Empty state slot
 * - Responsive horizontal scroll
 */
export function DataTable<T>({
  data,
  columns,
  rowKey,
  selectable = false,
  selectedKeys = new Set(),
  onSelectionChange,
  onRowClick,
  emptyState,
  isLoading = false,
  stickyHeader = false,
  className,
  ...props
}: DataTableProps<T>) {
  const [sortConfig, setSortConfig] = React.useState<{
    key: string
    direction: 'asc' | 'desc'
  } | null>(null)

  const handleSort = (key: string) => {
    setSortConfig((prev) => {
      if (prev?.key === key) {
        if (prev.direction === 'asc') {
          return { key, direction: 'desc' }
        }
        // Reset sort if clicking third time
        return null
      }
      return { key, direction: 'asc' }
    })
  }

  const handleSelectAll = () => {
    if (!onSelectionChange) return

    if (selectedKeys.size === data.length) {
      // All selected, deselect all
      onSelectionChange(new Set())
    } else {
      // Select all
      onSelectionChange(new Set(data.map(rowKey)))
    }
  }

  const handleSelectRow = (key: string) => {
    if (!onSelectionChange) return

    const newKeys = new Set(selectedKeys)
    if (newKeys.has(key)) {
      newKeys.delete(key)
    } else {
      newKeys.add(key)
    }
    onSelectionChange(newKeys)
  }

  const allSelected = data.length > 0 && selectedKeys.size === data.length
  const someSelected = selectedKeys.size > 0 && selectedKeys.size < data.length

  const colSpan = columns.length + (selectable ? 1 : 0)

  return (
    <div
      className={cn(
        'rounded-xl border border-border overflow-hidden',
        className
      )}
      {...props}
    >
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead
            className={cn(
              'bg-surface-subtle border-b border-border',
              stickyHeader && 'sticky top-0 z-10'
            )}
          >
            <tr>
              {selectable && (
                <th className="w-12 px-4 py-3">
                  <Checkbox
                    checked={allSelected}
                    indeterminate={someSelected}
                    onCheckedChange={handleSelectAll}
                    aria-label="Select all rows"
                  />
                </th>
              )}
              {columns.map((col) => (
                <th
                  key={col.key}
                  className={cn(
                    'px-6 py-3 text-left text-xs font-semibold uppercase tracking-wider text-content-secondary',
                    col.sortable &&
                      'cursor-pointer select-none hover:text-content-primary transition-colors'
                  )}
                  style={{ width: col.width }}
                  onClick={() => col.sortable && handleSort(col.key)}
                  role={col.sortable ? 'button' : undefined}
                  tabIndex={col.sortable ? 0 : undefined}
                  onKeyDown={(e) => {
                    if (col.sortable && (e.key === 'Enter' || e.key === ' ')) {
                      e.preventDefault()
                      handleSort(col.key)
                    }
                  }}
                  aria-sort={
                    sortConfig?.key === col.key
                      ? sortConfig.direction === 'asc'
                        ? 'ascending'
                        : 'descending'
                      : undefined
                  }
                >
                  <div className="flex items-center gap-1">
                    <span>{col.header}</span>
                    {col.sortable && (
                      <span className="text-content-muted">
                        {sortConfig?.key === col.key ? (
                          sortConfig.direction === 'asc' ? (
                            <ChevronUp className="h-4 w-4" />
                          ) : (
                            <ChevronDown className="h-4 w-4" />
                          )
                        ) : (
                          <ChevronsUpDown className="h-4 w-4 opacity-50" />
                        )}
                      </span>
                    )}
                  </div>
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-border bg-surface-elevated">
            {isLoading ? (
              <tr>
                <td colSpan={colSpan} className="px-6 py-12">
                  <div className="flex justify-center">
                    <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
                  </div>
                </td>
              </tr>
            ) : data.length === 0 ? (
              <tr>
                <td colSpan={colSpan} className="px-6 py-12">
                  {emptyState || (
                    <div className="text-center text-content-muted">
                      No data available
                    </div>
                  )}
                </td>
              </tr>
            ) : (
              data.map((row) => {
                const key = rowKey(row)
                const isSelected = selectedKeys.has(key)

                return (
                  <tr
                    key={key}
                    className={cn(
                      'transition-colors',
                      isSelected && 'bg-brand-500/5',
                      onRowClick &&
                        'cursor-pointer hover:bg-surface-muted',
                      !onRowClick && 'hover:bg-surface-subtle'
                    )}
                    onClick={() => onRowClick?.(row)}
                    role={onRowClick ? 'button' : undefined}
                    tabIndex={onRowClick ? 0 : undefined}
                    onKeyDown={(e) => {
                      if (onRowClick && (e.key === 'Enter' || e.key === ' ')) {
                        e.preventDefault()
                        onRowClick(row)
                      }
                    }}
                  >
                    {selectable && (
                      <td
                        className="w-12 px-4 py-4"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <Checkbox
                          checked={isSelected}
                          onCheckedChange={() => handleSelectRow(key)}
                          aria-label={`Select row ${key}`}
                        />
                      </td>
                    )}
                    {columns.map((col) => (
                      <td
                        key={col.key}
                        className="px-6 py-4 text-sm text-content-primary"
                      >
                        {col.cell(row)}
                      </td>
                    ))}
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
