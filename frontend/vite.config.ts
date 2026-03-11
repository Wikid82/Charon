import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

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
