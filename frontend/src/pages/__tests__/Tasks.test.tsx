import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import Tasks from '../Tasks'

let mockUser: { role: string } | undefined

vi.mock('../../hooks/useAuth', () => ({
  useAuth: () => ({ user: mockUser }),
}))

const renderTasks = (path = '/tasks/backups') =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="tasks" element={<Tasks />}>
          <Route path="backups" element={<div>backups outlet</div>} />
          <Route path="database" element={<div>database outlet</div>} />
        </Route>
      </Routes>
    </MemoryRouter>
  )

describe('Tasks tabs', () => {
  beforeEach(() => {
    mockUser = { role: 'admin' }
  })

  it('shows Backups, Logs and Database tabs for an admin', () => {
    renderTasks()
    expect(screen.getByRole('link', { name: 'Backups' })).toHaveAttribute('href', '/tasks/backups')
    expect(screen.getByRole('link', { name: 'Logs' })).toHaveAttribute('href', '/tasks/logs')
    expect(screen.getByRole('link', { name: 'Database' })).toHaveAttribute('href', '/tasks/database')
    expect(screen.getByText('backups outlet')).toBeInTheDocument()
  })

  it('hides the Database tab for a non-admin and keeps the other tabs', () => {
    mockUser = { role: 'user' }
    renderTasks()
    expect(screen.getByRole('link', { name: 'Backups' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Logs' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Database' })).toBeNull()
  })

  it('hides the Database tab when no user is loaded', () => {
    mockUser = undefined
    renderTasks()
    expect(screen.queryByRole('link', { name: 'Database' })).toBeNull()
  })

  it('highlights the active tab', () => {
    renderTasks('/tasks/database')
    expect(screen.getByRole('link', { name: 'Database' }).className).toContain('bg-blue-50')
    expect(screen.getByRole('link', { name: 'Backups' }).className).not.toContain('bg-blue-50')
    expect(screen.getByText('database outlet')).toBeInTheDocument()
  })
})
