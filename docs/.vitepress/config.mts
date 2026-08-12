import { defineConfig } from 'vitepress'
import { shared } from './config/shared.mts'
import { en } from './config/en.mts'
import { vi } from './config/vi.mts'

// English is the root locale: its pages sit at the top of docs/ and are served without a
// language prefix. Vietnamese mirrors the same tree under docs/vi/.
export default defineConfig({
  ...shared,
  locales: {
    root: { ...en },
    vi: { ...vi },
  },
})
