import { Cloud, Puzzle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link, Outlet, useLocation } from 'react-router-dom'

import { PageShell } from '../components/layout/PageShell'
import { cn } from '../utils/cn'


export default function DNS() {
  const { t } = useTranslation()
  const location = useLocation()

  const isActive = (path: string) => location.pathname === path

  const navItems = [
    { path: '/dns/providers', label: t('navigation.dnsProviders'), icon: Cloud },
    { path: '/dns/plugins', label: t('navigation.plugins'), icon: Puzzle },
  ]

  return (
    <PageShell
      title={t('dns.title')}
      description={t('dns.description')}
      actions={
        <div className="flex items-center gap-2 text-content-muted">
          <Cloud className="h-5 w-5" />
        </div>
      }
    >
      {/* Tab Navigation */}
      <nav className="flex items-center gap-1 p-1 bg-surface-subtle rounded-lg w-fit">
        {navItems.map(({ path, label, icon: Icon }) => (
          <Link
            key={path}
            to={path}
            className={cn(
              'flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-all duration-fast',
              isActive(path)
                ? 'bg-surface-elevated text-content-primary shadow-sm'
                : 'text-content-secondary hover:text-content-primary hover:bg-surface-muted'
            )}
          >
            <Icon className="h-4 w-4" />
            {label}
          </Link>
        ))}
      </nav>

      {/* Content Area */}
      <div className="bg-surface-elevated border border-border rounded-lg p-6">
        <Outlet />
      </div>
    </PageShell>
  )
}
