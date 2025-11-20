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
import Settings from './pages/Settings'
import Logs from './pages/Logs'
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
            <Route path="certificates" element={<Certificates />} />
            <Route path="import" element={<ImportCaddy />} />
            <Route path="logs" element={<Logs />} />
            <Route path="settings" element={<Settings />} />
          </Route>
        </Routes>
        <ToastContainer />
      </Router>
    </AuthProvider>
  )
}
