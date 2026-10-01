import { useTranslation } from 'react-i18next'
import { Link, Outlet, useLocation } from 'react-router'

import { useAuth } from '../hooks/useAuth'

export default function Tasks() {
  const { t } = useTranslation()
  const location = useLocation()
  const { user } = useAuth()

  const isActive = (path: string) => location.pathname === path

  const tabs = [
    { path: '/tasks/backups', label: t('navigation.backups') },
    { path: '/tasks/logs', label: t('navigation.logs') },
    // The Database endpoints are admin-only, so other roles would only get a redirect.
    ...(user?.role === 'admin' ? [{ path: '/tasks/database', label: t('navigation.database') }] : []),
  ]

  return (
    <div className="">
      <div className="mb-6">
        <h2 className="text-2xl font-semibold text-gray-900 dark:text-white">{t('tasks.title')}</h2>
        <p className="text-sm text-gray-500 dark:text-gray-400">{t('tasks.description')}</p>
      </div>

      <div className="flex items-center gap-4 mb-6">
        {tabs.map((tab) => (
          <Link
            key={tab.path}
            to={tab.path}
            className={`px-3 py-2 rounded-md text-sm font-medium transition-colors ${
              isActive(tab.path)
                ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
                : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
            }`}
          >
            {tab.label}
          </Link>
        ))}
      </div>

      <div className="bg-white dark:bg-dark-card border border-gray-200 dark:border-gray-800 rounded-md p-6">
        <Outlet />
      </div>
    </div>
  )
}
