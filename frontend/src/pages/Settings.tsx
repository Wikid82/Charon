import { Settings as SettingsIcon, Server, Mail, Bell, Users, Palette } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link, Outlet, useLocation } from 'react-router-dom'

import { PageShell } from '../components/layout/PageShell'
import { useAuth } from '../hooks/useAuth'
import { cn } from '../utils/cn'

export default function Settings() {
  const { t } = useTranslation()
  const location = useLocation()
  const { user } = useAuth()

  const isActive = (path: string) => location.pathname === path

  const navItems = [
    { path: '/settings/system', label: t('settings.system'), icon: Server },
    { path: '/settings/notifications', label: t('navigation.notifications'), icon: Bell },
    { path: '/settings/smtp', label: t('settings.smtp'), icon: Mail },
    { path: '/settings/appearance', label: t('settings.appearance'), icon: Palette },
    ...(user?.role === 'admin' ? [{ path: '/settings/users', label: t('navigation.users'), icon: Users }] : []),
  ]

  return (
    <PageShell
      title={t('settings.title')}
      description={t('settings.description')}
      actions={
        <div className="flex items-center gap-2 text-content-muted">
          <SettingsIcon className="h-5 w-5" />
        </div>
      }
    >
      {/* Tab Navigation */}
      <nav aria-label={t('settings.title')} className="flex items-center gap-1 p-1 bg-surface-subtle rounded-lg w-fit">
        {navItems.map(({ path, label, icon: Icon }) => (
          <Link
            key={path}
            to={path}
            aria-current={isActive(path) ? 'page' : undefined}
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
