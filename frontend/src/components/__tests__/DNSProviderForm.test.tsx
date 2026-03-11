import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import DNSProviderForm from '../DNSProviderForm';

// Mock the hooks
const mockCreateMutation = {
  mutateAsync: vi.fn(),
  isPending: false,
};
const mockUpdateMutation = {
  mutateAsync: vi.fn(),
  isPending: false,
};
const mockTestCredentialsMutation = {
  mutateAsync: vi.fn(),
  isPending: false,
};
const mockEnableMultiCredentialsMutation = {
  mutateAsync: vi.fn(),
  isPending: false,
};

vi.mock('../../hooks/useDNSProviders', () => ({
  useDNSProviderTypes: vi.fn(() => ({
    data: [
      {
        type: 'cloudflare',
        name: 'Cloudflare',
        fields: [
          { name: 'api_token', label: 'API Token', type: 'password', required: true }
        ]
      },
      {
        type: 'route53',
        name: 'Route53',
        fields: [
            { name: 'access_key_id', label: 'Access Key ID', type: 'text', required: true },
            { name: 'secret_access_key', label: 'Secret Access Key', type: 'password', required: true }
        ]
      }
    ],
    isLoading: false,
  })),
  useDNSProviderMutations: vi.fn(() => ({
    createMutation: mockCreateMutation,
    updateMutation: mockUpdateMutation,
    testCredentialsMutation: mockTestCredentialsMutation,
  })),
}));

vi.mock('../../hooks/useCredentials', () => ({
  useEnableMultiCredentials: vi.fn(() => mockEnableMultiCredentialsMutation),
  useCredentials: vi.fn(() => ({
    data: [],
  })),
}));

// Mock CredentialManager component to avoid complex nested testing
vi.mock('../CredentialManager', () => ({
  default: () => <div data-testid="credential-manager">Credential Manager Mock</div>,
}));

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'dnsProviders.addProvider': 'Add DNS Provider',
        'dnsProviders.editProvider': 'Edit DNS Provider',
        'dnsProviders.providerName': 'Provider Name',
        'dnsProviders.providerType': 'Provider Type',
        'dnsProviders.propagationTimeout': 'Propagation Timeout (seconds)',
        'dnsProviders.pollingInterval': 'Polling Interval (seconds)',
        'dnsProviders.setAsDefault': 'Set as default provider',
        'dnsProviders.advancedSettings': 'Advanced Settings',
        'dnsProviders.testConnection': 'Test Connection',
        'dnsProviders.testSuccess': 'Connection test successful',
        'dnsProviders.testFailed': 'Connection test failed',
        'common.create': 'Create',
        'common.update': 'Update',
        'common.cancel': 'Cancel',
      };
      return translations[key] || key;
    },
  }),
}));

