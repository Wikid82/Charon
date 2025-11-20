import { useState } from 'react'
import { useImport } from '../hooks/useImport'
import ImportBanner from '../components/ImportBanner'
import ImportReviewTable from '../components/ImportReviewTable'

export default function ImportCaddy() {
  const { session, preview, loading, error, upload, commit, cancel } = useImport()
  const [content, setContent] = useState('')
  const [showReview, setShowReview] = useState(false)

  const handleUpload = async () => {
    if (!content.trim()) {
      alert('Please enter Caddyfile content')
      return
    }

    try {
      await upload(content)
      setShowReview(true)
    } catch (err) {
      // Error is already set by hook
    }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    const text = await file.text()
    setContent(text)
  }

  const handleCommit = async (resolutions: Record<string, string>) => {
    try {
      await commit(resolutions)
      setContent('')
      setShowReview(false)
      alert('Import completed successfully!')
    } catch (err) {
      // Error is already set by hook
    }
  }

  const handleCancel = async () => {
    if (confirm('Are you sure you want to cancel this import?')) {
      try {
        await cancel()
        setShowReview(false)
      } catch (err) {
        // Error is already set by hook
      }
    }
  }

  return (
    <div className="p-8">
      <h1 className="text-3xl font-bold text-white mb-6">Import Caddyfile</h1>

      {session && (
        <ImportBanner
          session={session}
          onReview={() => setShowReview(true)}
          onCancel={handleCancel}
        />
      )}

      {error && (
        <div className="bg-red-900/20 border border-red-500 text-red-400 px-4 py-3 rounded mb-6">
          {error}
        </div>
      )}

      {!session && (
        <div className="bg-dark-card rounded-lg border border-gray-800 p-6">
          <div className="mb-6">
            <h2 className="text-xl font-semibold text-white mb-2">Upload or Paste Caddyfile</h2>
            <p className="text-gray-400 text-sm">
              Import an existing Caddyfile to automatically create proxy host configurations.
              The system will detect conflicts and allow you to review changes before committing.
            </p>
          </div>

          <div className="space-y-4">
            {/* File Upload */}
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">
                Upload Caddyfile
              </label>
              <input
                type="file"
                accept=".caddyfile,.txt,text/plain"
                onChange={handleFileUpload}
                className="w-full text-sm text-gray-400 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-sm file:font-medium file:bg-blue-active file:text-white hover:file:bg-blue-hover file:cursor-pointer cursor-pointer"
              />
            </div>

            {/* Or Divider */}
            <div className="flex items-center gap-4">
              <div className="flex-1 border-t border-gray-700" />
              <span className="text-gray-500 text-sm">or paste content</span>
              <div className="flex-1 border-t border-gray-700" />
            </div>

            {/* Text Area */}
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">
                Caddyfile Content
              </label>
              <textarea
                value={content}
                onChange={e => setContent(e.target.value)}
                className="w-full h-96 bg-gray-900 border border-gray-700 rounded-lg p-4 text-white font-mono text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder={`example.com {
  reverse_proxy localhost:8080
}

api.example.com {
  reverse_proxy localhost:3000
}`}
              />
            </div>

            <button
              onClick={handleUpload}
              disabled={loading || !content.trim()}
              className="px-6 py-2 bg-blue-active hover:bg-blue-hover text-white rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              {loading ? 'Processing...' : 'Parse and Review'}
            </button>
          </div>
        </div>
      )}

      {showReview && preview && preview.preview && (
        <ImportReviewTable
          hosts={preview.preview.hosts}
          conflicts={Object.keys(preview.preview.conflicts)}
          errors={preview.preview.errors}
          onCommit={handleCommit}
          onCancel={() => setShowReview(false)}
        />
      )}
    </div>
  )
}
