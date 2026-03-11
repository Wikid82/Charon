import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { AlertCircle } from 'lucide-react'
import { describe, it, expect, vi } from 'vitest'

import { Alert, AlertTitle, AlertDescription } from '../Alert'

describe('Alert', () => {
  it('renders with default variant', () => {
    render(<Alert>Default alert content</Alert>)

    const alert = screen.getByRole('alert')
    expect(alert).toBeInTheDocument()
    expect(alert).toHaveClass('bg-surface-subtle')
    expect(screen.getByText('Default alert content')).toBeInTheDocument()
  })

  it('renders with info variant', () => {
    render(<Alert variant="info">Info message</Alert>)

    const alert = screen.getByRole('alert')
    expect(alert).toHaveClass('bg-info-muted')
    expect(alert).toHaveClass('border-info/30')
  })

  it('renders with success variant', () => {
    render(<Alert variant="success">Success message</Alert>)

    const alert = screen.getByRole('alert')
    expect(alert).toHaveClass('bg-success-muted')
    expect(alert).toHaveClass('border-success/30')
  })

  it('renders with warning variant', () => {
    render(<Alert variant="warning">Warning message</Alert>)

    const alert = screen.getByRole('alert')
    expect(alert).toHaveClass('bg-warning-muted')
    expect(alert).toHaveClass('border-warning/30')
  })

  it('renders with error variant', () => {
    render(<Alert variant="error">Error message</Alert>)

    const alert = screen.getByRole('alert')
    expect(alert).toHaveClass('bg-error-muted')
    expect(alert).toHaveClass('border-error/30')
  })

  it('renders with title', () => {
    render(<Alert title="Alert Title">Alert content</Alert>)

    expect(screen.getByText('Alert Title')).toBeInTheDocument()
    expect(screen.getByText('Alert content')).toBeInTheDocument()
  })

  it('renders dismissible alert with dismiss button', () => {
    const onDismiss = vi.fn()
    render(
      <Alert dismissible onDismiss={onDismiss}>
        Dismissible alert
      </Alert>
    )

    const dismissButton = screen.getByRole('button', { name: /dismiss alert/i })
    expect(dismissButton).toBeInTheDocument()
  })

  it('calls onDismiss and hides alert when dismiss button is clicked', () => {
    const onDismiss = vi.fn()
    render(
      <Alert dismissible onDismiss={onDismiss}>
        Dismissible alert
      </Alert>
    )

    const dismissButton = screen.getByRole('button', { name: /dismiss alert/i })
    fireEvent.click(dismissButton)

    expect(onDismiss).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('hides alert on dismiss without onDismiss callback', () => {
    render(
      <Alert dismissible>
        Dismissible alert
      </Alert>
    )

    const dismissButton = screen.getByRole('button', { name: /dismiss alert/i })
    fireEvent.click(dismissButton)

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('renders with custom icon', () => {
    render(
      <Alert icon={AlertCircle} data-testid="alert-with-icon">
        Alert with custom icon
      </Alert>
    )

    const alert = screen.getByTestId('alert-with-icon')
    // Custom icon should be rendered (AlertCircle)
    const iconContainer = alert.querySelector('svg')
    expect(iconContainer).toBeInTheDocument()
  })

  it('renders default icon based on variant', () => {
    render(<Alert variant="error">Error alert</Alert>)

    const alert = screen.getByRole('alert')
    // Error variant uses XCircle icon
    const icon = alert.querySelector('svg')
    expect(icon).toBeInTheDocument()
    expect(icon).toHaveClass('text-error')
  })

  it('applies custom className', () => {
    render(<Alert className="custom-class">Alert content</Alert>)

    const alert = screen.getByRole('alert')
    expect(alert).toHaveClass('custom-class')
  })

  it('does not render dismiss button when not dismissible', () => {
    render(<Alert>Non-dismissible alert</Alert>)

    expect(screen.queryByRole('button', { name: /dismiss/i })).not.toBeInTheDocument()
  })
})

describe('AlertTitle', () => {
  it('renders correctly', () => {
    render(<AlertTitle>Test Title</AlertTitle>)

    const title = screen.getByText('Test Title')
    expect(title).toBeInTheDocument()
    expect(title.tagName).toBe('H5')
    expect(title).toHaveClass('font-semibold')
  })

  it('applies custom className', () => {
    render(<AlertTitle className="custom-class">Title</AlertTitle>)

    const title = screen.getByText('Title')
    expect(title).toHaveClass('custom-class')
  })
})

describe('AlertDescription', () => {
  it('renders correctly', () => {
    render(<AlertDescription>Test Description</AlertDescription>)

    const description = screen.getByText('Test Description')
    expect(description).toBeInTheDocument()
    expect(description.tagName).toBe('P')
    expect(description).toHaveClass('text-sm')
  })

  it('applies custom className', () => {
    render(<AlertDescription className="custom-class">Description</AlertDescription>)

    const description = screen.getByText('Description')
    expect(description).toHaveClass('custom-class')
  })
})

describe('Alert composition', () => {
  it('works with AlertTitle and AlertDescription subcomponents', () => {
    render(
      <Alert>
        <AlertTitle>Composed Title</AlertTitle>
        <AlertDescription>Composed description text</AlertDescription>
      </Alert>
    )

    expect(screen.getByText('Composed Title')).toBeInTheDocument()
    expect(screen.getByText('Composed description text')).toBeInTheDocument()
  })
})
