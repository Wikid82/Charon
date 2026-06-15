import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'

import { ServiceHealthWidget } from '../ServiceHealthWidget'

import type { StatsHealth } from '../../../api/stats'

const healthOk: StatsHealth = { dropped_count: 0 }
const healthDegraded: StatsHealth = { dropped_count: 12 }

describe('ServiceHealthWidget', () => {
  it('renders card title', () => {
    render(<ServiceHealthWidget health={healthOk} isLoading={false} wsConnected={false} />)

    expect(screen.getByText('Stats Service Health')).toBeInTheDocument()
  })

  it('shows Live badge when WebSocket is connected', () => {
    render(<ServiceHealthWidget health={healthOk} isLoading={false} wsConnected />)

    expect(screen.getByText('Live')).toBeInTheDocument()
  })

  it('shows Offline badge when WebSocket is disconnected', () => {
    render(<ServiceHealthWidget health={healthOk} isLoading={false} wsConnected={false} />)

    expect(screen.getByText('Offline')).toBeInTheDocument()
  })

  it('shows polling message when not connected via WebSocket', () => {
    render(<ServiceHealthWidget health={healthOk} isLoading={false} wsConnected={false} />)

    expect(screen.getByText('Polling every 30 s')).toBeInTheDocument()
  })

  it('shows connected message when WebSocket is live', () => {
    render(<ServiceHealthWidget health={healthOk} isLoading={false} wsConnected />)

    expect(screen.getByText('Connected via WebSocket')).toBeInTheDocument()
  })

  it('shows 0 dropped events when health is ok', () => {
    render(<ServiceHealthWidget health={healthOk} isLoading={false} wsConnected={false} />)

    expect(screen.getByText('0 events dropped')).toBeInTheDocument()
  })

  it('shows warning message when dropped_count > 0', () => {
    render(<ServiceHealthWidget health={healthDegraded} isLoading={false} wsConnected={false} />)

    expect(screen.getByText(/12 events dropped/)).toBeInTheDocument()
    expect(screen.getByText(/ingester may be overloaded/)).toBeInTheDocument()
  })

  it('shows dropped count in the metric display', () => {
    render(<ServiceHealthWidget health={healthDegraded} isLoading={false} wsConnected={false} />)

    expect(screen.getByLabelText('12 dropped')).toBeInTheDocument()
  })

  it('renders loading skeletons when isLoading is true', () => {
    render(<ServiceHealthWidget health={undefined} isLoading wsConnected={false} />)

    // Badge should not be rendered while loading
    expect(screen.queryByText('Live')).not.toBeInTheDocument()
    expect(screen.queryByText('Offline')).not.toBeInTheDocument()
  })

  it('falls back to 0 when health is undefined', () => {
    render(<ServiceHealthWidget health={undefined} isLoading={false} wsConnected={false} />)

    expect(screen.getByText('0 events dropped')).toBeInTheDocument()
  })
})
