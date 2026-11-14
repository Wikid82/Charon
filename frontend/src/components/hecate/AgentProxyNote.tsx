import { useTranslation } from 'react-i18next';

import { type OrthrusAgent } from '../../api/orthrus';
import { useAgentProxyStatus } from '../../hooks/useOrthrus';
import { isLoopbackListener } from './externalProxyBind';

interface AgentProxyNoteProps {
  agent: OrthrusAgent;
}

/**
 * Passive note under an agent's PROXY badge. It describes where the live listener is reachable
 * and never asks the operator to act. Automatic mode is noted on a routable listener too, since
 * an existing setup may need an explicit network. Renders nothing otherwise.
 */
export function AgentProxyNote({ agent }: AgentProxyNoteProps) {
  const { t } = useTranslation();
  const { data: status } = useAgentProxyStatus(
    agent.uuid,
    agent.status === 'online' && agent.external_proxy_port > 0,
  );

  if (!status?.active) {
    return null;
  }

  const loopback = isLoopbackListener(status.bind_address);
  const automatic = (agent.external_proxy_bind ?? '') === '';

  if (!loopback && !automatic) {
    return null;
  }

  return (
    <div className="space-y-0.5 text-xs text-content-muted">
      {status.bind_fallback && <p>{t('hecate.externalProxy.fallbackNote')}</p>}
      {loopback && <p>{t('hecate.externalProxy.loopbackNote')}</p>}
      {!loopback && automatic && <p>{t('hecate.externalProxy.cardAutomaticNote')}</p>}
    </div>
  );
}
