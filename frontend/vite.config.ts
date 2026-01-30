import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
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
    testTimeout: 10000, // 10 seconds max per test
    hookTimeout: 10000, // 10 seconds for beforeEach/afterEach
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
    // TEMPORARY: Disable code splitting to diagnose React initialization issue
    // If this works, the problem is module loading order in async chunks
    chunkSizeWarningLimit: 2000,
    rollupOptions: {
      output: {
        // Disable code splitting - bundle everything into one file
        manualChunks: undefined,
        inlineDynamicImports: true
      }
    }
  }
})
