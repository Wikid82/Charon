import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';

import { CSPBuilder } from '../CSPBuilder';

describe('CSPBuilder', () => {
  const mockOnChange = vi.fn();
  const mockOnValidate = vi.fn();

  const defaultProps = {
    value: '',
    onChange: mockOnChange,
    onValidate: mockOnValidate,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render with empty directives', () => {
    render(<CSPBuilder {...defaultProps} />);

    expect(screen.getByText('Content Security Policy Builder')).toBeInTheDocument();
    expect(screen.getByText('No CSP directives configured. Add directives above to build your policy.')).toBeInTheDocument();
  });

  it('should add a directive', async () => {
    render(<CSPBuilder {...defaultProps} />);

    const valueInput = screen.getByPlaceholderText(/Enter value/);
    const addButton = screen.getByRole('button', { name: '' }); // Plus button

    fireEvent.change(valueInput, { target: { value: "'self'" } });
    fireEvent.click(addButton);

    await waitFor(() => {
      expect(mockOnChange).toHaveBeenCalled();
    });

    const callArg = mockOnChange.mock.calls[0][0];
    const parsed = JSON.parse(callArg);
    expect(parsed).toEqual([
      { directive: 'default-src', values: ["'self'"] },
    ]);
  });

  it('should remove a directive', async () => {
    const initialValue = JSON.stringify([
      { directive: 'default-src', values: ["'self'"] },
    ]);

    render(<CSPBuilder {...defaultProps} value={initialValue} />);

    await waitFor(() => {
      const directiveElements = screen.getAllByText('default-src');
      expect(directiveElements.length).toBeGreaterThan(0);
    });

    // Find the X button in the directive row (not in the select)
    const allButtons = screen.getAllByRole('button');
    const removeButton = allButtons.find(btn => {
      const svg = btn.querySelector('svg');
      return svg && btn.closest('.bg-gray-50, .dark\\:bg-gray-800');
    });

    if (removeButton) {
      fireEvent.click(removeButton);
    }

    await waitFor(() => {
      expect(mockOnChange).toHaveBeenCalled();
    });
  });

  it('should apply preset', async () => {
    render(<CSPBuilder {...defaultProps} />);

    const presetButton = screen.getByRole('button', { name: 'Strict Default' });
    fireEvent.click(presetButton);

    await waitFor(() => {
      expect(mockOnChange).toHaveBeenCalled();
    });

    const callArg = mockOnChange.mock.calls[0][0];
    const parsed = JSON.parse(callArg);
    expect(parsed.length).toBeGreaterThan(0);
    expect(parsed[0].directive).toBe('default-src');
  });

  it('should toggle preview display', () => {
    render(<CSPBuilder {...defaultProps} />);

    const previewButton = screen.getByRole('button', { name: /Show Preview/ });
    expect(screen.queryByText('Generated CSP Header:')).not.toBeInTheDocument();

    fireEvent.click(previewButton);
    expect(screen.getByRole('button', { name: /Hide Preview/ })).toBeInTheDocument();
  });

  it('should validate CSP and show warnings', async () => {
    render(<CSPBuilder {...defaultProps} />);

    // Add an unsafe directive to trigger validation
    const directiveSelect = screen.getAllByRole('combobox')[0];
    const valueInput = screen.getByPlaceholderText(/Enter value/);
    const addButton = screen.getAllByRole('button').find(btn => btn.querySelector('.lucide-plus'));

    fireEvent.change(directiveSelect, { target: { value: 'script-src' } });
    fireEvent.change(valueInput, { target: { value: "'unsafe-inline'" } });
    if (addButton) {
      fireEvent.click(addButton);
    }

    await waitFor(() => {
      expect(mockOnValidate).toHaveBeenCalled();
      const validateCall = mockOnValidate.mock.calls.find(call => call[1]?.length > 0);
      expect(validateCall).toBeDefined();
    });

    const validateCall = mockOnValidate.mock.calls.find(call => call[1]?.length > 0);
    expect(validateCall?.[0]).toBe(false);
    expect(validateCall?.[1]).toContain('Using unsafe-inline or unsafe-eval weakens CSP protection');
  });

  it('should not add duplicate values to same directive', async () => {
    const initialValue = JSON.stringify([
      { directive: 'default-src', values: ["'self'"] },
    ]);

    render(<CSPBuilder {...defaultProps} value={initialValue} />);

    const valueInput = screen.getByPlaceholderText(/Enter value/);
    const allButtons = screen.getAllByRole('button');
    const addButton = allButtons.find(btn => btn.querySelector('.lucide-plus'));

    // Try to add the same value again
    fireEvent.change(valueInput, { target: { value: "'self'" } });
    if (addButton) {
      fireEvent.click(addButton);
    }

    await waitFor(() => {
      // Should not call onChange since it's a duplicate
      const calls = mockOnChange.mock.calls.filter(call => {
        const parsed = JSON.parse(call[0]);
        return parsed[0].values.filter((v: string) => v === "'self'").length > 1;
      });
      expect(calls.length).toBe(0);
    });
  });

  it('should parse initial value correctly', () => {
    const initialValue = JSON.stringify([
      { directive: 'default-src', values: ["'self'", 'https:'] },
      { directive: 'script-src', values: ["'self'"] },
    ]);

    render(<CSPBuilder {...defaultProps} value={initialValue} />);

    // Use getAllByText since these appear in both the select and the directive list
    const defaultSrcElements = screen.getAllByText('default-src');
    expect(defaultSrcElements.length).toBeGreaterThan(0);

    const scriptSrcElements = screen.getAllByText('script-src');
    expect(scriptSrcElements.length).toBeGreaterThan(0);

    const selfElements = screen.getAllByText("'self'");
    expect(selfElements.length).toBeGreaterThan(0);
  });

  it('should change directive selector', () => {
    render(<CSPBuilder {...defaultProps} />);

    // Get the first combobox (the directive selector)
    const allSelects = screen.getAllByRole('combobox');
    const directiveSelect = allSelects[0];
    fireEvent.change(directiveSelect, { target: { value: 'script-src' } });

    expect(directiveSelect).toHaveValue('script-src');
  });

  it('should handle Enter key to add directive', async () => {
    render(<CSPBuilder {...defaultProps} />);

    const valueInput = screen.getByPlaceholderText(/Enter value/);

    fireEvent.change(valueInput, { target: { value: "'self'" } });
    fireEvent.keyDown(valueInput, { key: 'Enter', code: 'Enter' });

    await waitFor(() => {
      expect(mockOnChange).toHaveBeenCalled();
    });
  });

  it('should not add empty values', () => {
    render(<CSPBuilder {...defaultProps} />);

    const addButton = screen.getByRole('button', { name: '' });
    fireEvent.click(addButton);

    expect(mockOnChange).not.toHaveBeenCalled();
  });

  it('should remove individual values from directive', async () => {
    const initialValue = JSON.stringify([
      { directive: 'default-src', values: ["'self'", 'https:', 'data:'] },
    ]);

    render(<CSPBuilder {...defaultProps} value={initialValue} />);

    const selfBadge = screen.getByText("'self'");
    fireEvent.click(selfBadge);

    await waitFor(() => {
      expect(mockOnChange).toHaveBeenCalled();
    });

    const callArg = mockOnChange.mock.calls[mockOnChange.mock.calls.length - 1][0];
    const parsed = JSON.parse(callArg);
    expect(parsed[0].values).not.toContain("'self'");
    expect(parsed[0].values).toContain('https:');
  });

  it('should show success alert when valid', async () => {
    const validValue = JSON.stringify([
      { directive: 'default-src', values: ["'self'"] },
    ]);

    render(<CSPBuilder {...defaultProps} value={validValue} />);

    await waitFor(() => {
      expect(screen.getByText('CSP configuration looks good!')).toBeInTheDocument();
    });
  });
});
