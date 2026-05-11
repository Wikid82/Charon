import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { AgentProviderAssignDialog } from '../AgentProviderAssignDialog';

const mockPatch = vi.fn();
const mockTunnels = [
  { uuid: 'cf-uuid', name: 'CF Tunnel', provider: 'cloudflare' },
  { uuid: 'ts-uuid', name: 'TS Tunnel', provider: 'tailscale' },
  { uuid: 'nb-uuid', name: 'NB Tunnel', provider: 'netbird' },
  { uuid: 'zt-uuid', name: 'ZT Tunnel', provider: 'zerotier' },
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

vi.mock('../TailscaleDevicePicker', () => ({
  TailscaleDevicePicker: ({ onSelect }: { onSelect: (id: string, addr: string) => void }) => (
    <button onClick={() => onSelect('ts-device-1', '100.72.3.4')}>TailscaleDevicePicker</button>
  ),
}));

vi.mock('../NetBirdPeerPicker', () => ({
  NetBirdPeerPicker: ({ onSelect }: { onSelect: (id: string, addr: string) => void }) => (
    <button onClick={() => onSelect('nb-peer-1', '10.0.0.1')}>NetBirdPeerPicker</button>
  ),
}));

vi.mock('../ZeroTierMemberPicker', () => ({
  ZeroTierMemberPicker: ({ onSelect }: { onSelect: (id: string, addr: string) => void }) => (
    <button onClick={() => onSelect('zt-member-1', '172.22.0.1')}>ZeroTierMemberPicker</button>
  ),
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

  it('cancel button closes without calling patchAgent', () => {
    const onClose = vi.fn();
    render(<AgentProviderAssignDialog agent={baseAgent} open onClose={onClose} />);

    fireEvent.click(screen.getByRole('button', { name: /common\.cancel/i }));

    expect(onClose).toHaveBeenCalledOnce();
    expect(mockPatch).not.toHaveBeenCalled();
  });

  it('clicking Remove Provider calls patchAgent with null fields', async () => {
    const onClose = vi.fn();
    const agentWithProvider = {
      ...baseAgent,
      hecate_tunnel_uuid: 'cf-uuid',
      device_id: 'dev-1',
      resolved_address: 'app.example.com',
    };
    render(<AgentProviderAssignDialog agent={agentWithProvider} open onClose={onClose} />);

    fireEvent.click(
      screen.getByRole('button', { name: /hecate\.agentManager\.removeProviderAssignment/i }),
    );

    expect(mockPatch).toHaveBeenCalledWith(
      {
        uuid: 'agent-1',
        req: { hecate_tunnel_uuid: null, device_id: null, resolved_address: null },
      },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });

  it('opens with tunnel pre-selected when agent has hecate_tunnel_uuid', () => {
    const agentWithProvider = { ...baseAgent, hecate_tunnel_uuid: 'ts-uuid' };
    render(<AgentProviderAssignDialog agent={agentWithProvider} open onClose={() => undefined} />);

    expect((screen.getByRole('combobox') as HTMLSelectElement).value).toBe('ts-uuid');
  });

  it('opens with resolved address pre-filled when agent has resolved_address', async () => {
    const agentWithProvider = {
      ...baseAgent,
      hecate_tunnel_uuid: 'cf-uuid',
      resolved_address: 'app.example.com',
    };
    render(<AgentProviderAssignDialog agent={agentWithProvider} open onClose={() => undefined} />);

    // Tunnel is pre-selected; just wait for the hostname input to appear
    const hostnameInput = await screen.findByRole('textbox', { name: /cloudflareTunnelHostname/i });
    expect((hostnameInput as HTMLInputElement).value).toBe('app.example.com');
  });

  it('save button is disabled when no tunnel is selected', () => {
    render(<AgentProviderAssignDialog agent={baseAgent} open onClose={() => undefined} />);

    expect(screen.getByRole('button', { name: /saveProviderAssignment/i })).toBeDisabled();
  });

  it('save button is enabled after a tunnel is selected', async () => {
    render(<AgentProviderAssignDialog agent={baseAgent} open onClose={() => undefined} />);

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'cf-uuid' } });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /saveProviderAssignment/i })).not.toBeDisabled();
    });
  });

  it('shows TailscaleDevicePicker when tailscale tunnel is selected and Select Device is clicked', async () => {
    render(<AgentProviderAssignDialog agent={baseAgent} open onClose={() => undefined} />);

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'ts-uuid' } });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /selectDevice/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /selectDevice/i }));

    await waitFor(() => {
      expect(screen.getByText('TailscaleDevicePicker')).toBeInTheDocument();
    });
  });

  it('shows ZeroTierMemberPicker when ZeroTier tunnel is selected and Select Device is clicked', async () => {
    render(<AgentProviderAssignDialog agent={baseAgent} open onClose={() => undefined} />);

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'zt-uuid' } });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /selectDevice/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /selectDevice/i }));

    await waitFor(() => {
      expect(screen.getByText('ZeroTierMemberPicker')).toBeInTheDocument();
    });
  });
});
