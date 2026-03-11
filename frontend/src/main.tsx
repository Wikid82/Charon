import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import ReactDOM from 'react-dom/client'

import App from './App.tsx'
import { LanguageProvider } from './context/LanguageContext'
import { ThemeProvider } from './context/ThemeContext'
import i18n from './i18n'
import './index.css'

// Global query client with optimized defaults for performance
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 30, // 30 seconds - reduces unnecessary refetches
      gcTime: 1000 * 60 * 5, // 5 minutes garbage collection
      refetchOnWindowFocus: false, // Prevents refetch when switching tabs
      refetchOnReconnect: 'always', // Refetch when network reconnects
      retry: 1, // Only retry failed requests once
    },
  },
})

// Wait for i18next to be fully initialized before rendering
// Prevents race condition where React renders before translations are loaded
const renderApp = () => {
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <LanguageProvider>
            <App />
          </LanguageProvider>
        </ThemeProvider>
      </QueryClientProvider>
    </React.StrictMode>,
  )
}

if (i18n.isInitialized) {
  renderApp()
} else {
  i18n.on('initialized', renderApp)
}
