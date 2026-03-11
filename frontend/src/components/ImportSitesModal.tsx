import React, { useState } from 'react'

import { uploadCaddyfilesMulti, type CaddyFile } from '../api/import'

type Props = {
  visible: boolean
  onClose: () => void
  onUploaded?: () => void
}

interface SiteEntry {
  filename: string;
  content: string;
}

export default function ImportSitesModal({ visible, onClose, onUploaded }: Props) {
  const [sites, setSites] = useState<SiteEntry[]>([{ filename: 'Caddyfile-1', content: '' }])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (!visible) return null

  const setSiteContent = (index: number, value: string) => {
    const s = [...sites]
    s[index] = { ...s[index], content: value }
    setSites(s)
  }

  const setSiteFilename = (index: number, value: string) => {
    const s = [...sites]
    s[index] = { ...s[index], filename: value }
    setSites(s)
  }

  const handleFileInput = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files
    if (!files || files.length === 0) return

    const newSites: SiteEntry[] = []
    for (let i = 0; i < files.length; i++) {
      try {
        const text = await files[i].text()
        newSites.push({ filename: files[i].name, content: text })
      } catch {
          // ignore read errors for individual files
          newSites.push({ filename: files[i].name, content: '' })
        }
    }
    if (newSites.length > 0) setSites(newSites)
  }

  const addSite = () => setSites(prev => [...prev, { filename: `Caddyfile-${prev.length + 1}`, content: '' }])
  const removeSite = (index: number) => setSites(prev => prev.filter((_, i) => i !== index))

  const handleSubmit = async () => {
    setError(null)
    setLoading(true)
    try {
      const cleaned: CaddyFile[] = sites.map((s, i) => ({
        filename: s.filename || `Caddyfile-${i + 1}`,
        content: s.content || '',
      }))
      await uploadCaddyfilesMulti(cleaned)
      setLoading(false)
      if (onUploaded) onUploaded()
      onClose()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(msg || 'Upload failed')
      setLoading(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      role="dialog"
      aria-modal="true"
      aria-labelledby="multi-site-modal-title"
      data-testid="multi-site-modal"
    >
      <div className="absolute inset-0 bg-black/60" onClick={onClose} />
      <div className="relative bg-dark-card rounded-lg p-6 w-[900px] max-w-full">
        <h3 id="multi-site-modal-title" className="text-xl font-semibold text-white mb-4">Multi-site Import</h3>
        <p className="text-gray-400 text-sm mb-4">Add each site's Caddyfile content separately, then parse them together.</p>

        {/* Hidden file input so E2E tests can programmatically upload multiple files */}
        <input
          type="file"
          accept=".caddy,.caddyfile,.txt,text/plain"
          multiple
          onChange={handleFileInput}
          style={{ display: 'none' }}
        />

        <div className="space-y-4 max-h-[60vh] overflow-auto mb-4">
          {sites.map((site, idx) => (
            <div key={idx} className="border border-gray-800 rounded-lg p-3">
              <div className="flex justify-between items-center mb-2">
                <input
                  type="text"
                  value={site.filename}
                  onChange={e => setSiteFilename(idx, e.target.value)}
                  className="text-sm text-gray-300 bg-transparent border-b border-gray-700 focus:border-blue-500 focus:outline-none"
                  placeholder={`Caddyfile-${idx + 1}`}
                />
                <div>
                  {sites.length > 1 && (
                    <button
                      onClick={() => removeSite(idx)}
                      className="text-red-400 text-sm hover:underline mr-2"
                    >
                      Remove
                    </button>
                  )}
                </div>
              </div>
              <textarea
                value={site.content}
                onChange={e => setSiteContent(idx, e.target.value)}
                placeholder={`example.com {\n  reverse_proxy localhost:8080\n}`}
                className="w-full h-48 bg-gray-900 border border-gray-700 rounded-lg p-3 text-white font-mono text-sm"
              />
            </div>
          ))}
        </div>

        {error && <div className="bg-red-900/20 border border-red-500 text-red-400 px-4 py-2 rounded mb-4">{error}</div>}

        <div className="flex gap-3 justify-end">
          <button onClick={addSite} className="px-4 py-2 bg-gray-800 text-white rounded">+ Add site</button>
          <button onClick={onClose} className="px-4 py-2 bg-gray-700 text-white rounded">Cancel</button>
          <button
            onClick={handleSubmit}
            disabled={loading}
            className="px-4 py-2 bg-blue-active text-white rounded disabled:opacity-60"
          >
            {loading ? 'Processing...' : 'Parse and Review'}
          </button>
        </div>
      </div>
    </div>
  )
}
