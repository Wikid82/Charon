import { useState } from 'react'
import { uploadCaddyfilesMulti } from '../api/import'

type Props = {
  visible: boolean
  onClose: () => void
  onUploaded?: () => void
}

export default function ImportSitesModal({ visible, onClose, onUploaded }: Props) {
  const [sites, setSites] = useState<string[]>([''])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (!visible) return null

  const setSite = (index: number, value: string) => {
    const s = [...sites]
    s[index] = value
    setSites(s)
  }

  const addSite = () => setSites(prev => [...prev, ''])
  const removeSite = (index: number) => setSites(prev => prev.filter((_, i) => i !== index))

  const handleSubmit = async () => {
    setError(null)
    setLoading(true)
    try {
      const cleaned = sites.map(s => s || '')
      await uploadCaddyfilesMulti(cleaned)
      setLoading(false)
      onUploaded && onUploaded()
      onClose()
    } catch (err: any) {
      setError(err?.message || 'Upload failed')
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/60" onClick={onClose} />
      <div className="relative bg-dark-card rounded-lg p-6 w-[900px] max-w-full">
        <h3 className="text-xl font-semibold text-white mb-4">Multi-site Import</h3>
        <p className="text-gray-400 text-sm mb-4">Add each site's Caddyfile content separately, then parse them together.</p>

        <div className="space-y-4 max-h-[60vh] overflow-auto mb-4">
          {sites.map((s, idx) => (
            <div key={idx} className="border border-gray-800 rounded-lg p-3">
              <div className="flex justify-between items-center mb-2">
                <div className="text-sm text-gray-300">Site {idx + 1}</div>
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
                value={s}
                onChange={e => setSite(idx, e.target.value)}
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
