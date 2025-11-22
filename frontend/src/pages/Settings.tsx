import { Link, Outlet, useLocation } from 'react-router-dom'

export default function Settings() {
  const location = useLocation()

  const isActive = (path: string) => location.pathname === path

  return (
    <div className="">
      <div className="mb-6">
        <h2 className="text-2xl font-semibold text-gray-900 dark:text-white">Settings</h2>
        <p className="text-sm text-gray-500 dark:text-gray-400">Manage system and account settings</p>
      </div>

      <div className="flex items-center gap-4 mb-6">
        <Link
          to="/settings/system"
          className={`px-3 py-2 rounded-md text-sm font-medium transition-colors ${
            isActive('/settings/system')
              ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
              : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
          }`}
        >
          System
        </Link>

        <Link
          to="/settings/account"
          className={`px-3 py-2 rounded-md text-sm font-medium transition-colors ${
            isActive('/settings/account')
              ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
              : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
          }`}
        >
          Account
        </Link>
      </div>

      <div className="bg-white dark:bg-dark-card border border-gray-200 dark:border-gray-800 rounded-md p-6">
        <Outlet />
      </div>
    </div>
  )
}
