import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Search, Mail, Lock } from 'lucide-react'
import { createRef } from 'react'
import { describe, it, expect, vi } from 'vitest'

import { Input } from '../Input'

describe('Input', () => {
  it('renders correctly with default props', () => {
    render(<Input placeholder="Enter text" />)

    const input = screen.getByPlaceholderText('Enter text')
    expect(input).toBeInTheDocument()
    expect(input.tagName).toBe('INPUT')
  })

  it('renders with label', () => {
    render(<Input label="Email" id="email-input" />)

    const label = screen.getByText('Email')
    expect(label).toBeInTheDocument()
    expect(label.tagName).toBe('LABEL')
    expect(label).toHaveAttribute('for', 'email-input')
  })

  it('renders with error state and message', () => {
    render(
      <Input
        error="This field is required"
        errorTestId="input-error"
      />
    )

    const errorMessage = screen.getByTestId('input-error')
    expect(errorMessage).toBeInTheDocument()
    expect(errorMessage).toHaveTextContent('This field is required')
    expect(errorMessage).toHaveAttribute('role', 'alert')

    const input = screen.getByRole('textbox')
    expect(input).toHaveClass('border-error')
  })

  it('renders with helper text', () => {
    render(<Input helperText="Enter your email address" />)

    expect(screen.getByText('Enter your email address')).toBeInTheDocument()
  })

  it('does not show helper text when error is present', () => {
    render(
      <Input
        helperText="Helper text"
        error="Error message"
      />
    )

    expect(screen.getByText('Error message')).toBeInTheDocument()
    expect(screen.queryByText('Helper text')).not.toBeInTheDocument()
  })

  it('renders with leftIcon', () => {
    render(<Input leftIcon={Search} data-testid="input-with-left-icon" />)

    const input = screen.getByRole('textbox')
    expect(input).toHaveClass('pl-10')

    // Icon should be rendered
    const container = input.parentElement
    const icon = container?.querySelector('svg')
    expect(icon).toBeInTheDocument()
  })

  it('renders with rightIcon', () => {
    render(<Input rightIcon={Mail} data-testid="input-with-right-icon" />)

    const input = screen.getByRole('textbox')
    expect(input).toHaveClass('pr-10')
  })

  it('renders with both leftIcon and rightIcon', () => {
    render(<Input leftIcon={Search} rightIcon={Mail} />)

    const input = screen.getByRole('textbox')
    expect(input).toHaveClass('pl-10')
    expect(input).toHaveClass('pr-10')
  })

  it('renders disabled state', () => {
    render(<Input disabled placeholder="Disabled input" />)

    const input = screen.getByPlaceholderText('Disabled input')
    expect(input).toBeDisabled()
    expect(input).toHaveClass('disabled:cursor-not-allowed')
    expect(input).toHaveClass('disabled:opacity-50')
  })

  it('applies custom className', () => {
    render(<Input className="custom-class" />)

    const input = screen.getByRole('textbox')
    expect(input).toHaveClass('custom-class')
  })

  it('forwards ref correctly', () => {
    const ref = createRef<HTMLInputElement>()
    render(<Input ref={ref} />)

    expect(ref.current).toBeInstanceOf(HTMLInputElement)
  })

  it('handles password type with toggle visibility', async () => {
    const user = userEvent.setup()
    render(<Input type="password" placeholder="Enter password" />)

    const input = screen.getByPlaceholderText('Enter password')
    expect(input).toHaveAttribute('type', 'password')

    // Toggle button should be present
    const toggleButton = screen.getByRole('button', { name: /show password/i })
    expect(toggleButton).toBeInTheDocument()

    // Click to show password
    await user.click(toggleButton)
    expect(input).toHaveAttribute('type', 'text')
    expect(screen.getByRole('button', { name: /hide password/i })).toBeInTheDocument()

    // Click again to hide
    await user.click(screen.getByRole('button', { name: /hide password/i }))
    expect(input).toHaveAttribute('type', 'password')
  })

  it('does not show password toggle for non-password types', () => {
    render(<Input type="email" placeholder="Enter email" />)

    expect(screen.queryByRole('button', { name: /password/i })).not.toBeInTheDocument()
  })

  it('handles value changes', async () => {
    const user = userEvent.setup()
    const handleChange = vi.fn()
    render(<Input onChange={handleChange} placeholder="Input" />)

    const input = screen.getByPlaceholderText('Input')
    await user.type(input, 'test value')

    expect(handleChange).toHaveBeenCalled()
    expect(input).toHaveValue('test value')
  })

  it('renders password input with leftIcon', () => {
    render(<Input type="password" leftIcon={Lock} placeholder="Password" />)

    const input = screen.getByPlaceholderText('Password')
    expect(input).toHaveClass('pl-10')
    expect(input).toHaveClass('pr-10') // Password toggle adds right padding
  })

  it('prioritizes password toggle over rightIcon for password type', () => {
    render(<Input type="password" rightIcon={Mail} placeholder="Password" />)

    // Should show password toggle, not the Mail icon
    expect(screen.getByRole('button', { name: /show password/i })).toBeInTheDocument()
  })
})
