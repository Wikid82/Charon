import { Database } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  DatabaseNotice,
  DatabaseStatusList,
  LastOptimization,
  ReclaimControl,
} from '../components/DatabaseMaintenance'
import { Alert, Button } from '../components/ui'
import { useDatabaseStatus } from '../hooks/useDatabaseMaintenance'

const AUTOMATIC_KEYS = [
  'databaseMaintenance.automatic.start',
  'databaseMaintenance.automatic.convert',
  'databaseMaintenance.automatic.drain',
] as const

/**
 * Tasks -> Database (admin only). Passive information about the database and the
 * automatic maintenance, plus an optional manual reclaim trigger. Never empty:
 * the explainer is shown in every state, including loading and load errors.
 */
export default function DatabaseMaintenancePage() {
  const { t } = useTranslation()
  const { data: db, isLoading, isError, refetch, isFetching } = useDatabaseStatus()

  return (
    <section aria-labelledby="database-page-title" className="space-y-6">
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <Database className="h-5 w-5 text-content-muted" aria-hidden="true" />
          <h3 id="database-page-title" className="text-lg font-semibold text-content-primary">
            {t('databaseMaintenance.title')}
          </h3>
        </div>
        <p className="text-sm text-content-secondary">{t('databaseMaintenance.description')}</p>
        <ul className="list-disc space-y-1 pl-5 text-sm text-content-secondary">
          {AUTOMATIC_KEYS.map((key) => (
            <li key={key}>{t(key)}</li>
          ))}
        </ul>
      </div>

      {isLoading && <p className="text-sm text-content-muted">{t('databaseMaintenance.loading')}</p>}

      {isError && (
        <Alert variant="error" title={t('databaseMaintenance.loadError')} aria-label={t('databaseMaintenance.loadErrorLabel')}>
          <Button variant="secondary" size="sm" onClick={() => refetch()} isLoading={isFetching}>
            {t('databaseMaintenance.retry')}
          </Button>
        </Alert>
      )}

      {db && (
        <>
          <DatabaseStatusList db={db} />
          <LastOptimization db={db} />
          <DatabaseNotice notice={db.notice} />
          <ReclaimControl db={db} />
        </>
      )}
    </section>
  )
}
