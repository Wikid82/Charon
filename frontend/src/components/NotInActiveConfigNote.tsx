import { useTranslation } from 'react-i18next'

import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from './ui/Tooltip'

/**
 * Passive note under a proxy host that the last successful apply left out of the configuration. The hint explains
 * the cause on hover or focus. The note is text only: no badge, banner or action.
 */
export function NotInActiveConfigNote() {
  const { t } = useTranslation()

  return (
    <TooltipProvider delayDuration={300}>
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            data-testid="proxy-host-not-in-active-config"
            className="text-xs text-content-muted cursor-help rounded-sm text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400"
          >
            {t('proxyHosts.notInActiveConfig')}
          </button>
        </TooltipTrigger>
        <TooltipContent>{t('proxyHosts.notInActiveConfigHint')}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
