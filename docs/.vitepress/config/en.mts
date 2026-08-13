import { defineConfig, type DefaultTheme } from 'vitepress'
import { REPO } from './shared.mts'

/** English is the root locale, so its pages live at the top of docs/ with no path prefix. */
export const en = defineConfig({
  label: 'English',
  lang: 'en-US',
  description:
    'A self-hosted, real-time web analytics platform — Go, ClickHouse, Kafka and Next.js.',

  themeConfig: {
    nav: nav(),
    sidebar: {
      '/guide/': { base: '/guide/', items: guideSidebar() },
      '/knowledge/': { base: '/knowledge/', items: knowledgeSidebar() },
      '/reference/': { base: '/reference/', items: referenceSidebar() },
      '/notes/': { base: '/notes/', items: notesSidebar() },
      '/adr/': { base: '/adr/', items: adrSidebar() },
    },

    editLink: {
      pattern: `https://github.com/${REPO}/edit/main/docs/:path`,
      text: 'Edit this page on GitHub',
    },

    docFooter: { prev: 'Previous', next: 'Next' },
    outline: { level: [2, 3], label: 'On this page' },
    lastUpdated: { text: 'Last updated' },
    returnToTopLabel: 'Back to top',
    darkModeSwitchLabel: 'Appearance',
    sidebarMenuLabel: 'Menu',
    langMenuLabel: 'Change language',

    footer: {
      message: 'Released under the MIT License.',
      copyright: `Copyright © 2026 <a href="https://github.com/${REPO.split('/')[0]}">nxhawk</a>`,
    },
  },
})

function nav(): DefaultTheme.NavItem[] {
  return [
    { text: 'Guide', link: '/guide/introduction', activeMatch: '/guide/' },
    { text: 'Knowledge', link: '/knowledge/', activeMatch: '/knowledge/' },
    { text: 'Reference', link: '/reference/api', activeMatch: '/reference/' },
    { text: 'Notes', link: '/notes/', activeMatch: '/notes/' },
    { text: 'ADR', link: '/adr/', activeMatch: '/adr/' },
    { text: 'Roadmap', link: '/roadmap' },
    {
      text: 'Planning docs',
      items: [
        { text: 'PLAN.md — specification', link: `https://github.com/${REPO}/blob/main/PLAN.md` },
        { text: 'PHASES.md — delivery phases', link: `https://github.com/${REPO}/blob/main/PHASES.md` },
        { text: 'TODO.md — checklist', link: `https://github.com/${REPO}/blob/main/TODO.md` },
        { text: 'DEPLOY-AWS.md — infrastructure', link: `https://github.com/${REPO}/blob/main/DEPLOY-AWS.md` },
      ],
    },
  ]
}

function guideSidebar(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Getting started',
      collapsed: false,
      items: [
        { text: 'Introduction', link: 'introduction' },
        { text: 'Quick start', link: 'quick-start' },
        { text: 'Configuration', link: 'configuration' },
      ],
    },
    {
      text: 'Understanding the system',
      collapsed: false,
      items: [
        { text: 'Architecture', link: 'architecture' },
        { text: 'Project structure', link: 'project-structure' },
      ],
    },
    {
      text: 'Working on it',
      collapsed: false,
      items: [
        { text: 'Contributing', link: 'contributing' },
        { text: 'Deploy configuration', link: 'deploy-config' },
        { text: 'Deployment', link: 'deployment' },
      ],
    },
  ]
}

function knowledgeSidebar(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Knowledge base',
      items: [
        { text: 'Overview', link: '/knowledge/' },
        { text: 'ClickHouse explained', link: 'clickhouse' },
      ],
    },
  ]
}

function referenceSidebar(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Reference',
      items: [
        { text: 'HTTP API', link: 'api' },
        { text: 'Event schema', link: 'event-schema' },
        { text: 'ClickHouse schema', link: 'clickhouse' },
        { text: 'Observability', link: 'observability' },
      ],
    },
  ]
}

function notesSidebar(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Engineering notes',
      items: [
        { text: 'Overview', link: '/notes/' },
        { text: 'ClickHouse notes', link: 'clickhouse-notes' },
        { text: 'Benchmark results', link: 'benchmark-results' },
        { text: 'Runbook', link: 'runbook' },
      ],
    },
  ]
}

function adrSidebar(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Architecture decisions',
      items: [
        { text: 'All decisions', link: '/adr/' },
        { text: 'ADR-0001 — No ORM', link: '0001-no-orm' },
      ],
    },
  ]
}
