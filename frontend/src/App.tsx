import { BrowserRouter as Router, Routes, Route, Outlet } from 'react-router-dom'
import Layout from './components/Layout'
import { ToastContainer } from './components/Toast'
import { SetupGuard } from './components/SetupGuard'
import RequireAuth from './components/RequireAuth'
import { AuthProvider } from './context/AuthContext'
import Dashboard from './pages/Dashboard'
import ProxyHosts from './pages/ProxyHosts'
import RemoteServers from './pages/RemoteServers'
import ImportCaddy from './pages/ImportCaddy'
import Certificates from './pages/Certificates'
import SystemSettings from './pages/SystemSettings'
import Account from './pages/Account'
import Settings from './pages/Settings'
import Backups from './pages/Backups'
import Tasks from './pages/Tasks'
import Logs from './pages/Logs'
import Domains from './pages/Domains'
import Login from './pages/Login'
import Setup from './pages/Setup'

export default function App() {
  return (
    <AuthProvider>
      <Router>
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
        <ToastContainer />
      </Router>
    </AuthProvider>
  )
}
