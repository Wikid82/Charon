import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { FileDropZone } from '../FileDropZone'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'en', changeLanguage: vi.fn() },
  }),
}))

const defaultProps = {
  id: 'cert-file',
  label: 'Certificate File',
  file: null as File | null,
  onFileChange: vi.fn(),
}

function createFile(name = 'test.pem', type = 'application/x-pem-file'): File {
  return new File(['cert-content'], name, { type })
}

describe('FileDropZone', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders label and empty drop zone', () => {
    render(<FileDropZone {...defaultProps} />)
    expect(screen.getByText('Certificate File')).toBeTruthy()
    expect(screen.getByText('certificates.dropFileHere')).toBeTruthy()
  })

  it('shows required asterisk when required', () => {
    render(<FileDropZone {...defaultProps} required />)
    expect(screen.getByText('*')).toBeTruthy()
  })

  it('displays file name when a file is provided', () => {
    const file = createFile('my-cert.pem')
    render(<FileDropZone {...defaultProps} file={file} />)
    expect(screen.getByText('my-cert.pem')).toBeTruthy()
  })

  it('displays format badge when file is provided', () => {
    const file = createFile('my-cert.pem')
    render(<FileDropZone {...defaultProps} file={file} formatBadge="PEM" />)
    expect(screen.getByText('PEM')).toBeTruthy()
  })

  it('triggers file input on click', async () => {
    render(<FileDropZone {...defaultProps} />)
    const dropZone = screen.getByRole('button')
    await userEvent.click(dropZone)
    // The hidden file input should exist
    const input = document.getElementById('cert-file') as HTMLInputElement
    expect(input).toBeTruthy()
    expect(input.type).toBe('file')
  })

  it('calls onFileChange when a file is selected via input', () => {
    render(<FileDropZone {...defaultProps} />)
    const input = document.getElementById('cert-file') as HTMLInputElement
    const file = createFile()
    fireEvent.change(input, { target: { files: [file] } })
    expect(defaultProps.onFileChange).toHaveBeenCalledWith(file)
  })

  it('calls onFileChange on drop', () => {
    render(<FileDropZone {...defaultProps} />)
    const dropZone = screen.getByRole('button')
    const file = createFile()

    fireEvent.dragOver(dropZone, { dataTransfer: { files: [file] } })
    fireEvent.drop(dropZone, { dataTransfer: { files: [file] } })

    expect(defaultProps.onFileChange).toHaveBeenCalledWith(file)
  })

  it('does not call onFileChange on drop when disabled', () => {
    render(<FileDropZone {...defaultProps} disabled />)
    const dropZone = screen.getByRole('button')
    const file = createFile()

    fireEvent.drop(dropZone, { dataTransfer: { files: [file] } })

    expect(defaultProps.onFileChange).not.toHaveBeenCalled()
  })

  it('activates via keyboard Enter', async () => {
    render(<FileDropZone {...defaultProps} />)
    const dropZone = screen.getByRole('button')
    fireEvent.keyDown(dropZone, { key: 'Enter' })
    // Should not throw; input ref click would be called
  })

  it('activates via keyboard Space', async () => {
    render(<FileDropZone {...defaultProps} />)
    const dropZone = screen.getByRole('button')
    fireEvent.keyDown(dropZone, { key: ' ' })
  })

  it('does not activate via keyboard when disabled', () => {
    render(<FileDropZone {...defaultProps} disabled />)
    const dropZone = screen.getByRole('button')
    fireEvent.keyDown(dropZone, { key: 'Enter' })
    // No crash, no file change
    expect(defaultProps.onFileChange).not.toHaveBeenCalled()
  })

  it('sets aria-disabled when disabled', () => {
    render(<FileDropZone {...defaultProps} disabled />)
    const dropZone = screen.getByRole('button')
    expect(dropZone.getAttribute('aria-disabled')).toBe('true')
  })

  it('has tabIndex=-1 when disabled', () => {
    render(<FileDropZone {...defaultProps} disabled />)
    const dropZone = screen.getByRole('button')
    expect(dropZone.tabIndex).toBe(-1)
  })

  it('has tabIndex=0 when not disabled', () => {
    render(<FileDropZone {...defaultProps} />)
    const dropZone = screen.getByRole('button')
    expect(dropZone.tabIndex).toBe(0)
  })

  it('has appropriate aria-label when file is selected', () => {
    const file = createFile('cert.pem')
    render(<FileDropZone {...defaultProps} file={file} />)
    const dropZone = screen.getByRole('button')
    expect(dropZone.getAttribute('aria-label')).toBe('Certificate File: cert.pem')
  })

  it('handles dragLeave event', () => {
    render(<FileDropZone {...defaultProps} />)
    const dropZone = screen.getByRole('button')
    fireEvent.dragOver(dropZone, { dataTransfer: { files: [] } })
    fireEvent.dragLeave(dropZone)
    // No crash; drag state should reset
  })

  it('sets accept attribute on input', () => {
    render(<FileDropZone {...defaultProps} accept=".pem,.crt" />)
    const input = document.getElementById('cert-file') as HTMLInputElement
    expect(input.getAttribute('accept')).toBe('.pem,.crt')
  })

  it('sets aria-required on input when required', () => {
    render(<FileDropZone {...defaultProps} required />)
    const input = document.getElementById('cert-file') as HTMLInputElement
    expect(input.getAttribute('aria-required')).toBe('true')
  })
})
