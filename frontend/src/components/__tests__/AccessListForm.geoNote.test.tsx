import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { AccessListForm } from '../AccessListForm'
import { AccessListGeoNote } from '../AccessListGeoNote'

import type { AccessListSelfCheckDraft } from '../../api/accessLists'

vi.mock('../../api/system', () => ({
  getMyIP: vi.fn(),
}))

vi.mock('../AccessListGeoNote', () => ({
  AccessListGeoNote: vi.fn(() => null),
}))

globalThis.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}))

const lastDraft = (): AccessListSelfCheckDraft => {
  const calls = vi.mocked(AccessListGeoNote).mock.calls
  const props = calls[calls.length - 1]?.[0]
  if (!props) throw new Error('AccessListGeoNote was not rendered')
  return props.draft
}

describe('AccessListForm country note', () => {
  const user = userEvent.setup()

  beforeEach(() => {
    vi.mocked(AccessListGeoNote).mockClear()
  })

  it('is not rendered for IP lists', () => {
    render(<AccessListForm onSubmit={vi.fn()} onCancel={vi.fn()} />)

    expect(AccessListGeoNote).not.toHaveBeenCalled()
  })

  it('is rendered for a country list and follows the edited countries', async () => {
    render(<AccessListForm onSubmit={vi.fn()} onCancel={vi.fn()} />)

    await user.selectOptions(screen.getByLabelText(/Type/i), 'geo_blacklist')
    expect(lastDraft()).toEqual({
      type: 'geo_blacklist',
      country_codes: '',
      ip_rules: '',
      local_network_only: false,
      enabled: true,
    })

    await user.selectOptions(screen.getByLabelText('Select Countries'), 'RU')
    await user.selectOptions(screen.getByLabelText('Select Countries'), 'CN')
    expect(lastDraft().country_codes).toBe('RU,CN')
  })

  it('reflects the enabled switch in the draft', async () => {
    render(
      <AccessListForm
        initialData={{
          uuid: 'list-1',
          name: 'Countries',
          description: '',
          type: 'geo_whitelist',
          ip_rules: '',
          country_codes: 'DE',
          local_network_only: false,
          enabled: true,
          created_at: '',
          updated_at: '',
        }}
        onSubmit={vi.fn()}
        onCancel={vi.fn()}
      />
    )
    expect(lastDraft()).toMatchObject({ type: 'geo_whitelist', country_codes: 'DE', enabled: true })

    await user.click(screen.getByRole('checkbox', { name: 'Enabled' }))
    expect(lastDraft().enabled).toBe(false)
  })

  it('never blocks saving while the note shows', async () => {
    const onSubmit = vi.fn()
    vi.mocked(AccessListGeoNote).mockImplementation(() => <p role="status">would refuse you</p>)
    render(<AccessListForm onSubmit={onSubmit} onCancel={vi.fn()} />)

    await user.type(screen.getByLabelText(/Name/), 'Countries')
    await user.selectOptions(screen.getByLabelText(/Type/i), 'geo_blacklist')
    await user.selectOptions(screen.getByLabelText('Select Countries'), 'RU')

    expect(screen.getByRole('status')).toHaveTextContent('would refuse you')
    const save = screen.getByRole('button', { name: 'Create' })
    expect(save).toBeEnabled()
    await user.click(save)
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ country_codes: 'RU' }))
  })
})
