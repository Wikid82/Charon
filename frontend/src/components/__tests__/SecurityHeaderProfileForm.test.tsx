import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi } from 'vitest';
import { SecurityHeaderProfileForm } from '../SecurityHeaderProfileForm';
import { securityHeadersApi, type SecurityHeaderProfile } from '../../api/securityHeaders';

vi.mock('../../api/securityHeaders');

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe('SecurityHeaderProfileForm', () => {
  const mockOnSubmit = vi.fn();
  const mockOnCancel = vi.fn();
  const mockOnDelete = vi.fn();

  const defaultProps = {
    onSubmit: mockOnSubmit,
    onCancel: mockOnCancel,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(securityHeadersApi.calculateScore).mockResolvedValue({
      score: 85,
      max_score: 100,
      breakdown: {},
      suggestions: [],
    });
  });

  it('should render with empty form', () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    expect(screen.getByPlaceholderText(/Production Security Headers/)).toBeInTheDocument();
    expect(screen.getByText('HTTP Strict Transport Security (HSTS)')).toBeInTheDocument();
  });

  it('should render with initial data', () => {
    const initialData: Partial<SecurityHeaderProfile> = {
      id: 1,
      name: 'Test Profile',
      description: 'Test description',
      hsts_enabled: true,
      hsts_max_age: 31536000,
      security_score: 85,
    };

    render(
      <SecurityHeaderProfileForm {...defaultProps} initialData={initialData as SecurityHeaderProfile} />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByDisplayValue('Test Profile')).toBeInTheDocument();
    expect(screen.getByDisplayValue('Test description')).toBeInTheDocument();
  });

  it('should submit form with valid data', async () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    const nameInput = screen.getByPlaceholderText(/Production Security Headers/);
    fireEvent.change(nameInput, { target: { value: 'New Profile' } });

    const submitButton = screen.getByRole('button', { name: /Save Profile/ });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalled();
    });

    const submitData = mockOnSubmit.mock.calls[0][0];
    expect(submitData.name).toBe('New Profile');
  });

  it('should not submit with empty name', () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    const submitButton = screen.getByRole('button', { name: /Save Profile/ });
    fireEvent.click(submitButton);

    expect(mockOnSubmit).not.toHaveBeenCalled();
  });

  it('should call onCancel when cancel button clicked', () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    const cancelButton = screen.getByRole('button', { name: /Cancel/ });
    fireEvent.click(cancelButton);

    expect(mockOnCancel).toHaveBeenCalled();
  });

  it('should toggle HSTS enabled', async () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    // Switch component uses checkbox with sr-only class
    const hstsSection = screen.getByText('HTTP Strict Transport Security (HSTS)').closest('div');
    const hstsToggle = hstsSection?.querySelector('input[type="checkbox"]') as HTMLInputElement;

    expect(hstsToggle).toBeTruthy();
    expect(hstsToggle.checked).toBe(true);

    fireEvent.click(hstsToggle);
    expect(hstsToggle.checked).toBe(false);
  });

  it('should show HSTS options when enabled', () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    expect(screen.getByText(/Max Age \(seconds\)/)).toBeInTheDocument();
    expect(screen.getByText('Include Subdomains')).toBeInTheDocument();
    expect(screen.getByText('Preload')).toBeInTheDocument();
  });

  it('should show preload warning when enabled', async () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    // Find the preload switch by finding the parent container with the "Preload" label
    const preloadText = screen.getByText('Preload');
    const preloadContainer = preloadText.closest('div')?.parentElement; // Go up to the flex container
    const preloadSwitch = preloadContainer?.querySelector('input[type="checkbox"]');

    expect(preloadSwitch).toBeTruthy();

    if (preloadSwitch) {
      fireEvent.click(preloadSwitch);
    }

    await waitFor(() => {
      expect(screen.getByText(/Warning: HSTS Preload is Permanent/)).toBeInTheDocument();
    });
  });

  it('should toggle CSP enabled', async () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    // CSP is disabled by default, so builder should not be visible
    expect(screen.queryByText('Content Security Policy Builder')).not.toBeInTheDocument();

    // Find and click the CSP toggle switch (checkbox with sr-only class)
    const cspSection = screen.getByText('Content Security Policy (CSP)').closest('div');
    const cspCheckbox = cspSection?.querySelector('input[type="checkbox"]');

    if (cspCheckbox) {
      fireEvent.click(cspCheckbox);
    }

    // Builder should now be visible
    await waitFor(() => {
      expect(screen.getByText('Content Security Policy Builder')).toBeInTheDocument();
    });
  });

  it('should disable form for presets', () => {
    const presetData: Partial<SecurityHeaderProfile> = {
      id: 1,
      name: 'Basic Security',
      is_preset: true,
      preset_type: 'basic',
      security_score: 65,
    };

    render(
      <SecurityHeaderProfileForm {...defaultProps} initialData={presetData as SecurityHeaderProfile} />,
      { wrapper: createWrapper() }
    );

    const nameInput = screen.getByPlaceholderText(/Production Security Headers/);
    expect(nameInput).toBeDisabled();

    expect(screen.getByText(/This is a system preset and cannot be modified/)).toBeInTheDocument();
  });

  it('should show delete button for non-presets', () => {
    const profileData: Partial<SecurityHeaderProfile> = {
      id: 1,
      name: 'Custom Profile',
      is_preset: false,
      security_score: 80,
    };

    render(
      <SecurityHeaderProfileForm
        {...defaultProps}
        initialData={profileData as SecurityHeaderProfile}
        onDelete={mockOnDelete}
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByRole('button', { name: /Delete Profile/ })).toBeInTheDocument();
  });

  it('should not show delete button for presets', () => {
    const presetData: Partial<SecurityHeaderProfile> = {
      id: 1,
      name: 'Basic Security',
      is_preset: true,
      preset_type: 'basic',
      security_score: 65,
    };

    render(
      <SecurityHeaderProfileForm
        {...defaultProps}
        initialData={presetData as SecurityHeaderProfile}
        onDelete={mockOnDelete}
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.queryByRole('button', { name: /Delete Profile/ })).not.toBeInTheDocument();
  });

  it('should change referrer policy', () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    const referrerSelect = screen.getAllByRole('combobox')[1]; // Referrer policy is second select
    fireEvent.change(referrerSelect, { target: { value: 'no-referrer' } });

    expect(referrerSelect).toHaveValue('no-referrer');
  });

  it('should change x-frame-options', () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    const xfoSelect = screen.getAllByRole('combobox')[0]; // X-Frame-Options is first select
    fireEvent.change(xfoSelect, { target: { value: 'SAMEORIGIN' } });

    expect(xfoSelect).toHaveValue('SAMEORIGIN');
  });

  it('should show loading state', () => {
    render(<SecurityHeaderProfileForm {...defaultProps} isLoading={true} />, { wrapper: createWrapper() });

    expect(screen.getByText('Saving...')).toBeInTheDocument();
  });

  it('should show deleting state', () => {
    const profileData: Partial<SecurityHeaderProfile> = {
      id: 1,
      name: 'Custom Profile',
      is_preset: false,
      security_score: 80,
    };

    render(
      <SecurityHeaderProfileForm
        {...defaultProps}
        initialData={profileData as SecurityHeaderProfile}
        onDelete={mockOnDelete}
        isDeleting={true}
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByText('Deleting...')).toBeInTheDocument();
  });

  it('should calculate security score on form changes', async () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    const nameInput = screen.getByPlaceholderText(/Production Security Headers/);
    fireEvent.change(nameInput, { target: { value: 'Test' } });

    await waitFor(() => {
      expect(securityHeadersApi.calculateScore).toHaveBeenCalled();
    }, { timeout: 1000 });
  });
});
