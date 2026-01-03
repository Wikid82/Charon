import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/setupTests.ts',
    coverage: {
      provider: 'istanbul',
      reporter: ['text', 'json-summary', 'lcov'],
      reportsDirectory: './coverage',
      exclude: [
        'node_modules/',
        'src/setupTests.ts',
        '**/*.d.ts',
        '**/*.config.*',
        '**/mockData',
        'dist/'
      ]
    }
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    // Increase chunk size warning limit to 600KB (reasonable for modern networks)
    chunkSizeWarningLimit: 600,
    // Code splitting for better caching and parallel loading
    rollupOptions: {
      output: {
        manualChunks: (id) => {
          // React core - changes rarely
          if (id.includes('node_modules/react') ||
              id.includes('node_modules/react-dom') ||
              id.includes('node_modules/react-router')) {
            return 'react-vendor'
          }

          // TanStack Query - changes rarely
          if (id.includes('node_modules/@tanstack/react-query')) {
            return 'query'
          }

          // Radix UI components - large and stable
          if (id.includes('node_modules/@radix-ui')) {
            return 'radix-ui'
          }

          // Recharts for dashboard - large but used infrequently
          if (id.includes('node_modules/recharts')) {
            return 'recharts'
          }

          // Icons - large but cacheable
          if (id.includes('node_modules/lucide-react')) {
            return 'icons'
          }

          // All other node_modules into vendor chunk
          if (id.includes('node_modules')) {
            return 'vendor'
          }
        }
      }
    }
  }
})
