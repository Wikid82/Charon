import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import Layout from '../Layout'

describe('Layout', () => {
  it('renders the application title', () => {
    render(
      <BrowserRouter>
        <Layout>
          <div>Test Content</div>
        </Layout>
      </BrowserRouter>
    )

    expect(screen.getByText('Caddy Proxy Manager+')).toBeInTheDocument()
  })

  it('renders all navigation items', () => {
    render(
      <BrowserRouter>
        <Layout>
          <div>Test Content</div>
        </Layout>
      </BrowserRouter>
    )

    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Proxy Hosts')).toBeInTheDocument()
    expect(screen.getByText('Remote Servers')).toBeInTheDocument()
    expect(screen.getByText('Import Caddyfile')).toBeInTheDocument()
    expect(screen.getByText('Settings')).toBeInTheDocument()
  })

  it('renders children content', () => {
    render(
      <BrowserRouter>
        <Layout>
          <div data-testid="test-content">Test Content</div>
        </Layout>
      </BrowserRouter>
    )

    expect(screen.getByTestId('test-content')).toBeInTheDocument()
  })

  it('displays version information', () => {
    render(
      <BrowserRouter>
        <Layout>
          <div>Test Content</div>
        </Layout>
      </BrowserRouter>
    )

    expect(screen.getByText('Version 0.1.0')).toBeInTheDocument()
  })
})
