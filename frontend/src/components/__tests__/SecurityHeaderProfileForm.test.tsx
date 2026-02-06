import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, it, expect, vi } from 'vitest';
import { SecurityHeaderProfileForm } from '../SecurityHeaderProfileForm';
import { securityHeadersApi, type SecurityHeaderProfile } from '../../api/securityHeaders';

vi.mock('../../api/securityHeaders');

// Mock child components that are complex or have their own tests
vi.mock('../CSPBuilder', () => ({
  CSPBuilder: ({
    value,
    onChange,
    onValidate,
  }: {
    value: string;
    onChange: (v: string) => void;
    onValidate: (v: boolean, e: string[]) => void;
  }) => (
    <div data-testid="csp-builder">
      <input
        data-testid="csp-input"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      <button type="button" data-testid="csp-valid" onClick={() => onValidate(true, [])}>
        Set Valid
      </button>
      <button type="button" data-testid="csp-invalid" onClick={() => onValidate(false, ['Error'])}>
        Set Invalid
      </button>
    </div>
  ),
}));

vi.mock('../PermissionsPolicyBuilder', () => ({
  PermissionsPolicyBuilder: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (v: string) => void;
  }) => (
    <div data-testid="permissions-builder">
      <input
        data-testid="permissions-input"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </div>
  ),
}));

