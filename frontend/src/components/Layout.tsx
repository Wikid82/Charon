import { Link, useLocation } from 'react-router-dom';
import clsx from 'clsx';
import { ReactNode } from 'react';

const links = [
  { to: '/', label: 'Dashboard' },
  { to: '/proxy-hosts', label: 'Proxy Hosts' },
  { to: '/status', label: 'System Status' }
];

const Layout = ({ children }: { children: ReactNode }) => {
  const location = useLocation();

  return (
    <div className="app-shell">
      <aside>
        <h1>CaddyProxyManager+</h1>
        <nav>
          {links.map((link) => (
            <Link
              key={link.to}
              to={link.to}
              className={clsx({ active: location.pathname === link.to })}
            >
              {link.label}
            </Link>
          ))}
        </nav>
      </aside>
      <main>{children}</main>
    </div>
  );
};

export default Layout;
