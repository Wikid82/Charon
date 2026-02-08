import '@testing-library/jest-dom/vitest'
import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import userEvent from '@testing-library/user-event'
import { Tabs, TabsList, TabsTrigger, TabsContent } from './Tabs'

describe('Tabs', () => {
  it('renders tabs container with proper role', () => {
    render(
      <Tabs defaultValue="tab1">
        <TabsList>
          <TabsTrigger value="tab1">Tab 1</TabsTrigger>
        </TabsList>
      </Tabs>
    )

    const tablist = screen.getByRole('tablist')
    expect(tablist).toBeInTheDocument()
  })

  it('renders all tabs with correct labels', () => {
    render(
      <Tabs defaultValue="tab1">
        <TabsList>
          <TabsTrigger value="tab1">First Tab</TabsTrigger>
          <TabsTrigger value="tab2">Second Tab</TabsTrigger>
          <TabsTrigger value="tab3">Third Tab</TabsTrigger>
        </TabsList>
      </Tabs>
    )

    expect(screen.getByRole('tab', { name: 'First Tab' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Second Tab' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Third Tab' })).toBeInTheDocument()
  })

  it('first tab is active by default', () => {
    render(
      <Tabs defaultValue="tab1">
        <TabsList>
          <TabsTrigger value="tab1">Tab 1</TabsTrigger>
          <TabsTrigger value="tab2">Tab 2</TabsTrigger>
        </TabsList>
      </Tabs>
    )

    const tab1 = screen.getByRole('tab', { name: 'Tab 1' })
    const tab2 = screen.getByRole('tab', { name: 'Tab 2' })

    expect(tab1).toHaveAttribute('data-state', 'active')
    expect(tab2).toHaveAttribute('data-state', 'inactive')
  })

  it('clicking tab changes active state', async () => {
    const user = userEvent.setup()
    render(
      <Tabs defaultValue="tab1">
        <TabsList>
          <TabsTrigger value="tab1">Tab 1</TabsTrigger>
          <TabsTrigger value="tab2">Tab 2</TabsTrigger>
        </TabsList>
      </Tabs>
    )

    const tab2 = screen.getByRole('tab', { name: 'Tab 2' })
    await user.click(tab2)

    expect(tab2).toHaveAttribute('data-state', 'active')
  })

  it('only one tab active at a time', async () => {
    const user = userEvent.setup()
    render(
      <Tabs defaultValue="tab1">
        <TabsList>
          <TabsTrigger value="tab1">Tab 1</TabsTrigger>
          <TabsTrigger value="tab2">Tab 2</TabsTrigger>
          <TabsTrigger value="tab3">Tab 3</TabsTrigger>
        </TabsList>
      </Tabs>
    )

    const tab1 = screen.getByRole('tab', { name: 'Tab 1' })
    const tab2 = screen.getByRole('tab', { name: 'Tab 2' })
    const tab3 = screen.getByRole('tab', { name: 'Tab 3' })

    // Initially tab1 is active
    expect(tab1).toHaveAttribute('data-state', 'active')

    // Click tab2
    await user.click(tab2)
    expect(tab2).toHaveAttribute('data-state', 'active')
    expect(tab1).toHaveAttribute('data-state', 'inactive')
    expect(tab3).toHaveAttribute('data-state', 'inactive')

    // Click tab3
    await user.click(tab3)
    expect(tab3).toHaveAttribute('data-state', 'active')
    expect(tab1).toHaveAttribute('data-state', 'inactive')
    expect(tab2).toHaveAttribute('data-state', 'inactive')
  })

  it('disabled tab cannot be clicked', async () => {
    const user = userEvent.setup()
    render(
      <Tabs defaultValue="tab1">
        <TabsList>
          <TabsTrigger value="tab1">Tab 1</TabsTrigger>
          <TabsTrigger value="tab2" disabled>Tab 2</TabsTrigger>
        </TabsList>
      </Tabs>
    )

    const tab1 = screen.getByRole('tab', { name: 'Tab 1' })
    const tab2 = screen.getByRole('tab', { name: 'Tab 2' })

    expect(tab2).toBeDisabled()
    await user.click(tab2)

    // Tab 1 should still be active
    expect(tab1).toHaveAttribute('data-state', 'active')
    expect(tab2).toHaveAttribute('data-state', 'inactive')
  })

  it('keyboard navigation with arrow keys', async () => {
    const user = userEvent.setup()
    render(
      <Tabs defaultValue="tab1">
        <TabsList>
          <TabsTrigger value="tab1">Tab 1</TabsTrigger>
          <TabsTrigger value="tab2">Tab 2</TabsTrigger>
          <TabsTrigger value="tab3">Tab 3</TabsTrigger>
        </TabsList>
      </Tabs>
    )

    const tab1 = screen.getByRole('tab', { name: 'Tab 1' })
    const tab2 = screen.getByRole('tab', { name: 'Tab 2' })

    await user.click(tab1)
    expect(tab1).toHaveFocus()

    // Arrow right should move focus and activate tab2
    await user.keyboard('{ArrowRight}')
    expect(tab2).toHaveFocus()
    expect(tab2).toHaveAttribute('data-state', 'active')
  })

  it('active tab has correct aria-selected', async () => {
    const user = userEvent.setup()
    render(
      <Tabs defaultValue="tab1">
        <TabsList>
          <TabsTrigger value="tab1">Tab 1</TabsTrigger>
          <TabsTrigger value="tab2">Tab 2</TabsTrigger>
        </TabsList>
      </Tabs>
    )

    const tab1 = screen.getByRole('tab', { name: 'Tab 1' })
    const tab2 = screen.getByRole('tab', { name: 'Tab 2' })

    expect(tab1).toHaveAttribute('aria-selected', 'true')
    expect(tab2).toHaveAttribute('aria-selected', 'false')

    await user.click(tab2)

    expect(tab1).toHaveAttribute('aria-selected', 'false')
    expect(tab2).toHaveAttribute('aria-selected', 'true')
  })

  it('tab panels show/hide based on active tab', async () => {
    const user = userEvent.setup()
    render(
      <Tabs defaultValue="tab1">
        <TabsList>
          <TabsTrigger value="tab1">Tab 1</TabsTrigger>
          <TabsTrigger value="tab2">Tab 2</TabsTrigger>
        </TabsList>
        <TabsContent value="tab1" data-testid="content1">Content 1</TabsContent>
        <TabsContent value="tab2" data-testid="content2">Content 2</TabsContent>
      </Tabs>
    )

    // Content 1 should be visible (active)
    const content1 = screen.getByTestId('content1')
    expect(content1).toBeInTheDocument()
    expect(content1).toHaveAttribute('data-state', 'active')

    // Content 2 should be hidden (inactive)
    const content2 = screen.getByTestId('content2')
    expect(content2).toBeInTheDocument()
    expect(content2).toHaveAttribute('data-state', 'inactive')

    const tab2 = screen.getByRole('tab', { name: 'Tab 2' })
    await user.click(tab2)

    // After click, content states should swap
    expect(content1).toHaveAttribute('data-state', 'inactive')
    expect(content2).toHaveAttribute('data-state', 'active')
  })

  it('custom className is applied', () => {
    render(
      <Tabs defaultValue="tab1">
        <TabsList className="custom-list-class">
          <TabsTrigger value="tab1" className="custom-trigger-class">Tab 1</TabsTrigger>
        </TabsList>
        <TabsContent value="tab1" className="custom-content-class">Content</TabsContent>
      </Tabs>
    )

    const tablist = screen.getByRole('tablist')
    const tab = screen.getByRole('tab', { name: 'Tab 1' })
    const content = screen.getByText('Content')

    expect(tablist).toHaveClass('custom-list-class')
    expect(tab).toHaveClass('custom-trigger-class')
    expect(content).toHaveClass('custom-content-class')
  })
})
