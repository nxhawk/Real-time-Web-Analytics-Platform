import { defineConfig, type DefaultTheme } from 'vitepress'
import { REPO } from './shared.mts'

/** Vietnamese lives under /vi/. Every path below is relative to that prefix. */
export const vi = defineConfig({
  label: 'Tiếng Việt',
  lang: 'vi-VN',
  description:
    'Nền tảng web analytics real-time tự host — Go, ClickHouse, Kafka và Next.js.',

  themeConfig: {
    nav: nav(),
    sidebar: {
      '/vi/guide/': { base: '/vi/guide/', items: guideSidebar() },
      '/vi/reference/': { base: '/vi/reference/', items: referenceSidebar() },
      '/vi/notes/': { base: '/vi/notes/', items: notesSidebar() },
      '/vi/adr/': { base: '/vi/adr/', items: adrSidebar() },
    },

    editLink: {
      pattern: `https://github.com/${REPO}/edit/main/docs/:path`,
      text: 'Sửa trang này trên GitHub',
    },

    docFooter: { prev: 'Trang trước', next: 'Trang sau' },
    outline: { level: [2, 3], label: 'Nội dung trang' },
    lastUpdated: { text: 'Cập nhật lần cuối' },
    returnToTopLabel: 'Lên đầu trang',
    darkModeSwitchLabel: 'Giao diện',
    lightModeSwitchTitle: 'Chuyển sang nền sáng',
    darkModeSwitchTitle: 'Chuyển sang nền tối',
    sidebarMenuLabel: 'Danh mục',
    langMenuLabel: 'Đổi ngôn ngữ',

    footer: {
      message: 'Phát hành theo giấy phép MIT.',
      copyright: `Bản quyền © 2026 <a href="https://github.com/${REPO.split('/')[0]}">nxhawk</a>`,
    },
  },
})

function nav(): DefaultTheme.NavItem[] {
  return [
    { text: 'Hướng dẫn', link: '/vi/guide/introduction', activeMatch: '/vi/guide/' },
    { text: 'Tra cứu', link: '/vi/reference/api', activeMatch: '/vi/reference/' },
    { text: 'Ghi chép', link: '/vi/notes/', activeMatch: '/vi/notes/' },
    { text: 'ADR', link: '/vi/adr/', activeMatch: '/vi/adr/' },
    { text: 'Lộ trình', link: '/vi/roadmap' },
    {
      text: 'Tài liệu kế hoạch',
      items: [
        { text: 'PLAN.md — đặc tả kỹ thuật', link: `https://github.com/${REPO}/blob/main/PLAN.md` },
        { text: 'PHASES.md — giai đoạn triển khai', link: `https://github.com/${REPO}/blob/main/PHASES.md` },
        { text: 'TODO.md — checklist', link: `https://github.com/${REPO}/blob/main/TODO.md` },
        { text: 'DEPLOY-AWS.md — hạ tầng', link: `https://github.com/${REPO}/blob/main/DEPLOY-AWS.md` },
      ],
    },
  ]
}

function guideSidebar(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Bắt đầu',
      collapsed: false,
      items: [
        { text: 'Giới thiệu', link: 'introduction' },
        { text: 'Chạy thử nhanh', link: 'quick-start' },
        { text: 'Cấu hình', link: 'configuration' },
      ],
    },
    {
      text: 'Hiểu hệ thống',
      collapsed: false,
      items: [
        { text: 'Kiến trúc', link: 'architecture' },
        { text: 'Cấu trúc dự án', link: 'project-structure' },
      ],
    },
    {
      text: 'Bắt tay vào làm',
      collapsed: false,
      items: [
        { text: 'Đóng góp', link: 'contributing' },
        { text: 'Triển khai', link: 'deployment' },
      ],
    },
  ]
}

function referenceSidebar(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Tra cứu',
      items: [
        { text: 'HTTP API', link: 'api' },
        { text: 'Event schema', link: 'event-schema' },
        { text: 'Schema ClickHouse', link: 'clickhouse' },
        { text: 'Observability', link: 'observability' },
      ],
    },
  ]
}

function notesSidebar(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Ghi chép kỹ thuật',
      items: [
        { text: 'Tổng quan', link: '/vi/notes/' },
        { text: 'Ghi chép ClickHouse', link: 'clickhouse-notes' },
        { text: 'Kết quả benchmark', link: 'benchmark-results' },
        { text: 'Runbook', link: 'runbook' },
      ],
    },
  ]
}

function adrSidebar(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Quyết định kiến trúc',
      items: [
        { text: 'Tất cả quyết định', link: '/vi/adr/' },
        { text: 'ADR-0001 — Không dùng ORM', link: '0001-no-orm' },
      ],
    },
  ]
}
