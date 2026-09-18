import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';

import styles from './index.module.css';

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div className={styles.buttons}>
          <Link className="button button--secondary button--lg" to="/docs/getting-started">
            Read the Docs
          </Link>
        </div>
      </div>
    </header>
  );
}

type StartHereGroup = {
  title: string;
  links: {label: string; to: string}[];
};

const startHereGroups: StartHereGroup[] = [
  {
    title: 'Getting Started',
    links: [
      {label: 'Getting Started', to: '/docs/getting-started'},
      {label: 'What Can It Do?', to: '/docs/features'},
      {label: 'Import Your Old Setup', to: '/docs/import-guide'},
    ],
  },
  {
    title: 'Security',
    links: [
      {label: 'Security Features', to: '/docs/security'},
      {label: 'Live Logs & Notifications', to: '/docs/live-logs-guide'},
      {label: 'Testing SSL Certificates', to: '/docs/acme-staging'},
      {label: 'Migration Guide', to: '/docs/migration-guide'},
    ],
  },
  {
    title: 'API',
    links: [
      {label: 'API Reference', to: '/docs/api'},
      {label: 'Database Schema', to: '/docs/database-schema'},
    ],
  },
  {
    title: 'Remote Access',
    links: [
      {label: 'Orthrus Tunnel Agent', to: '/docs/features/orthrus'},
      {label: 'Hecate Agent Manager', to: '/docs/features/hecate'},
      {label: 'Connecting a Remote Docker Host', to: '/docs/guides/remote-docker-setup'},
    ],
  },
];

function StartHere() {
  return (
    <section className={styles.startHere}>
      <div className="container">
        <Heading as="h2" className={styles.startHereTitle}>
          Start Here
        </Heading>
        <div className="row">
          {startHereGroups.map((group) => (
            <div key={group.title} className="col col--3">
              <Heading as="h3">{group.title}</Heading>
              <ul>
                {group.links.map((link) => (
                  <li key={link.to}>
                    <Link to={link.to}>{link.label}</Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={siteConfig.title}
      description="Documentation for Charon, a self-hosted reverse proxy manager.">
      <HomepageHeader />
      <main>
        <StartHere />
      </main>
    </Layout>
  );
}
