import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { AgentProviderAssignDialog } from '../AgentProviderAssignDialog';

const mockPatch = vi.fn();
const mockTunnels = [
  { uuid: 'cf-uuid', name: 'CF Tunnel', provider: 'cloudflare' },
  { uuid: 'ts-uuid', name: 'TS Tunnel', provider: 'tailscale' },
  { uuid: 'nb-uuid', name: 'NB Tunnel', provider: 'netbird' },
];

vi.mock('../../../hooks/useHecate', () => ({
  useHecate: () => ({
    tunnels: mockTunnels,
    isLoading: false,
  }),
}));

vi.mock('../../../hooks/useOrthrus', () => ({
  usePatchAgent: () => ({
    mutate: mockPatch,
    isPending: false,
  }),
}));

vi.mock('@tanstack/react-query', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-query')>();
  return {
    ...actual,
    useQuery: vi.fn().mockReturnValue({ data: [] }),
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, string>) => {
      if (opts?.name) return `${key}:${opts.name}`;
      return key;
    },
  }),
}));

const baseAgent = {
  uuid: 'agent-1',
  name: 'Test Agent',
  status: 'online' as const,
  capabilities: '["proxy"]',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
};

describe('AgentProviderAssignDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders dialog with tunnel dropdown', () => {
    render(
      <AgentProviderAssignDialog agent={baseAgent} open onClose={() => undefined} />,
    );

    expect(screen.getByRole('combobox')).toBeInTheDocument();
    expect(screen.getByText('CF Tunnel')).toBeInTheDocument();
    expect(screen.getByText('TS Tunnel')).toBeInTheDocument();
  });

  it('shows hostname input when cloudflare tunnel is selected', async () => {
    render(
      <AgentProviderAssignDialog agent={baseAgent} open onClose={() => undefined} />,
    );

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'cf-uuid' } });

    await waitFor(() => {
      expect(screen.getByRole('textbox', { name: /cloudflareTunnelHostname/i })).toBeInTheDocument();
    });
  });

  it('shows Select Device button when non-cloudflare tunnel is selected', async () => {
    render(
      <AgentProviderAssignDialog agent={baseAgent} open onClose={() => undefined} />,
    );

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'nb-uuid' } });

    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /selectDevice/i }),
      ).toBeInTheDocument();
    });
  });

  it('calls patchAgent with correct fields on save', async () => {
    const onClose = vi.fn();
    render(
      <AgentProviderAssignDialog agent={baseAgent} open onClose={onClose} />,
    );

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'cf-uuid' } });

    const hostnameInput = await screen.findByRole('textbox', { name: /cloudflareTunnelHostname/i });
    fireEvent.change(hostnameInput, { target: { value: 'app.example.com' } });

    fireEvent.click(screen.getByRole('button', { name: /saveProviderAssignment/i }));

    expect(mockPatch).toHaveBeenCalledWith(
      {
        uuid: 'agent-1',
        req: {
          hecate_tunnel_uuid: 'cf-uuid',
          device_id: undefined,
          resolved_address: 'app.example.com',
        },
      },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });
});
