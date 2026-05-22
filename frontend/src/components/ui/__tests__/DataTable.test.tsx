import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { DataTable, type Column } from '../DataTable'

interface TestRow {
  id: string
  name: string
  status: string
}

const mockData: TestRow[] = [
  { id: '1', name: 'Item 1', status: 'Active' },
  { id: '2', name: 'Item 2', status: 'Inactive' },
  { id: '3', name: 'Item 3', status: 'Active' },
]

const mockColumns: Column<TestRow>[] = [
  { key: 'name', header: 'Name', cell: (row) => row.name },
  { key: 'status', header: 'Status', cell: (row) => row.status },
]

const sortableColumns: Column<TestRow>[] = [
  { key: 'name', header: 'Name', cell: (row) => row.name, sortable: true },
  { key: 'status', header: 'Status', cell: (row) => row.status, sortable: true },
]

describe('DataTable', () => {
  it('renders correctly with data', () => {
    render(
      <DataTable
        data={mockData}
        columns={mockColumns}
        rowKey={(row) => row.id}
      />
    )

    expect(screen.getByText('Name')).toBeInTheDocument()
    expect(screen.getByText('Status')).toBeInTheDocument()
    expect(screen.getByText('Item 1')).toBeInTheDocument()
    expect(screen.getByText('Item 2')).toBeInTheDocument()
    expect(screen.getByText('Item 3')).toBeInTheDocument()
  })

  it('renders empty state when no data', () => {
    render(
      <DataTable
        data={[]}
        columns={mockColumns}
        rowKey={(row) => row.id}
      />
    )

    expect(screen.getByText('No data available')).toBeInTheDocument()
  })

  it('renders custom empty state', () => {
    render(
      <DataTable
        data={[]}
        columns={mockColumns}
        rowKey={(row) => row.id}
        emptyState={<div>Custom empty message</div>}
      />
    )

    expect(screen.getByText('Custom empty message')).toBeInTheDocument()
  })

  it('renders loading state', () => {
    render(
      <DataTable
        data={[]}
        columns={mockColumns}
        rowKey={(row) => row.id}
        isLoading={true}
      />
    )

    // Loading spinner should be present (animated div)
    const spinnerContainer = document.querySelector('.animate-spin')
    expect(spinnerContainer).toBeInTheDocument()
  })

  it('handles sortable column click - ascending', () => {
    render(
      <DataTable
        data={mockData}
        columns={sortableColumns}
        rowKey={(row) => row.id}
      />
    )

    const nameHeader = screen.getByText('Name').closest('th')
    expect(nameHeader).toHaveAttribute('role', 'button')

    fireEvent.click(nameHeader!)
    expect(nameHeader).toHaveAttribute('aria-sort', 'ascending')
  })

  it('handles sortable column click - descending on second click', () => {
    render(
      <DataTable
        data={mockData}
        columns={sortableColumns}
        rowKey={(row) => row.id}
      />
    )

    const nameHeader = screen.getByText('Name').closest('th')

    // First click - ascending
    fireEvent.click(nameHeader!)
    expect(nameHeader).toHaveAttribute('aria-sort', 'ascending')

    // Second click - descending
    fireEvent.click(nameHeader!)
    expect(nameHeader).toHaveAttribute('aria-sort', 'descending')
  })

  it('handles sortable column click - resets on third click', () => {
    render(
      <DataTable
        data={mockData}
        columns={sortableColumns}
        rowKey={(row) => row.id}
      />
    )

    const nameHeader = screen.getByText('Name').closest('th')

    // First click - ascending
    fireEvent.click(nameHeader!)
    // Second click - descending
    fireEvent.click(nameHeader!)
    // Third click - reset
    fireEvent.click(nameHeader!)
    expect(nameHeader).not.toHaveAttribute('aria-sort')
  })

  it('handles sortable column keyboard navigation', () => {
    render(
      <DataTable
        data={mockData}
        columns={sortableColumns}
        rowKey={(row) => row.id}
      />
    )

    const nameHeader = screen.getByText('Name').closest('th')

    fireEvent.keyDown(nameHeader!, { key: 'Enter' })
    expect(nameHeader).toHaveAttribute('aria-sort', 'ascending')

    fireEvent.keyDown(nameHeader!, { key: ' ' })
    expect(nameHeader).toHaveAttribute('aria-sort', 'descending')
  })

  it('handles row selection - single row', () => {
    const onSelectionChange = vi.fn()
    render(
      <DataTable
        data={mockData}
        columns={mockColumns}
        rowKey={(row) => row.id}
        selectable={true}
        selectedKeys={new Set()}
        onSelectionChange={onSelectionChange}
      />
    )

    const checkboxes = screen.getAllByRole('checkbox')
    // First checkbox is "select all", row checkboxes start at index 1
    fireEvent.click(checkboxes[1])

    expect(onSelectionChange).toHaveBeenCalledWith(new Set(['1']))
  })

  it('handles row selection - deselect row', () => {
    const onSelectionChange = vi.fn()
    render(
      <DataTable
        data={mockData}
        columns={mockColumns}
        rowKey={(row) => row.id}
        selectable={true}
        selectedKeys={new Set(['1'])}
        onSelectionChange={onSelectionChange}
      />
    )

    const checkboxes = screen.getAllByRole('checkbox')
    fireEvent.click(checkboxes[1])

    expect(onSelectionChange).toHaveBeenCalledWith(new Set())
  })

  it('handles row selection - select all', () => {
    const onSelectionChange = vi.fn()
    render(
      <DataTable
        data={mockData}
        columns={mockColumns}
        rowKey={(row) => row.id}
        selectable={true}
        selectedKeys={new Set()}
        onSelectionChange={onSelectionChange}
      />
    )

    const checkboxes = screen.getAllByRole('checkbox')
    // First checkbox is "select all"
    fireEvent.click(checkboxes[0])

    expect(onSelectionChange).toHaveBeenCalledWith(new Set(['1', '2', '3']))
  })

  it('handles row selection - deselect all when all selected', () => {
    const onSelectionChange = vi.fn()
    render(
      <DataTable
        data={mockData}
        columns={mockColumns}
        rowKey={(row) => row.id}
        selectable={true}
        selectedKeys={new Set(['1', '2', '3'])}
        onSelectionChange={onSelectionChange}
      />
    )

    const checkboxes = screen.getAllByRole('checkbox')
    // First checkbox is "select all" - clicking it deselects all
    fireEvent.click(checkboxes[0])

    expect(onSelectionChange).toHaveBeenCalledWith(new Set())
  })

  it('handles row click', () => {
    const onRowClick = vi.fn()
    render(
      <DataTable
        data={mockData}
        columns={mockColumns}
        rowKey={(row) => row.id}
        onRowClick={onRowClick}
      />
    )

    const row = screen.getByText('Item 1').closest('tr')
    fireEvent.click(row!)

    expect(onRowClick).toHaveBeenCalledWith(mockData[0])
  })

  it('handles row keyboard navigation', () => {
    const onRowClick = vi.fn()
    render(
      <DataTable
        data={mockData}
        columns={mockColumns}
        rowKey={(row) => row.id}
        onRowClick={onRowClick}
      />
    )

    const row = screen.getByText('Item 1').closest('tr')

    fireEvent.keyDown(row!, { key: 'Enter' })
    expect(onRowClick).toHaveBeenCalledWith(mockData[0])

    fireEvent.keyDown(row!, { key: ' ' })
    expect(onRowClick).toHaveBeenCalledTimes(2)
  })

  it('applies sticky header class when stickyHeader is true', () => {
    render(
      <DataTable
        data={mockData}
        columns={mockColumns}
        rowKey={(row) => row.id}
        stickyHeader={true}
      />
    )

    const thead = document.querySelector('thead')
    expect(thead).toHaveClass('sticky')
  })

  it('applies custom className', () => {
    const { container } = render(
      <DataTable
        data={mockData}
        columns={mockColumns}
        rowKey={(row) => row.id}
        className="custom-class"
      />
    )

    expect(container.firstChild).toHaveClass('custom-class')
  })

  it('highlights selected rows', () => {
    render(
      <DataTable
        data={mockData}
        columns={mockColumns}
        rowKey={(row) => row.id}
        selectable={true}
        selectedKeys={new Set(['1'])}
        onSelectionChange={() => {}}
      />
    )

    const selectedRow = screen.getByText('Item 1').closest('tr')
    expect(selectedRow).toHaveClass('bg-brand-500/5')
  })

  it('does not call onSelectionChange when not provided', () => {
    // This test ensures no error when clicking selection without handler
    render(
      <DataTable
        data={mockData}
        columns={mockColumns}
        rowKey={(row) => row.id}
        selectable={true}
        selectedKeys={new Set()}
      />
    )

    const checkboxes = screen.getAllByRole('checkbox')
    // Should not throw
    fireEvent.click(checkboxes[0])
    fireEvent.click(checkboxes[1])
  })

  it('applies column width when specified', () => {
    const columnsWithWidth: Column<TestRow>[] = [
      { key: 'name', header: 'Name', cell: (row) => row.name, width: '200px' },
      { key: 'status', header: 'Status', cell: (row) => row.status },
    ]

    render(
      <DataTable
        data={mockData}
        columns={columnsWithWidth}
        rowKey={(row) => row.id}
      />
    )

    const nameHeader = screen.getByText('Name').closest('th')
    expect(nameHeader).toHaveStyle({ width: '200px' })
  })

  describe('renderDragHandle prop', () => {
    it('does not render drag handle column when prop is not provided', () => {
      render(
        <DataTable data={mockData} columns={mockColumns} rowKey={(row) => row.id} />,
      )
      const headers = document.querySelectorAll('th')
      expect(headers).toHaveLength(2)
    })

    it('renders an aria-hidden drag handle header cell when provided', () => {
      render(
        <DataTable
          data={mockData}
          columns={mockColumns}
          rowKey={(row) => row.id}
          renderDragHandle={(row) => <span data-testid={`drag-${row.id}`} />}
        />,
      )
      const headers = document.querySelectorAll('th')
      expect(headers).toHaveLength(3)
      expect(headers[0]).toHaveAttribute('aria-hidden', 'true')
    })

    it('renders drag handle cell for each data row', () => {
      render(
        <DataTable
          data={mockData}
          columns={mockColumns}
          rowKey={(row) => row.id}
          renderDragHandle={(row) => <span data-testid={`drag-${row.id}`} />}
        />,
      )
      expect(screen.getByTestId('drag-1')).toBeInTheDocument()
      expect(screen.getByTestId('drag-2')).toBeInTheDocument()
      expect(screen.getByTestId('drag-3')).toBeInTheDocument()
    })

    it('increases colSpan to include drag handle column in empty state', () => {
      const { container } = render(
        <DataTable
          data={[]}
          columns={mockColumns}
          rowKey={(row) => row.id}
          renderDragHandle={() => <span />}
        />,
      )
      const emptyTd = container.querySelector('td')
      // columns.length (2) + renderDragHandle (1) = 3
      expect(emptyTd).toHaveAttribute('colspan', '3')
    })

    it('increases colSpan further when both selectable and renderDragHandle are provided', () => {
      const { container } = render(
        <DataTable
          data={[]}
          columns={mockColumns}
          rowKey={(row) => row.id}
          selectable={true}
          selectedKeys={new Set()}
          onSelectionChange={() => {}}
          renderDragHandle={() => <span />}
        />,
      )
      const emptyTd = container.querySelector('td')
      // columns.length (2) + selectable (1) + renderDragHandle (1) = 4
      expect(emptyTd).toHaveAttribute('colspan', '4')
    })

    it('stops click propagation on drag handle cell so onRowClick is not triggered', () => {
      const onRowClick = vi.fn()
      render(
        <DataTable
          data={mockData}
          columns={mockColumns}
          rowKey={(row) => row.id}
          onRowClick={onRowClick}
          renderDragHandle={(row) => <span data-testid={`drag-${row.id}`} />}
        />,
      )
      const dragHandle = screen.getByTestId('drag-1')
      fireEvent.click(dragHandle.parentElement!)
      expect(onRowClick).not.toHaveBeenCalled()
    })

    it('stops keydown propagation on drag handle cell so onRowClick is not triggered', () => {
      const onRowClick = vi.fn()
      render(
        <DataTable
          data={mockData}
          columns={mockColumns}
          rowKey={(row) => row.id}
          onRowClick={onRowClick}
          renderDragHandle={(row) => <span data-testid={`drag-${row.id}`} />}
        />,
      )
      const dragHandle = screen.getByTestId('drag-1')
      fireEvent.keyDown(dragHandle.parentElement!, { key: 'Enter' })
      expect(onRowClick).not.toHaveBeenCalled()
    })
  })
})
