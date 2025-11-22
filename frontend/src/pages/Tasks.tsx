import { Link, Outlet, useLocation } from 'react-router-dom'

export default function Tasks() {
  const location = useLocation()

  const isActive = (path: string) => location.pathname === path

  return (
    <div className="">
      <div className="mb-6">
        <h2 className="text-2xl font-semibold text-gray-900 dark:text-white">Tasks</h2>
        <p className="text-sm text-gray-500 dark:text-gray-400">Manage system tasks and view logs</p>
      </div>

      <div className="flex items-center gap-4 mb-6">
        <Link
          to="/tasks/backups"
          className={`px-3 py-2 rounded-md text-sm font-medium transition-colors ${
            isActive('/tasks/backups')
              ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
              : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
          }`}
        >
          Backups
        </Link>

        <Link
          to="/tasks/logs"
          className={`px-3 py-2 rounded-md text-sm font-medium transition-colors ${
            isActive('/tasks/logs')
              ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300'
              : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
          }`}
        >
          Logs
        </Link>
      </div>

      <div className="bg-white dark:bg-dark-card border border-gray-200 dark:border-gray-800 rounded-md p-6">
        <Outlet />
      </div>
    </div>
  )
}
