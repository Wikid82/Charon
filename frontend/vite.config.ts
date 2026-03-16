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
    chunkSizeWarningLimit: 2000,
    rolldownOptions: {
      output: {
        // Disable code splitting — single bundle for React init stability
        // codeSplitting: false is the Rolldown-native approach
        // (inlineDynamicImports is deprecated in Rolldown)
        codeSplitting: false
      }
    }
  }
})
