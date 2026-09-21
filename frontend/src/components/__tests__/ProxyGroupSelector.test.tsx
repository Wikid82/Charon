import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';

import * as useProxyGroupsHook from '../../hooks/useProxyGroups';
import ProxyGroupSelector from '../ProxyGroupSelector';

import type { ProxyGroup } from '../../api/proxyGroups';

// Mock the hooks
vi.mock('../../hooks/useProxyGroups');

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

const mockGroups: ProxyGroup[] = [
  {
    uuid: 'group-uuid-1',
    name: 'Media',
    description: 'Media services',
    color: '#6366f1',
    host_count: 3,
    created_at: '2024-01-01',
    updated_at: '2024-01-01',
  },
  {
    uuid: 'group-uuid-2',
    name: 'Internal',
    description: 'Internal tools',
    color: '#22c55e',
    host_count: 1,
    created_at: '2024-01-01',
    updated_at: '2024-01-01',
  },
];

describe('ProxyGroupSelector', () => {
  it('renders the "No Group" sentinel plus one option per group', async () => {
    vi.mocked(useProxyGroupsHook.useProxyGroups).mockReturnValue({
      data: mockGroups,
    } as unknown as ReturnType<typeof useProxyGroupsHook.useProxyGroups>);

    const mockOnChange = vi.fn();
    const Wrapper = createWrapper();
    const user = userEvent.setup();

    render(
      <Wrapper>
        <ProxyGroupSelector value={null} onChange={mockOnChange} />
      </Wrapper>
    );

    const trigger = screen.getByRole('combobox', { name: /Proxy Group/i });
    expect(trigger).toBeInTheDocument();
    expect(screen.getByText('No Group')).toBeInTheDocument();

    await user.click(trigger);

    expect(screen.getByRole('option', { name: /No Group/i })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Media' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Internal' })).toBeInTheDocument();
  });

  it('calls onChange with the group uuid when a group is selected', async () => {
    vi.mocked(useProxyGroupsHook.useProxyGroups).mockReturnValue({
      data: mockGroups,
    } as unknown as ReturnType<typeof useProxyGroupsHook.useProxyGroups>);

    const mockOnChange = vi.fn();
    const Wrapper = createWrapper();
    const user = userEvent.setup();

    render(
      <Wrapper>
        <ProxyGroupSelector value={null} onChange={mockOnChange} />
      </Wrapper>
    );

    await user.click(screen.getByRole('combobox', { name: /Proxy Group/i }));
    await user.click(await screen.findByRole('option', { name: 'Media' }));

    expect(mockOnChange).toHaveBeenCalledWith('group-uuid-1');
  });

  it('calls onChange with null when the "No Group" sentinel is selected', async () => {
    vi.mocked(useProxyGroupsHook.useProxyGroups).mockReturnValue({
      data: mockGroups,
    } as unknown as ReturnType<typeof useProxyGroupsHook.useProxyGroups>);

    const mockOnChange = vi.fn();
    const Wrapper = createWrapper();
    const user = userEvent.setup();

    render(
      <Wrapper>
        <ProxyGroupSelector value="group-uuid-1" onChange={mockOnChange} />
      </Wrapper>
    );

    await user.click(screen.getByRole('combobox', { name: /Proxy Group/i }));
    await user.click(await screen.findByRole('option', { name: /No Group/i }));

    expect(mockOnChange).toHaveBeenCalledWith(null);
  });

  it('pre-selects the option matching a passed-in value', () => {
    vi.mocked(useProxyGroupsHook.useProxyGroups).mockReturnValue({
      data: mockGroups,
    } as unknown as ReturnType<typeof useProxyGroupsHook.useProxyGroups>);

    const mockOnChange = vi.fn();
    const Wrapper = createWrapper();

    render(
      <Wrapper>
        <ProxyGroupSelector value="group-uuid-2" onChange={mockOnChange} />
      </Wrapper>
    );

    expect(screen.getByRole('combobox', { name: /Proxy Group/i })).toHaveTextContent('Internal');
  });

  it('renders a sentinel-only dropdown without crashing when groups are empty/loading', async () => {
    vi.mocked(useProxyGroupsHook.useProxyGroups).mockReturnValue({
      data: undefined,
    } as unknown as ReturnType<typeof useProxyGroupsHook.useProxyGroups>);

    const mockOnChange = vi.fn();
    const Wrapper = createWrapper();
    const user = userEvent.setup();

    render(
      <Wrapper>
        <ProxyGroupSelector value={null} onChange={mockOnChange} />
      </Wrapper>
    );

    const trigger = screen.getByRole('combobox', { name: /Proxy Group/i });
    expect(trigger).toBeInTheDocument();

    await user.click(trigger);

    expect(screen.getByRole('option', { name: /No Group/i })).toBeInTheDocument();
    expect(screen.queryAllByRole('option')).toHaveLength(1);
  });
});
