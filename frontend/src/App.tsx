import { Route, Routes } from 'react-router-dom';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import ProxyHosts from './pages/ProxyHosts';
import HealthStatus from './pages/HealthStatus';

const App = () => {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/proxy-hosts" element={<ProxyHosts />} />
        <Route path="/status" element={<HealthStatus />} />
      </Routes>
    </Layout>
  );
};

export default App;
