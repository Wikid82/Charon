import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { vi } from 'vitest'

import ImportSitesModal from './ImportSitesModal'
import { type CaddyFile } from '../api/import'

// Mock the upload API used by the component
const mockUpload = vi.fn()
vi.mock('../api/import', () => ({
  uploadCaddyfilesMulti: (files: CaddyFile[]) => mockUpload(files),
}))

describe('ImportSitesModal', () => {
  beforeEach(() => {
    mockUpload.mockReset()
  })

  test('renders modal, add and remove sites, and edits textarea', () => {
    const onClose = vi.fn()
    render(<ImportSitesModal visible={true} onClose={onClose} />)

    // modal container is present
    expect(screen.getByTestId('multi-site-modal')).toBeInTheDocument()

    // initially one site with filename input and content textarea
    const textareas = screen.getAllByRole('textbox').filter(el => el.tagName === 'TEXTAREA')
    expect(textareas.length).toBe(1)

    // add a site -> two sites
    fireEvent.click(screen.getByText('+ Add site'))
    const textareasAfterAdd = screen.getAllByRole('textbox').filter(el => el.tagName === 'TEXTAREA')
    expect(textareasAfterAdd.length).toBe(2)

    // remove the second site (use getAllByText since multiple Remove buttons now exist)
    const removeButtons = screen.getAllByText('Remove')
    fireEvent.click(removeButtons[removeButtons.length - 1])
    const textareasAfterRemove = screen.getAllByRole('textbox').filter(el => el.tagName === 'TEXTAREA')
    expect(textareasAfterRemove.length).toBe(1)

    // type into textarea
    const ta = screen.getAllByRole('textbox').find(el => el.tagName === 'TEXTAREA')
    fireEvent.change(ta!, { target: { value: 'example.com { reverse_proxy 127.0.0.1:8080 }' } })
    expect((ta as HTMLTextAreaElement).value).toContain('example.com')
  })

  test('reads multiple files via hidden input and submits successfully', async () => {
    const onClose = vi.fn()
    const onUploaded = vi.fn()
    mockUpload.mockResolvedValueOnce(undefined)

    const { container } = render(<ImportSitesModal visible={true} onClose={onClose} onUploaded={onUploaded} />)

    // find the hidden file input
    const input: HTMLInputElement | null = container.querySelector('input[type="file"]')
    expect(input).toBeTruthy()

    // create two files (note: jsdom's File.text() returns empty strings, so we'll set content manually)
    const f1 = new File(['site1'], 'site1.caddy', { type: 'text/plain' })
    const f2 = new File(['site2'], 'site2.caddy', { type: 'text/plain' })

    // fire change event with files
    fireEvent.change(input!, { target: { files: [f1, f2] } })

    // after input, two textareas should appear (one per file)
    await waitFor(() => {
      const textareas = screen.getAllByRole('textbox').filter(el => el.tagName === 'TEXTAREA')
      expect(textareas.length).toBe(2)
    })

    // Manually fill textareas since jsdom's File.text() doesn't work correctly
    const textareas = screen.getAllByRole('textbox').filter(el => el.tagName === 'TEXTAREA')
    fireEvent.change(textareas[0], { target: { value: 'site1' } })
    fireEvent.change(textareas[1], { target: { value: 'site2' } })

    // submit
    fireEvent.click(screen.getByText('Parse and Review'))

    await waitFor(() => expect(mockUpload).toHaveBeenCalled())
    // New API contract: files are passed as {filename, content} objects
    expect(mockUpload).toHaveBeenCalledWith([
      { filename: 'site1.caddy', content: 'site1' },
      { filename: 'site2.caddy', content: 'site2' },
    ])
    expect(onUploaded).toHaveBeenCalled()
    expect(onClose).toHaveBeenCalled()
  })

  test('displays error when upload fails', async () => {
    const onClose = vi.fn()
    mockUpload.mockRejectedValueOnce(new Error('upload-failed'))

    render(<ImportSitesModal visible={true} onClose={onClose} />)

    // click submit with default empty site
    fireEvent.click(screen.getByText('Parse and Review'))

    // error message appears
    expect(await screen.findByText(/upload-failed|Upload failed/i)).toBeInTheDocument()
  })
})
