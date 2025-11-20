import { Outlet, Link, useLocation } from 'react-router-dom'
import { Shield, Archive, FileText, ChevronDown, ChevronRight, Server } from 'lucide-react'
import { useState } from 'react'

export default function SettingsLayout() {
  const location = useLocation()
  const [tasksOpen, setTasksOpen] = useState(true)

  const isActive = (path: string) => location.pathname === path

  return (
    <div className="flex h-[calc(100vh-4rem)]">
      {/* Settings Sidebar */}
      <div className="w-64 bg-white dark:bg-dark-sidebar border-r border-gray-200 dark:border-gray-800 overflow-y-auto">
        <div className="p-4">
          <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-4">
            Settings
          </h2>
          <nav className="space-y-1">
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
              to="/settings/security"
              className={`flex items-center gap-2 px-3 py-2 text-sm font-medium rounded-md transition-colors ${
                isActive('/settings/security')
                  ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
            >
              <Shield className="w-4 h-4" />
              Security
            </Link>

            {/* Tasks Group */}
            <div>
              <button
                onClick={() => setTasksOpen(!tasksOpen)}
                className="w-full flex items-center justify-between px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-md transition-colors"
              >
                <div className="flex items-center gap-2">
                  <span className="text-lg">📋</span>
                  Tasks
                </div>
                {tasksOpen ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
              </button>

              {tasksOpen && (
                <div className="ml-4 mt-1 space-y-1 border-l-2 border-gray-100 dark:border-gray-800 pl-2">
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
                </div>
              )}
            </div>
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