vi.mock('../SecurityScoreDisplay', () => ({
  SecurityScoreDisplay: ({
    score,
    maxScore,
  }: {
    score: number;
    maxScore: number;
  }) => (
    <div data-testid="security-score">
      Score: {score}/{maxScore}
    </div>
  ),
}));

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
    expect(
      screen.getByText('Profile Information')
    ).toBeInTheDocument();
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
      <SecurityHeaderProfileForm
        {...defaultProps}
        initialData={initialData as SecurityHeaderProfile}
      />,
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

  it('should call onDelete when delete button clicked', () => {
    render(
      <SecurityHeaderProfileForm
        {...defaultProps}
        initialData={{ id: 1, name: 'Test', is_preset: false } as SecurityHeaderProfile}
        onDelete={mockOnDelete}
      />,
      { wrapper: createWrapper() }
    );

    const deleteButton = screen.getByRole('button', { name: /Delete Profile/ });
    fireEvent.click(deleteButton);

    expect(mockOnDelete).toHaveBeenCalled();
  });

  it('should toggle HSTS enabled', async () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    // HSTS is true by default
    const hstsSection = screen
      .getByText('HTTP Strict Transport Security (HSTS)')
      .closest('div');
    const hstsToggle = hstsSection?.querySelector(
      'input[type="checkbox"]'
    ) as HTMLInputElement;

    expect(hstsToggle).toBeTruthy();
    expect(hstsToggle.checked).toBe(true);

    fireEvent.click(hstsToggle);
    expect(hstsToggle.checked).toBe(false);
  });

  it('should show HSTS options when enabled and handle updates', async () => {
    const initialData: Partial<SecurityHeaderProfile> = {
      hsts_enabled: true,
      hsts_max_age: 1000,
    };

    render(
      <SecurityHeaderProfileForm {...defaultProps} initialData={initialData as SecurityHeaderProfile} />,
      { wrapper: createWrapper() }
    );

    const maxAgeInput = screen.getByDisplayValue('1000');
    fireEvent.change(maxAgeInput, { target: { value: '63072000' } });

    // Try include subdomains toggle
    const includeSubdomainsText = screen.getByText('Include Subdomains');
    const includeSubdomainsContainer = includeSubdomainsText.closest('div')?.parentElement;
    const includeSubdomainsToggle = includeSubdomainsContainer?.querySelector('input[type="checkbox"]');

    if(includeSubdomainsToggle) {
        fireEvent.click(includeSubdomainsToggle);
    }

    // Check submit gets updated values
    const nameInput = screen.getByPlaceholderText(/Production Security Headers/);
    fireEvent.change(nameInput, { target: { value: 'HSTS Update' } });

    const submitButton = screen.getByRole('button', { name: /Save Profile/ });
    fireEvent.click(submitButton);

    await waitFor(() => {
        const submitted = mockOnSubmit.mock.calls[0][0];
        expect(submitted.hsts_max_age).toBe(63072000);
    });
  });

  it('should show preload warning when enabled', async () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    const preloadText = screen.getByText('Preload');
    const preloadContainer = preloadText.closest('div')?.parentElement;
    const preloadSwitch = preloadContainer?.querySelector('input[type="checkbox"]');

    if (preloadSwitch) {
      fireEvent.click(preloadSwitch);
    }

    await waitFor(() => {
      expect(screen.getByText(/Warning: HSTS Preload is Permanent/)).toBeInTheDocument();
    });
  });

  it('should toggle CSP enabled and show CSP builder', async () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    const cspSection = screen
      .getByText('Content Security Policy (CSP)')
      .closest('div');
    const cspCheckbox = cspSection?.querySelector('input[type="checkbox"]');

    if (cspCheckbox) {
      fireEvent.click(cspCheckbox); // Enable CSP (default is false)
    }

    await waitFor(() => {
      expect(screen.getByTestId('csp-builder')).toBeInTheDocument();
    });

    // Test that submit button is disabled when CSP is invalid
    const invalidButton = screen.getByTestId('csp-invalid');
    fireEvent.click(invalidButton);

    const submitButton = screen.getByRole('button', { name: /Save Profile/ });
    expect(submitButton).toBeDisabled();

    // Re-enable
    const validButton = screen.getByTestId('csp-valid');
    fireEvent.click(validButton);
    expect(submitButton).not.toBeDisabled();

    // Update CSP value through mock
    const cspInput = screen.getByTestId('csp-input');
    fireEvent.change(cspInput, { target: { value: '{"test": "val"}' } });
  });

  it('should handle CSP report only URI', async () => {
    const initialData: Partial<SecurityHeaderProfile> = {
      csp_enabled: true,
      csp_report_only: true, // Report only enabled
    };

    render(
      <SecurityHeaderProfileForm {...defaultProps} initialData={initialData as SecurityHeaderProfile} />,
      { wrapper: createWrapper() }
    );

    const reportUriInput = screen.getByPlaceholderText(/example.com\/csp-report/);
    fireEvent.change(reportUriInput, { target: { value: 'https://test.com/report' } });

    expect(reportUriInput).toHaveValue('https://test.com/report');

    // Verify toggle for report only
    const reportOnlyText = screen.getByText('Report-Only Mode');
    const reportOnlyContainer = reportOnlyText.closest('div')?.parentElement;
    const reportOnlySwitch = reportOnlyContainer?.querySelector('input[type="checkbox"]');

    if(reportOnlySwitch) {
        fireEvent.click(reportOnlySwitch); // Disable
        expect(screen.queryByPlaceholderText(/example.com\/csp-report/)).not.toBeInTheDocument();
    }
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
      <SecurityHeaderProfileForm
        {...defaultProps}
        initialData={presetData as SecurityHeaderProfile}
      />,
      { wrapper: createWrapper() }
    );

    const nameInput = screen.getByPlaceholderText(/Production Security Headers/);
    expect(nameInput).toBeDisabled();
    expect(
      screen.getByText(/This is a system preset and cannot be modified/)
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Delete Profile/ })).not.toBeInTheDocument();
  });

  it('should handle cross origin policies', () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    // Use traversing to find selects since labels are not associated
    // Order: X-Frame, Referrer, Opener, Resource, Embedder
    const selects = screen.getAllByRole('combobox');

    // Verify we have the expected number of selects (5 standard + potential others?)
    // X-Frame-Options is index 0
    // Referrer-Policy is index 1
    // Cross-Origin-Opener-Policy is index 2
    // Cross-Origin-Resource-Policy is index 3
    // Cross-Origin-Embedder-Policy is index 4

    expect(selects.length).toBeGreaterThanOrEqual(5);

    const openerPolicy = selects[2];
    expect(openerPolicy).toHaveValue('same-origin');
    fireEvent.change(openerPolicy, { target: { value: 'unsafe-none' } });
    expect(openerPolicy).toHaveValue('unsafe-none');

    const resourcePolicy = selects[3];
    expect(resourcePolicy).toHaveValue('same-origin');
    fireEvent.change(resourcePolicy, { target: { value: 'same-site' } });
    expect(resourcePolicy).toHaveValue('same-site');

    const embedderPolicy = selects[4];
    // Default is likely empty string per component default
    fireEvent.change(embedderPolicy, { target: { value: 'require-corp' } });
    expect(embedderPolicy).toHaveValue('require-corp');
  });

  it('should handle additional options', () => {
    // xss_protection defaults to true
    // cache_control_no_store defaults to false

    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    const xssSection = screen.getByText('X-XSS-Protection').closest('div')?.parentElement;
    const xssSwitch = xssSection?.querySelector('input[type="checkbox"]');
    expect(xssSwitch).toBeChecked(); // Default true

    if(xssSwitch) fireEvent.click(xssSwitch);
    expect(xssSwitch).not.toBeChecked();

    const cacheSection = screen.getByText('Cache-Control: no-store').closest('div')?.parentElement;
    const cacheSwitch = cacheSection?.querySelector('input[type="checkbox"]');
    expect(cacheSwitch).not.toBeChecked(); // Default false

    if(cacheSwitch) fireEvent.click(cacheSwitch);
    expect(cacheSwitch).toBeChecked();
  });

  it('should update permissions policy', () => {
     render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

     const permissionsInput = screen.getByTestId('permissions-input');
     fireEvent.change(permissionsInput, { target: { value: 'geolocation=()' } });

     const nameInput = screen.getByPlaceholderText(/Production Security Headers/);
     fireEvent.change(nameInput, { target: { value: 'PP Update' } });

     const submitButton = screen.getByRole('button', { name: /Save Profile/ });
     fireEvent.click(submitButton);

     const submitted = mockOnSubmit.mock.calls[0][0];
     expect(submitted.permissions_policy).toBe('geolocation=()');
  });

  it('should show security score', async () => {
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    await waitFor(() => {
        expect(screen.getByTestId('security-score')).toBeInTheDocument();
        expect(screen.getByText('Score: 85/100')).toBeInTheDocument();
    });
  });

  it('should calculate score after debounce', async () => {
    // Use real timers for simplicity with debounce
    render(<SecurityHeaderProfileForm {...defaultProps} />, { wrapper: createWrapper() });

    // Clear initial calls from mount
    vi.clearAllMocks();

    const nameInput = screen.getByPlaceholderText(/Production Security Headers/);
    fireEvent.change(nameInput, { target: { value: 'Checking Debounce' } });

    // Should not have called immediately
    expect(securityHeadersApi.calculateScore).not.toHaveBeenCalled();

    // Wait for debounce (500ms) + buffer
    await waitFor(() => {
      expect(securityHeadersApi.calculateScore).toHaveBeenCalled();
    }, { timeout: 1500 });
  });
});
