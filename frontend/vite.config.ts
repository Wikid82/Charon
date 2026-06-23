import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin } from 'vite'

// Vite plugin: ensure the inline theme-init script appears after all injected
// <link rel="stylesheet"> tags in the built index.html.
// This is required for AC-02 (FOUC fix) — the theme script must run after CSS
// is declared so browsers do not trigger a forced-layout style recalculation.
function themeScriptAfterStylesheets(): Plugin {
  return {
    name: 'theme-script-after-stylesheets',
    enforce: 'post',
    transformIndexHtml(html) {
      // Extract the inline theme-init script element
      const scriptMatch = html.match(/<script>!function\(\).*?<\/script>/s)
      if (!scriptMatch) return html
      const scriptTag = scriptMatch[0]

      // Remove it from its current position
      let result = html.replace(scriptTag, '')

      // Insert it immediately before </head> so it follows all injected <link> tags
      result = result.replace('</head>', `  ${scriptTag}\n  </head>`)

      return result
    },
  }
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), themeScriptAfterStylesheets()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    rolldownOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules/recharts') || id.includes('node_modules/d3-')) {
            return 'vendor-charts'
          }
          if (id.includes('node_modules/i18next') || id.includes('node_modules/react-i18next')) {
            return 'vendor-i18n'
          }
          if (id.includes('node_modules/@radix-ui') || id.includes('node_modules/lucide-react')) {
            return 'vendor-ui'
          }
          if (id.includes('node_modules/@tanstack')) {
            return 'vendor-query'
          }
          if (
            id.includes('node_modules/react/') ||
            id.includes('node_modules/react-dom/') ||
            id.includes('node_modules/react-router-dom/')
          ) {
            return 'vendor-react'
          }
        }
      }
    }
  }
})
