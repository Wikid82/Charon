import { BrowserRouter as Router, Routes, Route, Outlet } from 'react-router-dom'
import Layout from './components/Layout'
import { ToastContainer } from './components/Toast'
import Dashboard from './pages/Dashboard'
import ProxyHosts from './pages/ProxyHosts'
import RemoteServers from './pages/RemoteServers'
import ImportCaddy from './pages/ImportCaddy'
import Settings from './pages/Settings'

export default function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<Layout><Outlet /></Layout>}>
          <Route index element={<Dashboard />} />
          <Route path="proxy-hosts" element={<ProxyHosts />} />
          <Route path="remote-servers" element={<RemoteServers />} />
          <Route path="import" element={<ImportCaddy />} />
          <Route path="settings" element={<Settings />} />
        </Route>
      </Routes>
      <ToastContainer />
    </Router>
  )
}
