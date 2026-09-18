import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const config: Config = {
  title: 'Charon',
  tagline: 'Your server, your rules — without the headaches.',
  favicon: 'img/favicon.png',

  // Future flags, see https://docusaurus.io/docs/api/docusaurus-config#future
  future: {
    v4: true, // Improve compatibility with the upcoming Docusaurus v4
  },

  // Set the production url of your site here
  url: 'https://wikid82.github.io',
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
  baseUrl: '/Charon/',

  // GitHub pages deployment config.
  organizationName: 'Wikid82', // GitHub org/user name.
  projectName: 'Charon', // Repo name.

  // DEVIATION from docs/plans/current_spec.md §3.7 (which assumed the
  // default 'throw'): the migrated docs/ content is riddled with relative
  // links into internal-only directories that are intentionally excluded
  // from the manifest (docs/runbooks/, docs/implementation/, docs/plans/,
  // docs/security/, root SECURITY.md, README.md, etc.), plus a handful of
  // pre-existing dead links unrelated to this migration (e.g.
  // features/proxy-hosts.md, guides/certificates.md do not exist anywhere
  // in docs/ today). Those links are correct as authored for GitHub's
  // All known dangling links/anchors have been triaged and fixed (rewritten
  // as plain text, pointed at GitHub, or corrected to the real target) — see
  // the docs-writer fix pass referenced in git history. Set to 'throw' so any
  // future dangling link or anchor fails the build instead of silently
  // warning.
  onBrokenLinks: 'throw',
  onBrokenAnchors: 'throw',

  // The synced content in docs/ (see scripts/sync-docs.mjs) is plain,
  // hand-written Markdown intended for GitHub's renderer — it was never
  // authored with MDX/JSX in mind, so prose containing characters like
  // "<1s" or "</code>" trips MDX's JSX parser. `format: 'detect'` parses
  // every .md file with the traditional CommonMark pipeline (only .mdx
  // files get full MDX/JSX support), which matches the source content.
  markdown: {
    format: 'detect',
  },

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang.
  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          path: 'docs',
          sidebarPath: './sidebars.ts',
          routeBasePath: 'docs',
          // No "edit this page" link — docs are generated from ../docs/
          // (source of truth) by scripts/sync-docs.mjs, not hand-edited here.
          editUrl: undefined,
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themes: [
    // Offline, no-external-service search index — consistent with Charon's
    // "no external dependencies" ethos (see docs/plans/current_spec.md §1.3,
    // §3.1). Indexes whatever scripts/sync-docs.mjs populates docs/ with.
    [
      '@easyops-cn/docusaurus-search-local',
      {
        hashed: true,
        indexDocs: true,
        indexBlog: false,
        indexPages: true,
        docsRouteBasePath: '/docs',
      },
    ],
  ],

  themeConfig: {
    // Replace with Charon's own social card once one is designed;
    // reuses the existing repo banner in the meantime.
    image: 'img/banner.webp',
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'Charon',
      logo: {
        alt: 'Charon Logo',
        src: 'img/favicon.png',
      },
      items: [
        {
          type: 'doc',
          docId: 'getting-started',
          position: 'left',
          label: 'Docs',
        },
        {
          href: 'https://github.com/Wikid82/Charon',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {
              label: 'Getting Started',
              to: '/docs/getting-started',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/Wikid82/Charon',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Charon. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
