import { Suspense, lazy } from 'react'
import { Toaster } from 'react-hot-toast'
import { BrowserRouter as Router, Routes, Route, Outlet, Navigate  } from 'react-router-dom'

import Layout from './components/Layout'
import { LoadingOverlay } from './components/LoadingStates'
import RequireAuth from './components/RequireAuth'
import RequireRole from './components/RequireRole'
import { SetupGuard } from './components/SetupGuard'
import { ToastContainer } from './components/Toast'
import { AuthProvider } from './context/AuthContext'

// Lazy load pages for code splitting
const Dashboard = lazy(() => import('./pages/Dashboard'))
const ProxyHosts = lazy(() => import('./pages/ProxyHosts'))
const RemoteServers = lazy(() => import('./pages/RemoteServers'))
const HecateTunnels   = lazy(() => import('./pages/HecateTunnels'))
const HecateAgent     = lazy(() => import('./pages/HecateAgent'))
const HecateProviders = lazy(() => import('./pages/HecateProviders'))
const DNS = lazy(() => import('./pages/DNS'))
const ImportCaddy = lazy(() => import('./pages/ImportCaddy'))
const ImportCrowdSec = lazy(() => import('./pages/ImportCrowdSec'))
const ImportNPM = lazy(() => import('./pages/ImportNPM'))
const ImportJSON = lazy(() => import('./pages/ImportJSON'))
const Certificates = lazy(() => import('./pages/Certificates'))
const DNSProviders = lazy(() => import('./pages/DNSProviders'))
const SystemSettings = lazy(() => import('./pages/SystemSettings'))
const SMTPSettings = lazy(() => import('./pages/SMTPSettings'))
const CrowdSecConfig = lazy(() => import('./pages/CrowdSecConfig'))
const Settings = lazy(() => import('./pages/Settings'))
const Backups = lazy(() => import('./pages/Backups'))
const Tasks = lazy(() => import('./pages/Tasks'))
const Logs = lazy(() => import('./pages/Logs'))
const Domains = lazy(() => import('./pages/Domains'))
const Security = lazy(() => import('./pages/Security'))
const AccessLists = lazy(() => import('./pages/AccessLists'))
const WafConfig = lazy(() => import('./pages/WafConfig'))
const RateLimiting = lazy(() => import('./pages/RateLimiting'))
const Uptime = lazy(() => import('./pages/Uptime'))
const Notifications = lazy(() => import('./pages/Notifications'))
const UsersPage = lazy(() => import('./pages/UsersPage'))
const SecurityHeaders = lazy(() => import('./pages/SecurityHeaders'))
const AuditLogs = lazy(() => import('./pages/AuditLogs'))
const EncryptionManagement = lazy(() => import('./pages/EncryptionManagement'))
const Plugins = lazy(() => import('./pages/Plugins'))
const Login = lazy(() => import('./pages/Login'))
const Setup = lazy(() => import('./pages/Setup'))
const AcceptInvite = lazy(() => import('./pages/AcceptInvite'))
const PassthroughLanding = lazy(() => import('./pages/PassthroughLanding'))

export default function App() {
  return (
    <AuthProvider>
      <Router>
        <Suspense fallback={<LoadingOverlay message="Loading application..." />}>
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route path="/setup" element={<Setup />} />
            <Route path="/accept-invite" element={<AcceptInvite />} />
            <Route path="/passthrough" element={
              <RequireAuth>
                <PassthroughLanding />
              </RequireAuth>
            } />
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
              <Route path="domains" element={<Domains />} />

              {/* Hecate Routes */}
              <Route path="hecate">
                <Route index element={<Navigate to="/hecate/tunnels" replace />} />
                <Route path="tunnels"        element={<HecateTunnels />} />
                <Route path="remote-servers" element={<RemoteServers />} />
                <Route path="providers"      element={<HecateProviders />} />
                <Route path="agent"          element={<HecateAgent />} />
              </Route>

              {/* Legacy redirect for old Remote Servers bookmarks */}
              <Route path="remote-servers" element={<Navigate to="/hecate/remote-servers" replace />} />
              <Route path="certificates" element={<Certificates />} />

              {/* DNS Routes */}
              <Route path="dns" element={<DNS />}>
                <Route index element={<Navigate to="/dns/providers" replace />} />
                <Route path="providers" element={<DNSProviders />} />
                <Route path="plugins" element={<Plugins />} />
              </Route>

              {/* Legacy redirect for old bookmarks */}
              <Route path="dns-providers" element={<Navigate to="/dns/providers" replace />} />

              <Route path="security" element={<Security />} />
              <Route path="security/audit-logs" element={<AuditLogs />} />
              <Route path="security/access-lists" element={<AccessLists />} />
              <Route path="security/crowdsec" element={<CrowdSecConfig />} />
              <Route path="security/rate-limiting" element={<RateLimiting />} />
              <Route path="security/waf" element={<WafConfig />} />
              <Route path="security/headers" element={<SecurityHeaders />} />
              <Route path="security/encryption" element={<EncryptionManagement />} />
              <Route path="access-lists" element={<AccessLists />} />
              <Route path="uptime" element={<Uptime />} />

              {/* Legacy redirects for old user management paths */}
              <Route path="users" element={<Navigate to="/settings/users" replace />} />
              <Route path="admin/plugins" element={<Navigate to="/dns/plugins" replace />} />
              <Route path="import" element={<Navigate to="/tasks/import/caddyfile" replace />} />

              {/* Settings Routes */}
              <Route path="settings" element={<RequireRole allowed={['admin', 'user']}><Settings /></RequireRole>}>
                <Route index element={<SystemSettings />} />
                <Route path="system" element={<SystemSettings />} />
                <Route path="notifications" element={<Notifications />} />
                <Route path="smtp" element={<SMTPSettings />} />
                <Route path="crowdsec" element={<Navigate to="/security/crowdsec" replace />} />
                <Route path="users" element={<RequireRole allowed={['admin']}><UsersPage /></RequireRole>} />
                {/* Legacy redirects */}
                <Route path="account" element={<Navigate to="/settings/users" replace />} />
                <Route path="account-management" element={<Navigate to="/settings/users" replace />} />
              </Route>

              {/* Tasks Routes */}
              <Route path="tasks" element={<Tasks />}>
                <Route index element={<Backups />} />
                <Route path="backups" element={<Backups />} />
                <Route path="logs" element={<Logs />} />
                <Route path="import">
                  <Route path="caddyfile" element={<ImportCaddy />} />
                  <Route path="crowdsec" element={<ImportCrowdSec />} />
                  <Route path="npm" element={<ImportNPM />} />
                  <Route path="json" element={<ImportJSON />} />
                </Route>
              </Route>

            </Route>
          </Routes>
        </Suspense>
        <ToastContainer />
        <Toaster
          position="bottom-right"
          toastOptions={{
            duration: 5000,
            success: {
              style: { background: '#16a34a', color: 'white' },
              ariaProps: { role: 'status', 'aria-live': 'polite' },
            },
            error: {
              style: { background: '#dc2626', color: 'white' },
              ariaProps: { role: 'alert', 'aria-live': 'assertive' },
            },
          }}
        />
      </Router>
    </AuthProvider>
  )
}