describe('DNSProviderForm', () => {
  const defaultProps = {
    open: true,
    onOpenChange: vi.fn(),
    onSuccess: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders correctly in add mode', () => {
    render(<DNSProviderForm {...defaultProps} />);

    expect(screen.getByText('Add DNS Provider')).toBeInTheDocument();
    expect(screen.getByLabelText('Provider Name')).toBeInTheDocument();
    // Use role to find the trigger specifically
    expect(screen.getByRole('combobox', { name: 'Provider Type' })).toBeInTheDocument();
  });

  it('populates fields when editing', async () => {
    const provider = {
      id: 1,
      uuid: 'prov-uuid',
      name: 'My Cloudflare',
      provider_type: 'cloudflare' as const,
      is_default: true,
      enabled: true,
      propagation_timeout: 180,
      polling_interval: 10,
      has_credentials: true,
      success_count: 0,
      failure_count: 0,
      created_at: '2023-01-01',
      updated_at: '2023-01-01',
    };

    render(<DNSProviderForm {...defaultProps} provider={provider} />);

    expect(screen.getByText('Edit DNS Provider')).toBeInTheDocument();
    expect(screen.getByDisplayValue('My Cloudflare')).toBeInTheDocument();

    await waitFor(() => {
        expect(screen.getByLabelText('API Token')).toBeInTheDocument();
    });
  });

  it('handles form submission for creation', async () => {
    const user = userEvent.setup();
    render(<DNSProviderForm {...defaultProps} />);

    await user.type(screen.getByLabelText('Provider Name'), 'New Provider');

    const typeSelectTrigger = screen.getByRole('combobox', { name: 'Provider Type' });
    await user.click(typeSelectTrigger);

    // Select option by role to distinguish from trigger text
    await user.click(screen.getByRole('option', { name: 'Cloudflare' }));

    const tokenInput = await screen.findByLabelText('API Token');
    await user.type(tokenInput, 'my-token');

    mockCreateMutation.mutateAsync.mockResolvedValue({});
    await user.click(screen.getByRole('button', { name: 'Create' }));

    expect(mockCreateMutation.mutateAsync).toHaveBeenCalledWith(expect.objectContaining({
      name: 'New Provider',
      provider_type: 'cloudflare',
      credentials: { api_token: 'my-token' },
    }));
    expect(defaultProps.onSuccess).toHaveBeenCalled();
  });

  it('handles validation failure (missing required fields)', async () => {
    const user = userEvent.setup();
    render(<DNSProviderForm {...defaultProps} />);

    await user.type(screen.getByLabelText('Provider Name'), 'New Provider');

    // Type is not selected, submit button should be disabled
    const submitBtn = screen.getByRole('button', { name: 'Create' });
    expect(submitBtn).toBeDisabled();
  });

  it('tests connection', async () => {
    const user = userEvent.setup();
    render(<DNSProviderForm {...defaultProps} />);

    await user.type(screen.getByLabelText('Provider Name'), 'Test Prov');
    await user.click(screen.getByRole('combobox', { name: 'Provider Type' }));
    await user.click(screen.getByRole('option', { name: 'Cloudflare' }));
    await user.type(screen.getByLabelText('API Token'), 'token');

    mockTestCredentialsMutation.mutateAsync.mockResolvedValue({ success: true, message: 'Connection valid' });

    await user.click(screen.getByRole('button', { name: 'Test Connection' }));

    expect(mockTestCredentialsMutation.mutateAsync).toHaveBeenCalledWith(expect.objectContaining({
        provider_type: 'cloudflare',
        credentials: { api_token: 'token' }
    }));

    expect(await screen.findByText('Connection test successful')).toBeInTheDocument();
  });

  it('handles test connection failure', async () => {
    const user = userEvent.setup();
    render(<DNSProviderForm {...defaultProps} />);

    await user.type(screen.getByLabelText('Provider Name'), 'Test Prov');
    await user.click(screen.getByRole('combobox', { name: 'Provider Type' }));
    await user.click(screen.getByRole('option', { name: 'Cloudflare' }));
    await user.type(screen.getByLabelText('API Token'), 'token');

    // Simulate error response structure
    const errorResponse = {
        response: { data: { error: 'Invalid token' } }
    };
    mockTestCredentialsMutation.mutateAsync.mockRejectedValue(errorResponse);

    await user.click(screen.getByRole('button', { name: 'Test Connection' }));

    expect(await screen.findByText('Connection test failed')).toBeInTheDocument();
    expect(await screen.findByText('Invalid token')).toBeInTheDocument();
  });

  it('toggles advanced settings', async () => {
    const user = userEvent.setup();
    render(<DNSProviderForm {...defaultProps} />);

    expect(screen.queryByLabelText('Propagation Timeout (seconds)')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Advanced Settings' }));

    expect(screen.getByLabelText('Propagation Timeout (seconds)')).toBeInTheDocument();
    expect(screen.getByLabelText('Polling Interval (seconds)')).toBeInTheDocument();
    expect(screen.getByLabelText('Set as default provider')).toBeInTheDocument();
  });
});
