import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import ProxyHostForm from '../ProxyHostForm';

import type { ProxyHost } from '../../api/proxyHosts';

vi.mock('../../hooks/useRemoteServers', () => ({
  useRemoteServers: vi.fn(() => ({
    servers: [],
    isLoading: false,
    error: null,
  })),
}));

vi.mock('../../hooks/useDocker', () => ({
  useDocker: vi.fn(() => ({
    containers: [],
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  })),
}));

vi.mock('../../hooks/useDomains', () => ({
  useDomains: vi.fn(() => ({
    domains: [{ uuid: 'domain-1', name: 'test.com' }],
    createDomain: vi.fn().mockResolvedValue({}),
    isLoading: false,
    error: null,
  })),
}));

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(() => ({
    certificates: [],
    isLoading: false,
    error: null,
  })),
}));

vi.mock('../../hooks/useDNSDetection', () => ({
  useDetectDNSProvider: vi.fn(() => ({
    mutateAsync: vi.fn(),
    isPending: false,
    data: undefined,
    reset: vi.fn(),
  })),
}));

vi.mock('../../hooks/useAccessLists', () => ({
  useAccessLists: vi.fn(() => ({
    data: [
      {
        id: 1,
        uuid: 'acl-uuid-1',
        name: 'Office Network',
        description: 'Office IP range',
        type: 'whitelist',
        enabled: true,
      },
    ],
    isLoading: false,
    error: null,
  })),
}));

vi.mock('../../hooks/useSecurityHeaders', () => ({
  useSecurityHeaderProfiles: vi.fn(() => ({
    data: [
      {
        id: 1,
        uuid: 'profile-uuid-1',
        name: 'Basic Security',
        description: 'Basic security headers',
        is_preset: true,
        preset_type: 'basic',
        security_score: 60,
      },
      {
        id: undefined,
        uuid: undefined,
        name: 'Malformed Custom',
        description: 'Should be skipped in options map',
        is_preset: false,
        preset_type: 'custom',
        security_score: 10,
      },
    ],
    isLoading: false,
    error: null,
  })),
}));

vi.mock('../ui/Select', () => {
  const findText = (children: React.ReactNode): string => {
    if (typeof children === 'string') {
      return children;
    }

    if (Array.isArray(children)) {
      return children.map((child) => findText(child)).join(' ');
    }

    if (children && typeof children === 'object' && 'props' in children) {
      const node = children as { props?: { children?: React.ReactNode } };
      return findText(node.props?.children);
    }

    return '';
  };

  const Select = ({ value, onValueChange, children }: { value?: string; onValueChange?: (value: string) => void; children?: React.ReactNode }) => {
    const text = findText(children);
    const isSecurityHeaders = text.includes('None (No Security Headers)');

    return (
      <div>
        {isSecurityHeaders && (
          <>
            <div data-testid="security-select-value">{value}</div>
            <button type="button" onClick={() => onValueChange?.('42')}>emit-security-plain-numeric</button>
            <button type="button" onClick={() => onValueChange?.('custom-header-token')}>emit-security-custom</button>
          </>
        )}
        {children}
      </div>
    );
  };

  const SelectTrigger = ({ children, ...rest }: React.ComponentProps<'button'>) => <button type="button" {...rest}>{children}</button>;
  const SelectContent = ({ children }: { children?: React.ReactNode }) => <div>{children}</div>;
  const SelectItem = ({ children }: { value: string; children?: React.ReactNode }) => <div>{children}</div>;
  const SelectValue = () => <span />;

  return {
    Select,
    SelectTrigger,
    SelectContent,
    SelectItem,
    SelectValue,
  };
});

vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({ json: () => Promise.resolve({ internal_ip: '127.0.0.1' }) })));

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

const fillRequiredFields = async () => {
  await userEvent.type(screen.getByLabelText(/^Name/), 'Coverage Host');
  await userEvent.type(screen.getByLabelText(/Domain Names/), 'test.com');
  await userEvent.type(screen.getByLabelText(/^Host$/), 'localhost');
  await userEvent.clear(screen.getByLabelText(/^Port$/));
  await userEvent.type(screen.getByLabelText(/^Port$/), '8080');
};

describe('ProxyHostForm token coverage branches', () => {
  const onCancel = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('normalizes prefixed and numeric-string security header IDs', async () => {
    const onSubmit = vi.fn<(data: Partial<ProxyHost>) => Promise<void>>().mockResolvedValue();
    const Wrapper = createWrapper();

    const { rerender } = render(
      <Wrapper>
        <ProxyHostForm
          host={{
            uuid: 'host-1',
            domain_names: 'a.test',
            forward_host: 'localhost',
            forward_port: 8080,
            security_header_profile_id: 'id:7',
          } as ProxyHost}
          onSubmit={onSubmit}
          onCancel={onCancel}
        />
      </Wrapper>
    );

    expect(screen.getByTestId('security-select-value')).toHaveTextContent('id:7');

    rerender(
      <Wrapper>
        <ProxyHostForm
          host={{
            uuid: 'host-2',
            domain_names: 'b.test',
            forward_host: 'localhost',
            forward_port: 8080,
            security_header_profile_id: '12',
          } as ProxyHost}
          onSubmit={onSubmit}
          onCancel={onCancel}
        />
      </Wrapper>
    );

    expect(screen.getByTestId('security-select-value')).toHaveTextContent('id:12');
  });

  it('converts plain numeric and custom security tokens on submit', async () => {
    const onSubmit = vi.fn<(data: Partial<ProxyHost>) => Promise<void>>().mockResolvedValue();
    const Wrapper = createWrapper();

    render(
      <Wrapper>
        <ProxyHostForm onSubmit={onSubmit} onCancel={onCancel} />
      </Wrapper>
    );

    await fillRequiredFields();

    await userEvent.click(screen.getByRole('button', { name: 'emit-security-plain-numeric' }));
    await userEvent.click(screen.getByRole('button', { name: /Save/i }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ security_header_profile_id: 42 })
      );
    });

    onSubmit.mockClear();

    await userEvent.click(screen.getByRole('button', { name: 'emit-security-custom' }));
    await userEvent.click(screen.getByRole('button', { name: /Save/i }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ security_header_profile_id: 'custom-header-token' })
      );
    });
  });
});
