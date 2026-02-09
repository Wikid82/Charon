import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { PermissionsPolicyBuilder } from '../PermissionsPolicyBuilder';
import userEvent from '@testing-library/user-event';

describe('PermissionsPolicyBuilder', () => {
  const defaultProps = {
    value: '',
    onChange: vi.fn(),
  };

  it('renders correctly with empty value', () => {
    render(<PermissionsPolicyBuilder {...defaultProps} />);

    expect(screen.getByText('Permissions Policy Builder')).toBeInTheDocument();
    expect(screen.getByText('No permissions policies configured. Add features above to restrict browser capabilities.')).toBeInTheDocument();
  });

  it('renders correctly with initial value', () => {
    const initialValue = JSON.stringify([
      { feature: 'camera', allowlist: [] },
      { feature: 'microphone', allowlist: ['self'] },
    ]);

    render(<PermissionsPolicyBuilder {...defaultProps} value={initialValue} />);

    expect(screen.getByRole('button', { name: 'Remove camera' })).toBeInTheDocument();
    expect(screen.getByText('Disabled')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Remove microphone' })).toBeInTheDocument();
    expect(screen.getByText('Self only')).toBeInTheDocument();
  });

  it('adds a new feature (disabled)', async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<PermissionsPolicyBuilder {...defaultProps} onChange={onChange} />);

    // Select feature 'geolocation'
    await user.selectOptions(screen.getByRole('combobox', { name: /select feature/i }), 'geolocation');

    // Select allowlist 'None' (default, but explicit check)
    // Value is ''

    // Click Add
    await user.click(screen.getByRole('button', { name: 'Add Feature' }));

    expect(onChange).toHaveBeenCalledWith(expect.stringContaining('"feature":"geolocation"'));
    expect(onChange).toHaveBeenCalledWith(expect.stringContaining('"allowlist":[]'));
  });

  it('adds a feature with custom origin', async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<PermissionsPolicyBuilder {...defaultProps} onChange={onChange} />);

    // To enter custom origin, value should be '' (None). It is default.
    // Enter origin. The input is visible.
    const customInput = screen.getByPlaceholderText('or enter origin (e.g., https://example.com)');
    await user.type(customInput, 'https://trusted.com');

    await user.selectOptions(screen.getByRole('combobox', { name: /select feature/i }), 'usb');

    await user.click(screen.getByRole('button', { name: 'Add Feature' }));

    expect(onChange).toHaveBeenCalledWith(expect.stringContaining('"feature":"usb"'));
    expect(onChange).toHaveBeenCalledWith(expect.stringContaining('"allowlist":["https://trusted.com"]'));
  });

  it('removes a feature', async () => {
    const onChange = vi.fn();
    const initialValue = JSON.stringify([
      { feature: 'camera', allowlist: [] }
    ]);
    const user = userEvent.setup();

    render(<PermissionsPolicyBuilder {...defaultProps} value={initialValue} onChange={onChange} />);

    expect(screen.getByRole('button', { name: 'Remove camera' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Remove camera' }));

    expect(onChange).toHaveBeenCalledWith('[]');
  });

  it('handles quick add', async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<PermissionsPolicyBuilder {...defaultProps} onChange={onChange} />);

    await user.click(screen.getByText('Disable Common Features'));

    expect(onChange).toHaveBeenCalledWith(expect.stringMatching(/camera/));
    expect(onChange).toHaveBeenCalledWith(expect.stringMatching(/microphone/));
    expect(onChange).toHaveBeenCalledWith(expect.stringMatching(/geolocation/));
  });

  it('updates existing feature if added again', async () => {
    const onChange = vi.fn();
    const initialValue = JSON.stringify([
      { feature: 'camera', allowlist: [] }
    ]);
    const user = userEvent.setup();

    render(<PermissionsPolicyBuilder {...defaultProps} value={initialValue} onChange={onChange} />);

    await user.selectOptions(screen.getByRole('combobox', { name: /select feature/i }), 'camera');
    await user.selectOptions(screen.getByRole('combobox', { name: /select allowlist origin/i }), 'self');

    await user.click(screen.getByRole('button', { name: 'Add Feature' }));

    expect(onChange).toHaveBeenCalledWith(expect.stringContaining('"feature":"camera"'));
    expect(onChange).toHaveBeenCalledWith(expect.stringContaining('"allowlist":["self"]'));
  });

  it('toggles preview', async () => {
    const initialValue = JSON.stringify([
      { feature: 'camera', allowlist: [] }
    ]);
    const user = userEvent.setup();
    render(<PermissionsPolicyBuilder {...defaultProps} value={initialValue} />);

    const toggleBtn = screen.getByText('Show Preview');
    await user.click(toggleBtn);

    expect(screen.getByText('Generated Permissions-Policy Header:')).toBeInTheDocument();
    expect(screen.getByText(/camera=\(\)/)).toBeInTheDocument();

    await user.click(screen.getByText('Hide Preview'));
    expect(screen.queryByText('Generated Permissions-Policy Header:')).not.toBeInTheDocument();
  });
});
