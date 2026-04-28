import { AlertCircle, CheckCircle2, Circle, Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { type TunnelState } from '../../api/hecate';
import { cn } from '../../utils/cn';
import { Badge } from '../ui/Badge';

interface TunnelStatusBadgeProps {
  state: TunnelState;
  className?: string;
  showLabel?: boolean;
}

const stateConfig: Record<
  TunnelState,
  {
    variant: 'success' | 'warning' | 'error' | 'secondary';
    icon: React.ReactNode;
    i18nKey: string;
  }
> = {
  connected: {
    variant: 'success',
    icon: <CheckCircle2 className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />,
    i18nKey: 'hecate.stateConnected',
  },
  connecting: {
    variant: 'warning',
    icon: <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" aria-hidden="true" />,
    i18nKey: 'hecate.stateConnecting',
  },
  error: {
    variant: 'error',
    icon: <AlertCircle className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />,
    i18nKey: 'hecate.stateError',
  },
  stopped: {
    variant: 'secondary',
    icon: <Circle className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />,
    i18nKey: 'hecate.stateStopped',
  },
};

export const TunnelStatusBadge = ({ state, className, showLabel = true }: TunnelStatusBadgeProps) => {
  const { t } = useTranslation();
  const config = stateConfig[state];
  const label = t(config.i18nKey);

  return (
    <Badge
      variant={config.variant}
      size="sm"
      role="status"
      aria-label={t('hecate.tunnelStatus', { state: label })}
      className={cn('gap-1', className)}
    >
      {config.icon}
      {showLabel && <span>{label}</span>}
    </Badge>
  );
};
