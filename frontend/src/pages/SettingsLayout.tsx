import { Outlet, Link, useLocation } from 'react-router-dom'
import { Shield, Server, Archive, FileText } from 'lucide-react'

export default function SettingsLayout() {
  const location = useLocation()

  const isActive = (path: string) => location.pathname === path

  return (
    <div className="flex h-[calc(100vh-4rem)]">
      {/* Settings Sidebar */}
      <div className="w-64 bg-white dark:bg-dark-sidebar border-r border-gray-200 dark:border-gray-800 overflow-y-auto">
        <div className="p-4">
          <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-4">
            Settings
          </h2>
          <nav className="space-y-1 mb-8">
            <Link
              to="/settings/system"
              className={`flex items-center gap-2 px-3 py-2 text-sm font-medium rounded-md transition-colors ${
                isActive('/settings/system')
                  ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
            >
              <Server className="w-4 h-4" />
              System
            </Link>
            <Link
              to="/settings/account"
              className={`flex items-center gap-2 px-3 py-2 text-sm font-medium rounded-md transition-colors ${
                isActive('/settings/account')
                  ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
            >
              <Shield className="w-4 h-4" />
              Account
            </Link>
          </nav>

          <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-4">
            Tasks
          </h2>
          <nav className="space-y-1">
            <Link
              to="/settings/tasks/backups"
              className={`flex items-center gap-2 px-3 py-2 text-sm font-medium rounded-md transition-colors ${
                isActive('/settings/tasks/backups')
                  ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
            >
              <Archive className="w-4 h-4" />
              Backups
            </Link>
            <Link
              to="/settings/tasks/logs"
              className={`flex items-center gap-2 px-3 py-2 text-sm font-medium rounded-md transition-colors ${
                isActive('/settings/tasks/logs')
                  ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
            >
              <FileText className="w-4 h-4" />
              Logs
            </Link>
          </nav>
        </div>
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-y-auto p-8">
        <Outlet />
      </div>
    </div>
  )
}
