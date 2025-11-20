import { ReactNode } from 'react'
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import Layout from '../Layout'
import { ThemeProvider } from '../../context/ThemeContext'

const renderWithProviders = (children: ReactNode) => {
  return render(
    <BrowserRouter>
      <ThemeProvider>
        {children}
      </ThemeProvider>
    </BrowserRouter>
  )
}

describe('Layout', () => {
  it('renders the application title', () => {
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    expect(screen.getAllByText('CPM+')[0]).toBeInTheDocument()
  })

  it('renders all navigation items', () => {
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Proxy Hosts')).toBeInTheDocument()
    expect(screen.getByText('Remote Servers')).toBeInTheDocument()
    expect(screen.getByText('Certificates')).toBeInTheDocument()
    expect(screen.getByText('Import Caddyfile')).toBeInTheDocument()
    expect(screen.getByText('Settings')).toBeInTheDocument()
  })

  it('renders children content', () => {
    renderWithProviders(
      <Layout>
        <div data-testid="test-content">Test Content</div>
      </Layout>
    )

    expect(screen.getByTestId('test-content')).toBeInTheDocument()
  })

  it('displays version information', () => {
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    expect(screen.getByText('Version 0.1.0')).toBeInTheDocument()
  })
})
