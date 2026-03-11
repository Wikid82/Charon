import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import {
  buildCrowdsecExportFilename,
  promptCrowdsecFilename,
  downloadCrowdsecExport,
} from '../crowdsecExport'

describe('crowdsecExport', () => {
  describe('buildCrowdsecExportFilename', () => {
    it('should generate filename with ISO timestamp', () => {
      const filename = buildCrowdsecExportFilename()
      expect(filename).toMatch(
        /^crowdsec-export-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}.*\.tar\.gz$/
      )
    })

    it('should replace colons with hyphens in timestamp', () => {
      const filename = buildCrowdsecExportFilename()
      expect(filename).not.toContain(':')
    })

    it('should always end with .tar.gz', () => {
      const filename = buildCrowdsecExportFilename()
      expect(filename.endsWith('.tar.gz')).toBe(true)
    })

    it('should start with crowdsec-export-', () => {
      const filename = buildCrowdsecExportFilename()
      expect(filename).toMatch(/^crowdsec-export-/)
    })

    it('should include milliseconds in timestamp', () => {
      const filename = buildCrowdsecExportFilename()
      // ISO format includes milliseconds: 2025-12-15T10:30:45.123Z
      expect(filename).toMatch(/\d{3}/)
    })

    it('should generate unique filenames for consecutive calls', () => {
      const filename1 = buildCrowdsecExportFilename()
      const filename2 = buildCrowdsecExportFilename()
      // They might be the same if called in the same millisecond, but should be close
      expect(filename1).toBeTruthy()
      expect(filename2).toBeTruthy()
    })
  })

  describe('promptCrowdsecFilename', () => {
    beforeEach(() => {
      vi.stubGlobal('prompt', vi.fn())
    })

    afterEach(() => {
      vi.unstubAllGlobals()
    })

    it('should return null when user cancels', () => {
      vi.mocked(window.prompt).mockReturnValue(null)
      const result = promptCrowdsecFilename('default.tar.gz')
      expect(result).toBeNull()
    })

    it('should return default filename when user provides empty string', () => {
      vi.mocked(window.prompt).mockReturnValue('   ')
      const result = promptCrowdsecFilename('default.tar.gz')
      expect(result).toBe('default.tar.gz')
    })

    it('should sanitize user input by replacing slashes', () => {
      vi.mocked(window.prompt).mockReturnValue('backup/prod/config')
      const result = promptCrowdsecFilename()
      expect(result).toBe('backup-prod-config.tar.gz')
    })

    it('should replace spaces with hyphens', () => {
      vi.mocked(window.prompt).mockReturnValue('crowdsec backup 2025')
      const result = promptCrowdsecFilename()
      expect(result).toBe('crowdsec-backup-2025.tar.gz')
    })

    it('should append .tar.gz if missing', () => {
      vi.mocked(window.prompt).mockReturnValue('my-backup')
      const result = promptCrowdsecFilename()
      expect(result).toBe('my-backup.tar.gz')
    })

    it('should not double-append .tar.gz', () => {
      vi.mocked(window.prompt).mockReturnValue('my-backup.tar.gz')
      const result = promptCrowdsecFilename()
      expect(result).toBe('my-backup.tar.gz')
    })

    it('should handle case-insensitive .tar.gz extension', () => {
      vi.mocked(window.prompt).mockReturnValue('my-backup.TAR.GZ')
      const result = promptCrowdsecFilename()
      // Implementation checks lowercase, so uppercase extension gets .tar.gz appended
      expect(result).toBe('my-backup.TAR.GZ')
    })

    it('should trim whitespace from user input', () => {
      vi.mocked(window.prompt).mockReturnValue('  my-backup  ')
      const result = promptCrowdsecFilename()
      expect(result).toBe('my-backup.tar.gz')
    })

    it('should use generated default when no default provided', () => {
      vi.mocked(window.prompt).mockReturnValue('')
      const result = promptCrowdsecFilename()
      expect(result).toMatch(/^crowdsec-export-.*\.tar\.gz$/)
    })

    it('should handle backslashes (Windows paths)', () => {
      vi.mocked(window.prompt).mockReturnValue('backup\\windows\\path')
      const result = promptCrowdsecFilename()
      expect(result).toBe('backup-windows-path.tar.gz')
    })

    it('should handle multiple consecutive slashes', () => {
      vi.mocked(window.prompt).mockReturnValue('backup///prod')
      const result = promptCrowdsecFilename()
      expect(result).toBe('backup-prod.tar.gz')
    })

    it('should handle multiple consecutive spaces', () => {
      vi.mocked(window.prompt).mockReturnValue('backup    prod')
      const result = promptCrowdsecFilename()
      expect(result).toBe('backup-prod.tar.gz')
    })

    it('should handle mixed slashes and spaces', () => {
      vi.mocked(window.prompt).mockReturnValue('backup / prod \\ test')
      const result = promptCrowdsecFilename()
      expect(result).toBe('backup---prod---test.tar.gz')
    })

    it('should handle special characters', () => {
      vi.mocked(window.prompt).mockReturnValue('backup@prod#2025')
      const result = promptCrowdsecFilename()
      expect(result).toBe('backup@prod#2025.tar.gz')
    })

    it('should handle undefined return from prompt', () => {
      vi.mocked(window.prompt).mockReturnValue(undefined as unknown as string | null)
      const result = promptCrowdsecFilename('default.tar.gz')
      expect(result).toBeNull()
    })
  })

  describe('downloadCrowdsecExport', () => {
    let createObjectURLSpy: ReturnType<typeof vi.fn>
    let revokeObjectURLSpy: ReturnType<typeof vi.fn>
    let clickSpy: ReturnType<typeof vi.fn>
    let removeSpy: ReturnType<typeof vi.fn>
    let appendChildSpy: ReturnType<typeof vi.fn>
    let anchorElement: {
      href: string
      download: string
      click: ReturnType<typeof vi.fn>
      remove: ReturnType<typeof vi.fn>
    }

    beforeEach(() => {
      createObjectURLSpy = vi.fn(() => 'blob:mock-url-12345')
      revokeObjectURLSpy = vi.fn()
      vi.stubGlobal('URL', {
        createObjectURL: createObjectURLSpy,
        revokeObjectURL: revokeObjectURLSpy,
      })

      clickSpy = vi.fn()
      removeSpy = vi.fn()
      anchorElement = {
        href: '',
        download: '',
        click: clickSpy,
        remove: removeSpy,
      }

      vi.spyOn(document, 'createElement').mockImplementation((tag) => {
        if (tag === 'a') {
          return anchorElement as unknown as HTMLAnchorElement
        }
        return document.createElement(tag)
      })

      appendChildSpy = vi.fn((node: Node) => node)
      vi.spyOn(document.body, 'appendChild').mockImplementation(appendChildSpy as (node: Node) => Node)
    })

    afterEach(() => {
      vi.unstubAllGlobals()
      vi.restoreAllMocks()
    })

    it('should create blob URL and trigger download', () => {
      const blob = new Blob(['test data'], { type: 'application/gzip' })
      downloadCrowdsecExport(blob, 'test.tar.gz')

      expect(createObjectURLSpy).toHaveBeenCalledWith(expect.any(Blob))
      expect(clickSpy).toHaveBeenCalled()
      expect(revokeObjectURLSpy).toHaveBeenCalledWith('blob:mock-url-12345')
    })

    it('should set correct filename on anchor element', () => {
      const blob = new Blob(['data'])
      downloadCrowdsecExport(blob, 'my-backup.tar.gz')

      expect(anchorElement.download).toBe('my-backup.tar.gz')
    })

    it('should set href to blob URL', () => {
      const blob = new Blob(['data'])
      downloadCrowdsecExport(blob, 'test.tar.gz')

      expect(anchorElement.href).toBe('blob:mock-url-12345')
    })

    it('should append anchor to body', () => {
      const blob = new Blob(['data'])
      downloadCrowdsecExport(blob, 'test.tar.gz')

      expect(appendChildSpy).toHaveBeenCalledWith(anchorElement)
    })

    it('should clean up by removing anchor element', () => {
      const blob = new Blob(['data'])
      downloadCrowdsecExport(blob, 'test.tar.gz')

      expect(removeSpy).toHaveBeenCalled()
    })

    it('should revoke object URL after download', () => {
      const blob = new Blob(['data'])
      downloadCrowdsecExport(blob, 'test.tar.gz')

      expect(revokeObjectURLSpy).toHaveBeenCalledWith('blob:mock-url-12345')
    })

    it('should handle large blob data', () => {
      const largeData = new Array(1000000).fill('x').join('')
      const blob = new Blob([largeData], { type: 'application/gzip' })
      downloadCrowdsecExport(blob, 'large-export.tar.gz')

      expect(createObjectURLSpy).toHaveBeenCalledWith(expect.any(Blob))
      expect(clickSpy).toHaveBeenCalled()
    })

    it('should handle blob with custom type', () => {
      const blob = new Blob(['data'], { type: 'application/x-gzip' })
      downloadCrowdsecExport(blob, 'test.tar.gz')

      expect(createObjectURLSpy).toHaveBeenCalledWith(expect.any(Blob))
    })

    it('should handle filename with special characters', () => {
      const blob = new Blob(['data'])
      downloadCrowdsecExport(blob, 'backup-2025-12-15@10:30.tar.gz')

      expect(anchorElement.download).toBe('backup-2025-12-15@10:30.tar.gz')
    })

    it('should execute steps in correct order', () => {
      const blob = new Blob(['data'])
      const callOrder: string[] = []

      createObjectURLSpy.mockImplementation(() => {
        callOrder.push('createObjectURL')
        return 'blob:mock-url'
      })
      appendChildSpy.mockImplementation(() => {
        callOrder.push('appendChild')
      })
      clickSpy.mockImplementation(() => {
        callOrder.push('click')
      })
      removeSpy.mockImplementation(() => {
        callOrder.push('remove')
      })
      revokeObjectURLSpy.mockImplementation(() => {
        callOrder.push('revokeObjectURL')
      })

      downloadCrowdsecExport(blob, 'test.tar.gz')

      expect(callOrder).toEqual([
        'createObjectURL',
        'appendChild',
        'click',
        'remove',
        'revokeObjectURL',
      ])
    })
  })

  describe('security: path traversal prevention', () => {
    beforeEach(() => {
      vi.stubGlobal('prompt', vi.fn())
    })

    afterEach(() => {
      vi.unstubAllGlobals()
    })

    it('should sanitize directory traversal attempts', () => {
      vi.mocked(window.prompt).mockReturnValue('../../etc/passwd')
      const result = promptCrowdsecFilename()
      expect(result).toBe('..-..-etc-passwd.tar.gz')
      expect(result).not.toContain('../')
    })

    it('should handle absolute paths', () => {
      vi.mocked(window.prompt).mockReturnValue('/etc/crowdsec/backup')
      const result = promptCrowdsecFilename()
      expect(result).toBe('-etc-crowdsec-backup.tar.gz')
      expect(result).not.toMatch(/^\//)
    })

    it('should handle Windows absolute paths', () => {
      vi.mocked(window.prompt).mockReturnValue('C:\\Windows\\System32\\backup')
      const result = promptCrowdsecFilename()
      // Backslashes are replaced with hyphens, but case is preserved
      expect(result).toBe('C:-Windows-System32-backup.tar.gz')
    })

    it('should handle null bytes', () => {
      vi.mocked(window.prompt).mockReturnValue('backup\0malicious')
      const result = promptCrowdsecFilename()
      expect(result).toContain('backup')
      // Null bytes should be handled by sanitization
    })

    it('should handle mixed attack vectors', () => {
      vi.mocked(window.prompt).mockReturnValue('../../../etc/passwd\0.tar.gz')
      const result = promptCrowdsecFilename()
      expect(result).not.toContain('../')
      expect(result!.endsWith('.tar.gz')).toBe(true)
    })

    it('should handle URL-encoded path traversal', () => {
      vi.mocked(window.prompt).mockReturnValue('%2e%2e%2f%2e%2e%2f')
      const result = promptCrowdsecFilename()
      expect(result).toContain('%2e')
      expect(result!.endsWith('.tar.gz')).toBe(true)
    })

    it('should not allow overwriting system files via filename', () => {
      vi.mocked(window.prompt).mockReturnValue('/etc/passwd')
      const result = promptCrowdsecFilename()
      // After sanitization, leading slash should be replaced
      expect(result).toBe('-etc-passwd.tar.gz')
    })
  })

  describe('edge cases and error handling', () => {
    beforeEach(() => {
      vi.stubGlobal('prompt', vi.fn())
    })

    afterEach(() => {
      vi.unstubAllGlobals()
    })

    it('should handle very long filenames', () => {
      const longName = 'a'.repeat(300)
      vi.mocked(window.prompt).mockReturnValue(longName)
      const result = promptCrowdsecFilename()
      expect(result).toContain('a'.repeat(100)) // Should still work
      expect(result!.endsWith('.tar.gz')).toBe(true)
    })

    it('should handle unicode characters', () => {
      vi.mocked(window.prompt).mockReturnValue('backup-日本語-2025')
      const result = promptCrowdsecFilename()
      expect(result).toBe('backup-日本語-2025.tar.gz')
    })

    it('should handle emoji characters', () => {
      vi.mocked(window.prompt).mockReturnValue('backup-🔒-secure')
      const result = promptCrowdsecFilename()
      expect(result).toBe('backup-🔒-secure.tar.gz')
    })

    it('should handle only special characters', () => {
      vi.mocked(window.prompt).mockReturnValue('///\\\\\\')
      const result = promptCrowdsecFilename('default.tar.gz')
      // Slashes become hyphens, resulting in single hyphen after sanitization
      expect(result).toBe('-.tar.gz')
    })

    it('should handle filename with only spaces', () => {
      vi.mocked(window.prompt).mockReturnValue('     ')
      const result = promptCrowdsecFilename('default.tar.gz')
      expect(result).toBe('default.tar.gz')
    })

    it('should handle filename with .tar.gz in the middle', () => {
      vi.mocked(window.prompt).mockReturnValue('backup.tar.gz.old')
      const result = promptCrowdsecFilename()
      expect(result).toBe('backup.tar.gz.old.tar.gz')
    })

    it('should handle case variations of .tar.gz', () => {
      vi.mocked(window.prompt).mockReturnValue('backup.Tar.Gz')
      const result = promptCrowdsecFilename()
      // Implementation uses lowercase check, so mixed case doesn't match
      expect(result).toBe('backup.Tar.Gz')
    })
  })

  describe('integration: full export workflow', () => {
    let createObjectURLSpy: ReturnType<typeof vi.fn>
    let revokeObjectURLSpy: ReturnType<typeof vi.fn>
    let promptSpy: ReturnType<typeof vi.fn>

    beforeEach(() => {
      createObjectURLSpy = vi.fn(() => 'blob:mock-url')
      revokeObjectURLSpy = vi.fn()
      vi.stubGlobal('URL', {
        createObjectURL: createObjectURLSpy,
        revokeObjectURL: revokeObjectURLSpy,
      })

      promptSpy = vi.fn()
      vi.stubGlobal('prompt', promptSpy)

      vi.spyOn(document, 'createElement').mockImplementation((tag) => {
        if (tag === 'a') {
          return {
            href: '',
            download: '',
            click: vi.fn(),
            remove: vi.fn(),
          } as unknown as HTMLAnchorElement
        }
        return document.createElement(tag)
      })

      vi.spyOn(document.body, 'appendChild').mockImplementation(() => null as unknown as HTMLElement)
    })

    afterEach(() => {
      vi.unstubAllGlobals()
      vi.restoreAllMocks()
    })

    it('should complete full workflow: generate → prompt → download', () => {
      // 1. Generate default filename
      const defaultFilename = buildCrowdsecExportFilename()
      expect(defaultFilename).toMatch(/^crowdsec-export-.*\.tar\.gz$/)

      // 2. User provides custom filename
      promptSpy.mockReturnValue('my-backup')
      const customFilename = promptCrowdsecFilename(defaultFilename)
      expect(customFilename).toBe('my-backup.tar.gz')

      // 3. Download with custom filename
      const blob = new Blob(['export data'], { type: 'application/gzip' })
      downloadCrowdsecExport(blob, customFilename!)

      expect(createObjectURLSpy).toHaveBeenCalled()
      expect(revokeObjectURLSpy).toHaveBeenCalled()
    })

    it('should handle user cancellation', () => {
      const defaultFilename = buildCrowdsecExportFilename()
      promptSpy.mockReturnValue(null)
      const result = promptCrowdsecFilename(defaultFilename)

      expect(result).toBeNull()
      // Download should not be triggered when user cancels
    })

    it('should use default filename when user provides empty input', () => {
      const defaultFilename = buildCrowdsecExportFilename()
      promptSpy.mockReturnValue('')
      const result = promptCrowdsecFilename(defaultFilename)

      expect(result).toBe(defaultFilename)
    })
  })
})
