import { Loader2 } from 'lucide-react'
import { useEffect, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useQueryClient } from '@tanstack/react-query'

import { Card, CardContent } from './ui/Card'
import { MAINTENANCE_STATUS_QUERY_KEY, useMaintenanceStatus } from '../hooks/useDatabaseMaintenance'
import { leaveMaintenance, useMaintenanceMode } from '../utils/maintenanceMode'

/**
 * Full-page view shown while the server optimizes its database. Polls the
 * unauthenticated status route and hands control back when it is no longer active.
 */
export function MaintenanceView() {
  const { t } = useTranslation()
  const { data } = useMaintenanceStatus(true)

  useEffect(() => {
    if (data && !data.active) leaveMaintenance()
  }, [data])

  return (
    <div className="min-h-screen flex items-center justify-center bg-surface-base p-4">
      <Card className="max-w-md w-full">
        <CardContent className="p-8 text-center space-y-4" role="status" aria-live="polite">
          <Loader2 className="h-10 w-10 mx-auto animate-spin text-brand-500" aria-hidden="true" />
          <h1 className="text-xl font-semibold text-content-primary">
            {t('systemSettings.maintenance.title')}
          </h1>
          <p className="text-sm text-content-secondary">{t('systemSettings.maintenance.body')}</p>
          {data?.active && (
            <p className="text-xs text-content-muted">
              {t('systemSettings.maintenance.elapsed', { seconds: data.elapsed_seconds })}
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

/**
 * Replaces the whole app with {@link MaintenanceView} while maintenance mode is
 * on and remounts it afterwards, so the session check re-runs against a healthy
 * server. In-flight queries are cancelled on entry so nothing retries against
 * the 503 while the view is up.
 */
export default function MaintenanceGate({ children }: { children: ReactNode }) {
  const maintenance = useMaintenanceMode()
  const queryClient = useQueryClient()

  useEffect(() => {
    if (!maintenance) return
    void queryClient.cancelQueries({
      predicate: (query) => query.queryKey[0] !== MAINTENANCE_STATUS_QUERY_KEY[0],
    })
  }, [maintenance, queryClient])

  return maintenance ? <MaintenanceView /> : <>{children}</>
}
