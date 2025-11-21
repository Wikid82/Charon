import { ReactNode, useState, useEffect } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ThemeToggle } from './ThemeToggle'
import { Button } from './ui/Button'
import { useAuth } from '../hooks/useAuth'
import { checkHealth } from '../api/health'
import NotificationCenter from './NotificationCenter'
import SystemStatus from './SystemStatus'
import { Menu } from 'lucide-react'

interface LayoutProps {
  children: ReactNode
}

export default function Layout({ children }: LayoutProps) {
  const location = useLocation()
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false)
  const [isCollapsed, setIsCollapsed] = useState(() => {
    const saved = localStorage.getItem('sidebarCollapsed')
    return saved ? JSON.parse(saved) : false
  })
  const { logout, user } = useAuth()

  useEffect(() => {
    localStorage.setItem('sidebarCollapsed', JSON.stringify(isCollapsed))
  }, [isCollapsed])

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
    { name: 'Settings', path: '/settings/system', icon: '⚙️' },
  ]

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-dark-bg flex transition-colors duration-200">
      {/* Mobile Header */}
      <div className="lg:hidden fixed top-0 left-0 right-0 h-16 bg-white dark:bg-dark-sidebar border-b border-gray-200 dark:border-gray-800 flex items-center justify-between px-4 z-40">
        <h1 className="text-lg font-bold text-gray-900 dark:text-white">CPM+</h1>
        <div className="flex items-center gap-2">
          <NotificationCenter />
          <ThemeToggle />
          <Button variant="ghost" size="sm" onClick={() => setMobileSidebarOpen(!mobileSidebarOpen)}>
            {mobileSidebarOpen ? '✕' : '☰'}
          </Button>
        </div>
      </div>

      {/* Sidebar */}
      <aside className={`
        fixed lg:static inset-y-0 left-0 z-30 transform transition-all duration-200 ease-in-out
        bg-white dark:bg-dark-sidebar border-r border-gray-200 dark:border-gray-800 flex flex-col
        ${mobileSidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
        ${isCollapsed ? 'w-20' : 'w-64'}
      `}>
        <div className={`p-4 hidden lg:flex items-center justify-center`}>
           {!isCollapsed && <h1 className="text-xl font-bold text-gray-900 dark:text-white">CPM+</h1>}
           {isCollapsed && <h1 className="text-xl font-bold text-gray-900 dark:text-white">C+</h1>}
        </div>

        <div className="flex flex-col flex-1 px-4 mt-16 lg:mt-0">
          <nav className="flex-1 space-y-1">
            {navigation.map((item) => {
              const isActive = location.pathname === item.path || (item.path.startsWith('/settings') && location.pathname.startsWith(item.path) && item.path !== '/settings/system')
              return (
                <Link
                  key={item.path}
                  to={item.path}
                  onClick={() => setMobileSidebarOpen(false)}
                  className={`flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-blue-100 text-blue-700 dark:bg-blue-active dark:text-white'
                      : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-gray-900 dark:hover:text-white'
                  } ${isCollapsed ? 'justify-center' : ''}`}
                  title={isCollapsed ? item.name : ''}
                >
                  <span className="text-lg">{item.icon}</span>
                  {!isCollapsed && item.name}
                </Link>
              )
            })}
          </nav>

          <div className={`mt-2 border-t border-gray-200 dark:border-gray-800 pt-4 ${isCollapsed ? 'hidden' : ''}`}>
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
                setMobileSidebarOpen(false)
                logout()
              }}
              className="mt-3 w-full flex items-center justify-center gap-2 px-4 py-3 rounded-lg text-sm font-medium transition-colors text-red-600 dark:text-red-400 bg-red-50 hover:bg-red-100 dark:bg-red-900/20 dark:hover:bg-red-900"
            >
              <span className="text-lg">🚪</span>
              Logout
            </button>
          </div>

          {/* Collapsed Logout */}
          {isCollapsed && (
             <div className="mt-2 border-t border-gray-200 dark:border-gray-800 pt-4 pb-4">
                <button
                  onClick={() => {
                    setMobileSidebarOpen(false)
                    logout()
                  }}
                  className="w-full flex items-center justify-center p-3 rounded-lg transition-colors text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20"
                  title="Logout"
                >
                  <span className="text-lg">🚪</span>
                </button>
             </div>
          )}

        </div>
      </aside>

      {/* Overlay for mobile */}
            {/* Mobile Overlay */}
      {mobileSidebarOpen && (
        <div
          className="fixed inset-0 bg-gray-900/50 z-20 lg:hidden"
          onClick={() => setMobileSidebarOpen(false)}
        />
      )}

      {/* Main Content */}
      <main className="flex-1 min-w-0 overflow-auto pt-16 lg:pt-0 flex flex-col">
        {/* Desktop Header */}
        <header className="hidden lg:flex items-center justify-between px-8 py-4 bg-white dark:bg-dark-sidebar border-b border-gray-200 dark:border-gray-800">
           <div className="w-1/3 flex items-center gap-4">
             <button
                onClick={() => setIsCollapsed(!isCollapsed)}
                className="p-2 rounded-lg text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                title={isCollapsed ? "Expand sidebar" : "Collapse sidebar"}
              >
                <Menu className="w-5 h-5" />
              </button>
           </div>
           <div className="w-1/3 flex justify-center">
             {/* Centered content if needed, or just spacer */}
           </div>
           <div className="w-1/3 flex justify-end items-center gap-4">
             {user && <span className="text-sm font-medium text-gray-700 dark:text-gray-300">{user.name}</span>}
             <SystemStatus />
             <NotificationCenter />
             <ThemeToggle />
           </div>
        </header>
        <div className="p-4 lg:p-8 max-w-7xl mx-auto w-full">
          {children}
        </div>
      </main>
    </div>
  )
}
