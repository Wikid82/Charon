import { ReactNode, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ThemeToggle } from './ThemeToggle'
import { Button } from './ui/Button'
import { useAuth } from '../context/AuthContext'
import { checkHealth } from '../api/health'
import NotificationCenter from './NotificationCenter'
import SystemStatus from './SystemStatus'

interface LayoutProps {
  children: ReactNode
}

export default function Layout({ children }: LayoutProps) {
  const location = useLocation()
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const { logout } = useAuth()

  const { data: health } = useQuery({
    queryKey: ['health'],
    queryFn: checkHealth,
    staleTime: 1000 * 60 * 60, // 1 hour
  })

  const navigation = [
    { name: 'Dashboard', path: '/', icon: '📊' },
    { name: 'Proxy Hosts', path: '/proxy-hosts', icon: '🌐' },
    { name: 'Remote Servers', path: '/remote-servers', icon: '🖥️' },
    { name: 'Certificates', path: '/certificates', icon: '🔒' },
    { name: 'Import Caddyfile', path: '/import', icon: '📥' },
    { name: 'Settings', path: '/settings/security', icon: '⚙️' },
  ]

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-dark-bg flex transition-colors duration-200">
      {/* Mobile Header */}
      <div className="lg:hidden fixed top-0 left-0 right-0 h-16 bg-white dark:bg-dark-sidebar border-b border-gray-200 dark:border-gray-800 flex items-center justify-between px-4 z-40">
        <h1 className="text-lg font-bold text-gray-900 dark:text-white">CPM+</h1>
        <div className="flex items-center gap-2">
          <NotificationCenter />
          <ThemeToggle />
          <Button variant="ghost" size="sm" onClick={() => setSidebarOpen(!sidebarOpen)}>
            {sidebarOpen ? '✕' : '☰'}
          </Button>
        </div>
      </div>

      {/* Sidebar */}
      <aside className={`
        fixed lg:static inset-y-0 left-0 z-30 w-64 transform transition-transform duration-200 ease-in-out
        bg-white dark:bg-dark-sidebar border-r border-gray-200 dark:border-gray-800 flex flex-col
        ${sidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
      `}>
        <div className="p-6 hidden lg:flex items-center justify-between">
          <h1 className="text-xl font-bold text-gray-900 dark:text-white">CPM+</h1>
          <ThemeToggle />
        </div>

        <div className="flex flex-col flex-1 px-4 mt-16 lg:mt-0">
          <nav className="flex-1 space-y-1">
            {navigation.map((item) => {
              const isActive = location.pathname === item.path || (item.path.startsWith('/settings') && location.pathname.startsWith('/settings'))
              return (
                <Link
                  key={item.path}
                  to={item.path}
                  onClick={() => setSidebarOpen(false)}
                  className={`flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-blue-100 text-blue-700 dark:bg-blue-active dark:text-white'
                      : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-gray-900 dark:hover:text-white'
                  }`}
                >
                  <span className="text-lg">{item.icon}</span>
                  {item.name}
                </Link>
              )
            })}
          </nav>
          <div className="mt-6 border-t border-gray-200 dark:border-gray-800 pt-4">
            <div className="text-xs text-gray-500 dark:text-gray-500 text-center mb-2 flex flex-col gap-0.5">
              <span>Version {health?.version || 'dev'}</span>
              {health?.git_commit && health.git_commit !== 'unknown' && (
                <span className="text-[10px] opacity-75 font-mono">
                  ({health.git_commit.substring(0, 7)})
                </span>
              )}
            </div>
            <button
              onClick={() => {
                setSidebarOpen(false)
                logout()
              }}
              className="mt-3 w-full flex items-center justify-center gap-2 px-4 py-3 rounded-lg text-sm font-medium transition-colors text-red-600 dark:text-red-400 bg-red-50 hover:bg-red-100 dark:bg-red-900/20 dark:hover:bg-red-900"
            >
              <span className="text-lg">🚪</span>
              Logout
            </button>
          </div>
        </div>
      </aside>

      {/* Overlay for mobile */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-20 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Main Content */}
      <main className="flex-1 min-w-0 overflow-auto pt-16 lg:pt-0 flex flex-col">
        {/* Desktop Header */}
        <header className="hidden lg:flex items-center justify-end px-8 py-4 gap-4 bg-white dark:bg-dark-sidebar border-b border-gray-200 dark:border-gray-800">
           <SystemStatus />
           <NotificationCenter />
        </header>
        <div className="p-4 lg:p-8 max-w-7xl mx-auto w-full">
          {children}
        </div>
      </main>
    </div>
  )
}
