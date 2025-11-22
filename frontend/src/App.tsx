import { Suspense, lazy } from 'react'
import { BrowserRouter as Router, Routes, Route, Outlet } from 'react-router-dom'
import Layout from './components/Layout'
import { ToastContainer } from './components/Toast'
import { SetupGuard } from './components/SetupGuard'
import { LoadingOverlay } from './components/LoadingStates'
import RequireAuth from './components/RequireAuth'
import { AuthProvider } from './context/AuthContext'

// Lazy load pages for code splitting
const Dashboard = lazy(() => import('./pages/Dashboard'))
const ProxyHosts = lazy(() => import('./pages/ProxyHosts'))
const RemoteServers = lazy(() => import('./pages/RemoteServers'))
const ImportCaddy = lazy(() => import('./pages/ImportCaddy'))
const Certificates = lazy(() => import('./pages/Certificates'))
const SystemSettings = lazy(() => import('./pages/SystemSettings'))
const Account = lazy(() => import('./pages/Account'))
const Settings = lazy(() => import('./pages/Settings'))
const Backups = lazy(() => import('./pages/Backups'))
const Tasks = lazy(() => import('./pages/Tasks'))
const Logs = lazy(() => import('./pages/Logs'))
const Domains = lazy(() => import('./pages/Domains'))
const Login = lazy(() => import('./pages/Login'))
const Setup = lazy(() => import('./pages/Setup'))

export default function App() {
  return (
    <AuthProvider>
      <Router>
        <Suspense fallback={<LoadingOverlay message="Loading application..." />}>
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route path="/setup" element={<Setup />} />
            <Route path="/" element={
              <SetupGuard>
                <RequireAuth>
                  <Layout>
                    <Outlet />
                  </Layout>
                </RequireAuth>
              </SetupGuard>
            }>
              <Route index element={<Dashboard />} />
              <Route path="proxy-hosts" element={<ProxyHosts />} />
              <Route path="remote-servers" element={<RemoteServers />} />
              <Route path="domains" element={<Domains />} />
              <Route path="certificates" element={<Certificates />} />
              <Route path="import" element={<ImportCaddy />} />

              {/* Settings Routes */}
              <Route path="settings" element={<Settings />}>
                <Route index element={<SystemSettings />} />
                <Route path="system" element={<SystemSettings />} />
                <Route path="account" element={<Account />} />
              </Route>

              {/* Tasks Routes */}
              <Route path="tasks" element={<Tasks />}>
                <Route index element={<Backups />} />
                <Route path="backups" element={<Backups />} />
                <Route path="logs" element={<Logs />} />
              </Route>

            </Route>
          </Routes>
        </Suspense>
        <ToastContainer />
      </Router>
    </AuthProvider>
  )
}
