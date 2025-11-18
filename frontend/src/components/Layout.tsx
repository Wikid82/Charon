import { ReactNode } from 'react'
import { Link, useLocation } from 'react-router-dom'

interface LayoutProps {
  children: ReactNode
}

export default function Layout({ children }: LayoutProps) {
  const location = useLocation()

  const navigation = [
    { name: 'Dashboard', path: '/', icon: '📊' },
    { name: 'Proxy Hosts', path: '/proxy-hosts', icon: '🌐' },
    { name: 'Remote Servers', path: '/remote-servers', icon: '🖥️' },
    { name: 'Import Caddyfile', path: '/import', icon: '📥' },
    { name: 'Settings', path: '/settings', icon: '⚙️' },
  ]

  return (
    <div className="min-h-screen bg-dark-bg flex">
      {/* Sidebar */}
      <aside className="w-60 bg-dark-sidebar border-r border-gray-800 flex flex-col">
        <div className="p-6">
          <h1 className="text-xl font-bold text-white">Caddy Proxy Manager+</h1>
        </div>
        <nav className="flex-1 px-4 space-y-1">
          {navigation.map((item) => {
            const isActive = location.pathname === item.path
            return (
              <Link
                key={item.path}
                to={item.path}
                className={`flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium transition-colors ${
                  isActive
                    ? 'bg-blue-active text-white'
                    : 'text-gray-400 hover:bg-gray-800 hover:text-white'
                }`}
              >
                <span className="text-lg">{item.icon}</span>
                {item.name}
              </Link>
            )
          })}
        </nav>
        <div className="p-4 border-t border-gray-800">
          <div className="text-xs text-gray-500">
            Version 0.1.0
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        {children}
      </main>
    </div>
  )
}
