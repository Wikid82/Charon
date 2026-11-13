import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { type ExternalProxyStatus, type OrthrusAgent } from '../../../api/orthrus';
import { AgentProxyNote } from '../AgentProxyNote';

let mockStatus: ExternalProxyStatus | undefined;
const mockUseAgentProxyStatus = vi.fn();

vi.mock('../../../hooks/useOrthrus', () => ({
  useAgentProxyStatus: (...args: unknown[]) => mockUseAgentProxyStatus(...args),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

const agent: OrthrusAgent = {
  uuid: 'agent-1',
  name: 'Agent',
  status: 'online',
  capabilities: '',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 2375,
  external_proxy_bind: '',
  write_enabled: false,
};

const status = (overrides: Partial<ExternalProxyStatus>): ExternalProxyStatus => ({
  agent_uuid: 'agent-1',
  agent_online: true,
  configured_port: 2375,
  active: true,
  active_port: 2375,
  bind_address: '172.18.0.4:2375',
  connection_string: 'tcp://charon:2375',
  error: '',
  configured_write_enabled: false,
  active_write_enabled: false,
  configured_bind: '',
  active_bind: '',
  bind_mode: 'auto',
  bind_fallback: false,
  ...overrides,
});

beforeEach(() => {
  mockUseAgentProxyStatus.mockReset();
  mockStatus = undefined;
  mockUseAgentProxyStatus.mockImplementation(() => ({ data: mockStatus }));
});

describe('AgentProxyNote', () => {
  it('queries status only for an online agent with the proxy enabled', () => {
    render(<AgentProxyNote agent={agent} />);
    expect(mockUseAgentProxyStatus).toHaveBeenCalledWith('agent-1', true);

    render(<AgentProxyNote agent={{ ...agent, status: 'offline' }} />);
    expect(mockUseAgentProxyStatus).toHaveBeenLastCalledWith('agent-1', false);
  });

  it('renders nothing while the status is unknown', () => {
    const { container } = render(<AgentProxyNote agent={agent} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('notes automatic mode on a routable listener, since an existing setup may need a network', () => {
    mockStatus = status({ bind_address: '172.18.0.4:2375' });
    render(<AgentProxyNote agent={agent} />);
    expect(screen.getByText('hecate.externalProxy.cardAutomaticNote')).toBeInTheDocument();
    expect(screen.queryByText('hecate.externalProxy.loopbackNote')).not.toBeInTheDocument();
  });

  it('renders nothing when the listen setting is explicit and the listener is routable', () => {
    mockStatus = status({ bind_address: '0.0.0.0:2375' });
    const { container } = render(<AgentProxyNote agent={{ ...agent, external_proxy_bind: '0.0.0.0' }} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('notes a loopback listener as not reachable from other containers or hosts', () => {
    mockStatus = status({ bind_address: '127.0.0.1:2375' });
    render(<AgentProxyNote agent={agent} />);
    expect(screen.getByText('hecate.externalProxy.loopbackNote')).toBeInTheDocument();
    expect(screen.queryByText('hecate.externalProxy.fallbackNote')).not.toBeInTheDocument();
  });

  it('adds the fallback line when automatic mode fell back to loopback', () => {
    mockStatus = status({ bind_address: '127.0.0.1:2375', bind_fallback: true, bind_mode: 'auto' });
    render(<AgentProxyNote agent={agent} />);
    expect(screen.getByText('hecate.externalProxy.fallbackNote')).toBeInTheDocument();
    expect(screen.getByText('hecate.externalProxy.loopbackNote')).toBeInTheDocument();
  });

  it('renders nothing when the proxy is not active', () => {
    mockStatus = status({ active: false, bind_address: '' });
    const { container } = render(<AgentProxyNote agent={agent} />);
    expect(container).toBeEmptyDOMElement();
  });
});
