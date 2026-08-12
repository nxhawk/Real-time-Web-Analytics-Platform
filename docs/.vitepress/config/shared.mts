import { defineConfig } from 'vitepress'

/**
 * The repository name decides the GitHub Pages base path. Change both together, or every
 * stylesheet and link on the deployed site breaks.
 *
 * Serving from a custom domain instead? Set BASE to '/' and add docs/public/CNAME.
 */
export const REPO = 'nxhawk/Real-time-Web-Analytics-Platform'
export const BASE = `/${REPO.split('/')[1]}/`

/** Settings that apply to every language. */
export const shared = defineConfig({
  title: 'Pulse Analytics',
  base: BASE,
  cleanUrls: true,
  lastUpdated: true,

  // A broken link is a bug, so fail the build instead of shipping it.
  ignoreDeadLinks: false,

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: `${BASE}logo.svg` }],
    ['meta', { name: 'theme-color', content: '#3c82f6' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:site_name', content: 'Pulse Analytics' }],
  ],

  markdown: {
    lineNumbers: true,
    // Render ```mermaid blocks as diagrams is not enabled: it pulls a large runtime.
    // ASCII diagrams are used instead, matching PLAN.md.
    theme: { light: 'github-light', dark: 'github-dark' },
  },

  sitemap: {
    hostname: `https://${REPO.split('/')[0]}.github.io${BASE}`,
  },

  themeConfig: {
    logo: '/logo.svg',

    socialLinks: [{ icon: 'github', link: `https://github.com/${REPO}` }],

    search: {
      provider: 'local',
      options: {
        locales: {
          vi: {
            translations: {
              button: { buttonText: 'Tìm kiếm', buttonAriaLabel: 'Tìm kiếm' },
              modal: {
                displayDetails: 'Hiện chi tiết',
                resetButtonTitle: 'Xoá tìm kiếm',
                backButtonTitle: 'Quay lại',
                noResultsText: 'Không có kết quả cho',
                footer: {
                  selectText: 'chọn',
                  selectKeyAriaLabel: 'enter',
                  navigateText: 'di chuyển',
                  navigateUpKeyAriaLabel: 'mũi tên lên',
                  navigateDownKeyAriaLabel: 'mũi tên xuống',
                  closeText: 'đóng',
                  closeKeyAriaLabel: 'esc',
                },
              },
            },
          },
        },
      },
    },
  },
})
