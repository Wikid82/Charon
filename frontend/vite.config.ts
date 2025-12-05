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
  build: {
    outDir: 'dist',
    sourcemap: true,
    // Code splitting for better caching and parallel loading
    rollupOptions: {
      output: {
        manualChunks: {
          // React ecosystem - changes rarely
          'react-vendor': ['react', 'react-dom', 'react-router-dom'],
          // TanStack Query - changes rarely
          'query': ['@tanstack/react-query'],
          // Icons - large but cacheable
          'icons': ['lucide-react'],
        }
      }
    }
  }
})
