import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import AccessListSelector from '../AccessListSelector';
import * as useAccessListsHook from '../../hooks/useAccessLists';
import type { AccessList } from '../../api/accessLists';

// Mock the hooks
vi.mock('../../hooks/useAccessLists');

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

describe('AccessListSelector', () => {
  it('should render with no access lists', () => {
    vi.mocked(useAccessListsHook.useAccessLists).mockReturnValue({
      data: [],
    } as unknown as ReturnType<typeof useAccessListsHook.useAccessLists>);

    const mockOnChange = vi.fn();
    const Wrapper = createWrapper();

    render(
      <Wrapper>
        <AccessListSelector value={null} onChange={mockOnChange} />
      </Wrapper>
    );

    expect(screen.getByRole('combobox')).toBeInTheDocument();
    expect(screen.getByText('No Access Control (Public)')).toBeInTheDocument();
  });

  it('should render with access lists and show only enabled ones', () => {
    const mockLists: AccessList[] = [
      {
        id: 1,
        uuid: 'uuid-1',
        name: 'Test ACL 1',
        description: 'Description 1',
        type: 'whitelist',
        ip_rules: '[]',
        country_codes: '',
        local_network_only: false,
        enabled: true,
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
      },
      {
        id: 2,
        uuid: 'uuid-2',
        name: 'Test ACL 2',
        description: 'Description 2',
        type: 'blacklist',
        ip_rules: '[]',
        country_codes: '',
        local_network_only: false,
        enabled: false,
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
      },
    ];

    vi.mocked(useAccessListsHook.useAccessLists).mockReturnValue({
      data: mockLists,
    } as unknown as ReturnType<typeof useAccessListsHook.useAccessLists>);

    const mockOnChange = vi.fn();
    const Wrapper = createWrapper();

    render(
      <Wrapper>
        <AccessListSelector value={null} onChange={mockOnChange} />
      </Wrapper>
    );

    expect(screen.getByRole('combobox')).toBeInTheDocument();
    expect(screen.getByText('Test ACL 1 (whitelist)')).toBeInTheDocument();
    expect(screen.queryByText('Test ACL 2 (blacklist)')).not.toBeInTheDocument();
  });

  it('should show selected ACL details', () => {
    const mockLists: AccessList[] = [
      {
        id: 1,
        uuid: 'uuid-1',
        name: 'Selected ACL',
        description: 'This is selected',
        type: 'geo_whitelist',
        ip_rules: '[]',
        country_codes: 'US,CA',
        local_network_only: false,
        enabled: true,
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
      },
    ];

    vi.mocked(useAccessListsHook.useAccessLists).mockReturnValue({
      data: mockLists,
    } as unknown as ReturnType<typeof useAccessListsHook.useAccessLists>);

    const mockOnChange = vi.fn();
    const Wrapper = createWrapper();

    render(
      <Wrapper>
        <AccessListSelector value={1} onChange={mockOnChange} />
      </Wrapper>
    );

    expect(screen.getByText('Selected ACL')).toBeInTheDocument();
    expect(screen.getByText('This is selected')).toBeInTheDocument();
    expect(screen.getByText(/Countries: US,CA/)).toBeInTheDocument();
  });
});
