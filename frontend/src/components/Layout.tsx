import { ReactNode, useState, useEffect } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ThemeToggle } from './ThemeToggle'
import { Button } from './ui/Button'
import { useAuth } from '../hooks/useAuth'
import { checkHealth } from '../api/health'
import NotificationCenter from './NotificationCenter'
import SystemStatus from './SystemStatus'
import { Menu, ChevronDown, ChevronRight } from 'lucide-react'

interface LayoutProps {
  children: ReactNode
}

type NavItem = {
  name: string
  path?: string
  icon?: string
  children?: NavItem[]
}

export default function Layout({ children }: LayoutProps) {
  const location = useLocation()
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false)
  const [isCollapsed, setIsCollapsed] = useState(() => {
    const saved = localStorage.getItem('sidebarCollapsed')
    return saved ? JSON.parse(saved) : false
  })
  const [expandedMenus, setExpandedMenus] = useState<string[]>([])
  const { logout, user } = useAuth()

  useEffect(() => {
    localStorage.setItem('sidebarCollapsed', JSON.stringify(isCollapsed))
  }, [isCollapsed])

  const toggleMenu = (name: string) => {
    setExpandedMenus(prev =>
      prev.includes(name)
        ? prev.filter(item => item !== name)
        : [...prev, name]
    )
  }

  const { data: health } = useQuery({
    queryKey: ['health'],
    queryFn: checkHealth,
    staleTime: 1000 * 60 * 60, // 1 hour
  })

  const navigation: NavItem[] = [
    { name: 'Dashboard', path: '/', icon: '📊' },
    { name: 'Proxy Hosts', path: '/proxy-hosts', icon: '🌐' },
    { name: 'Remote Servers', path: '/remote-servers', icon: '🖥️' },
    { name: 'Domains', path: '/domains', icon: '🌍' },
    { name: 'Certificates', path: '/certificates', icon: '🔒' },
    { name: 'Security', path: '/security', icon: '🛡️', children: [
      { name: 'Overview', path: '/security', icon: '🛡️' },
      { name: 'CrowdSec', path: '/security/crowdsec', icon: '🛡️' },
      { name: 'Access Lists', path: '/security/access-lists', icon: '🔒' },
      { name: 'Rate Limiting', path: '/security/rate-limiting', icon: '⚡' },
      { name: 'WAF (Coraza)', path: '/security/waf', icon: '🛡️' },
    ]},
    { name: 'Uptime', path: '/uptime', icon: '📈' },
    { name: 'Notifications', path: '/notifications', icon: '🔔' },
    // Import group moved under Tasks
    {
      name: 'Settings',
      path: '/settings',
      icon: '⚙️',
      children: [
        { name: 'System', path: '/settings/system', icon: '⚙️' },
        { name: 'Account', path: '/settings/account', icon: '🛡️' },
      ]
    },
    {
      name: 'Tasks',
      path: '/tasks',
      icon: '📋',
      children: [
        {
          name: 'Import',
          path: '/tasks/import',
          icon: '📥',
          children: [
            { name: 'Caddyfile', path: '/tasks/import/caddyfile', icon: '📥' },
            { name: 'CrowdSec', path: '/tasks/import/crowdsec', icon: '🛡️' },
          ]
        },
        { name: 'Backups', path: '/tasks/backups', icon: '💾' },
        { name: 'Logs', path: '/tasks/logs', icon: '📝' },
      ]
    },
  ]

  return (
    <div className="min-h-screen bg-light-bg dark:bg-dark-bg flex transition-colors duration-200">
      {/* Mobile Header */}
      <div className="lg:hidden fixed top-0 left-0 right-0 h-16 bg-white dark:bg-dark-sidebar border-b border-gray-200 dark:border-gray-800 flex items-center justify-between px-4 z-40">
        <img src="/banner.png" alt="Charon" height={1280} width={640} />
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
        fixed lg:fixed inset-y-0 left-0 z-30 transform transition-all duration-200 ease-in-out
        bg-white dark:bg-dark-sidebar border-r border-gray-200 dark:border-gray-800 flex flex-col
        ${mobileSidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
        ${isCollapsed ? 'w-20' : 'w-64'}
      `}>
        <div className={`h-20 flex items-center justify-center border-b border-gray-200 dark:border-gray-800`}>
           {isCollapsed ? (
                                <img src="/logo.png" alt="Charon" style={{ height: '150px', width: 'auto' }}/>


           ) : (
             <img src="/banner.png" alt="Charon" className="h-16 w-auto" />
           )}
        </div>

        <div className="flex flex-col flex-1 px-4 mt-16 lg:mt-6">
          <nav className="flex-1 space-y-1">
            {navigation.map((item) => {
              if (item.children) {
                // Collapsible Group
                const isExpanded = expandedMenus.includes(item.name)
                const isActive = location.pathname.startsWith(item.path!)

                // If sidebar is collapsed, render as a simple link (icon only)
                if (isCollapsed) {
                  return (
                    <Link
                      key={item.name}
                      to={item.path!}
                      onClick={() => setMobileSidebarOpen(false)}
                      className={`flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium transition-colors justify-center ${
                        isActive
                          ? 'bg-blue-100 text-blue-700 dark:bg-blue-active dark:text-white'
                          : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-gray-900 dark:hover:text-white'
                      }`}
                      title={item.name}
                    >
                      <span className="text-lg">{item.icon}</span>
                    </Link>
                  )
                }

                // If sidebar is expanded, render as collapsible accordion
                return (
                  <div key={item.name} className="space-y-1">
                    <button
                      onClick={() => toggleMenu(item.name)}
                      className={`w-full flex items-center justify-between px-4 py-3 rounded-lg text-sm font-medium transition-colors ${
                        isActive
                          ? 'text-blue-700 dark:text-blue-400'
                          : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-gray-900 dark:hover:text-white'
                      }`}
                    >
                      <div className="flex items-center gap-3">
                        <span className="text-lg">{item.icon}</span>
                        <span>{item.name}</span>
                      </div>
                      {isExpanded ? (
                        <ChevronDown className="w-4 h-4" />
                      ) : (
                        <ChevronRight className="w-4 h-4" />
                      )}
                    </button>

                    {isExpanded && (
                      <div className="pl-11 space-y-1">
                        {item.children.map((child: NavItem) => {
                          // If this child has its own children, render a nested accordion
                          if (child.children && child.children.length > 0) {

                            const nestedExpandedKey = `${item.name}:${child.name}`
                            const isNestedOpen = expandedMenus.includes(nestedExpandedKey)
                            return (
                              <div key={child.path} className="space-y-1">
                                <button
                                  onClick={() => toggleMenu(nestedExpandedKey)}
                                  className={`w-full flex items-center justify-between py-2 px-3 rounded-md text-sm transition-colors ${
                                    location.pathname.startsWith(child.path!)
                                      ? 'text-blue-700 dark:text-blue-400'
                                      : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 hover:text-gray-900 dark:hover:text-white'
                                  }`}
                                >
                                  <div className="flex items-center gap-2">
                                    <span className="text-lg">{child.icon}</span>
                                    <span>{child.name}</span>
                                  </div>
                                  {isNestedOpen ? (
                                    <ChevronDown className="w-4 h-4" />
                                  ) : (
                                    <ChevronRight className="w-4 h-4" />
                                  )}
                                </button>
                                {isNestedOpen && (
                                  <div className="pl-6 space-y-1">
                                    {child.children.map((sub: NavItem) => (
                                      <Link
                                        key={sub.path}
                                        to={sub.path!}
                                        onClick={() => setMobileSidebarOpen(false)}
                                        className={`block py-2 px-3 rounded-md text-sm transition-colors ${
                                          location.pathname === sub.path
                                            ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
                                            : 'text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-50 dark:hover:bg-gray-800/50'
                                        }`}
                                      >
                                        {sub.name}
                                      </Link>
                                    ))}
                                  </div>
                                )}
                              </div>
                            )
                          }
                          const isChildActive = location.pathname === child.path
                          return (
                            <Link
                              key={child.path}
                              to={child.path!}
                              onClick={() => setMobileSidebarOpen(false)}
                              className={`block py-2 px-3 rounded-md text-sm transition-colors ${
                                isChildActive
                                  ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
                                  : 'text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-50 dark:hover:bg-gray-800/50'
                              }`}
                            >
                              {child.name}
                            </Link>
                          )
                        })}
                      </div>
                    )}
                  </div>
                )
              }

              const isActive = location.pathname === item.path

              return (
                <Link
                  key={item.path}
                  to={item.path!}
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
      <main className={`flex-1 min-w-0 overflow-auto pt-16 lg:pt-0 flex flex-col transition-all duration-200 ${isCollapsed ? 'lg:ml-20' : 'lg:ml-64'}`}>
        {/* Desktop Header */}
        <header className="hidden lg:flex items-center justify-between px-8 h-20 bg-white dark:bg-dark-sidebar border-b border-gray-200 dark:border-gray-800 relative">
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
             {/* Banner moved to sidebar */}
           </div>
           <div className="w-1/3 flex justify-end items-center gap-4">
             {user && (
               <Link to="/settings/account" className="text-sm font-medium text-gray-700 dark:text-gray-300 hover:text-blue-600 dark:hover:text-blue-400 transition-colors">
                 {user.name}
               </Link>
             )}
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
