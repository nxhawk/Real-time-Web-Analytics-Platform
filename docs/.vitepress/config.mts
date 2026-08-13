import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'
import { shared } from './config/shared.mts'
import { en } from './config/en.mts'
import { vi } from './config/vi.mts'

// English is the root locale: its pages sit at the top of docs/ and are served without a
// language prefix. Vietnamese mirrors the same tree under docs/vi/.
//
// withMermaid() wraps the whole config so that ```mermaid fenced blocks render as diagrams.
// It registers the client component, keeps mermaid out of the SSR bundle, and reads the
// `mermaid` and `mermaidPlugin` keys off this same object — they are not a second argument.
// Diagram source stays in the Markdown file, so a diagram is reviewed in a pull request like
// any other text.
export default withMermaid({
  ...defineConfig({
    ...shared,
    locales: {
      root: { ...en },
      vi: { ...vi },
    },
  }),

  // Mermaid runtime options. The plugin switches `theme` to 'dark' by itself when VitePress
  // is in dark mode, so never pin a theme here — node colours come from the classDef palette
  // declared inside each diagram, and those read correctly on both backgrounds.
  mermaid: {
    startOnLoad: false,
    securityLevel: 'loose',
    // Do not set fontFamily to 'inherit': mermaid measures label width with its own font and
    // then renders with the inherited one, which clips multi-line labels inside a shape.
    flowchart: {
      curve: 'basis',
      htmlLabels: true,
      nodeSpacing: 45,
      rankSpacing: 55,
      padding: 12,
    },
    // A sequence diagram is scaled down to the width of the prose column, so every pixel of
    // lifeline spacing is paid for in font size. These values keep six lifelines legible.
    sequence: {
      actorMargin: 28,
      width: 118,
      boxMargin: 8,
      mirrorActors: false,
      wrap: true,
      actorFontSize: 14,
      messageFontSize: 14,
      noteFontSize: 13,
    },
  },

  // Applied to the wrapper <div> of every rendered diagram.
  mermaidPlugin: {
    class: 'mermaid pulse-diagram',
  },
})
