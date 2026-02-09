import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { createBackup } from '../api/backups'
import { useJSONImport } from '../hooks/useJSONImport'
import ImportSuccessModal from '../components/dialogs/ImportSuccessModal'

export default function ImportJSON() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const {
    preview,
    loading,
    error,
    upload,
    commit,
    committing,
    commitResult,
    clearCommitResult,
    cancel,
    reset,
  } = useJSONImport()
  const [content, setContent] = useState('')
  const [showReview, setShowReview] = useState(false)
  const [showSuccessModal, setShowSuccessModal] = useState(false)
  const [resolutions, setResolutions] = useState<Record<string, string>>({})
  const [names] = useState<Record<string, string>>({})

  const handleUpload = async () => {
    if (!content.trim()) {
      return
    }

    try {
      JSON.parse(content)
    } catch {
      alert(t('importJSON.invalidJSON'))
      return
    }

    try {
      await upload(content)
      setShowReview(true)
    } catch {
      // Error is handled by hook
    }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    const text = await file.text()
    setContent(text)
  }

  const handleCommit = async () => {
    try {
      await createBackup()
      await commit(resolutions, names)
      setContent('')
      setShowReview(false)
      setShowSuccessModal(true)
    } catch {
      // Error is handled by hook
    }
  }

  const handleCloseSuccessModal = () => {
    setShowSuccessModal(false)
    clearCommitResult()
  }

  const handleCancel = async () => {
    if (confirm(t('importJSON.cancelConfirm'))) {
      try {
        await cancel()
        setShowReview(false)
        reset()
      } catch {
        // Error is handled by hook
      }
    }
  }

  const handleResolutionChange = (domain: string, resolution: string) => {
    setResolutions((prev) => ({ ...prev, [domain]: resolution }))
  }

  return (
    <div className="p-8">
      <h1 className="text-3xl font-bold text-white mb-6">{t('importJSON.title')}</h1>

      {error && (
        <div
          className="bg-red-900/20 border border-red-500 text-red-400 px-4 py-3 rounded mb-6"
          role="alert"
        >
          {error.message}
        </div>
      )}

      {!showReview && (
        <div className="bg-dark-card rounded-lg border border-gray-800 p-6">
          <div className="mb-6">
            <h2 className="text-xl font-semibold text-white mb-2">
              {t('importJSON.title')}
            </h2>
            <p className="text-gray-400 text-sm">{t('importJSON.description')}</p>
          </div>

          <div className="space-y-4">
            <div>
              <label
                htmlFor="json-file-upload"
                className="block text-sm font-medium text-gray-300 mb-2"
              >
                {t('common.upload')}
              </label>
              <input
                id="json-file-upload"
                type="file"
                accept=".json,application/json"
                onChange={handleFileUpload}
                className="w-full text-sm text-gray-400 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-sm file:font-medium file:bg-blue-active file:text-white hover:file:bg-blue-hover file:cursor-pointer cursor-pointer"
                data-testid="json-import-dropzone"
              />
            </div>

            <div className="flex items-center gap-4">
              <div className="flex-1 border-t border-gray-700" />
              <span className="text-gray-500 text-sm">
                {t('importCaddy.orPasteContent')}
              </span>
              <div className="flex-1 border-t border-gray-700" />
            </div>

            <div>
              <label
                htmlFor="json-content"
                className="block text-sm font-medium text-gray-300 mb-2"
              >
                {t('importJSON.enterContent')}
              </label>
              <textarea
                id="json-content"
                value={content}
                onChange={(e) => setContent(e.target.value)}
                className="w-full h-96 bg-gray-900 border border-gray-700 rounded-lg p-4 text-white font-mono text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder={`{
  "proxy_hosts": [
    {
      "domain_names": ["example.com"],
      "forward_host": "192.168.1.100",
      "forward_port": 8080,
      "forward_scheme": "http"
    }
  ]
}`}
              />
            </div>

            <button
              type="button"
              onClick={handleUpload}
              disabled={loading || !content.trim()}
              className="px-6 py-2 bg-blue-active hover:bg-blue-hover text-white rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              {loading ? t('common.loading') : t('importJSON.upload')}
            </button>
          </div>
        </div>
      )}

      {showReview && preview?.preview && (
        <div className="bg-dark-card rounded-lg border border-gray-800 p-6">
          <h2 className="text-xl font-semibold text-white mb-4">
            {t('importJSON.previewTitle')}
          </h2>

          {preview.preview.errors.length > 0 && (
            <div
              className="bg-red-900/20 border border-red-500 text-red-400 px-4 py-3 rounded mb-4"
              role="alert"
            >
              <ul className="list-disc list-inside">
                {preview.preview.errors.map((err, idx) => (
                  <li key={idx}>{err}</li>
                ))}
              </ul>
            </div>
          )}

          <div className="overflow-x-auto mb-6">
            <table className="w-full text-sm text-left text-gray-300">
              <caption className="sr-only">JSON Import Preview</caption>
              <thead className="text-xs uppercase bg-gray-800 text-gray-400">
                <tr>
                  <th scope="col" className="px-4 py-3">
                    {t('proxyHosts.domainNames')}
                  </th>
                  <th scope="col" className="px-4 py-3">
                    {t('proxyHosts.forwardHost')}
                  </th>
                  <th scope="col" className="px-4 py-3">
                    {t('proxyHosts.forwardPort')}
                  </th>
                  <th scope="col" className="px-4 py-3">
                    {t('proxyHosts.sslForced')}
                  </th>
                  <th scope="col" className="px-4 py-3">
                    {t('common.status')}
                  </th>
                  {preview.preview.conflicts.length > 0 && (
                    <th scope="col" className="px-4 py-3">
                      {t('common.actions')}
                    </th>
                  )}
                </tr>
              </thead>
              <tbody>
                {preview.preview.hosts.map((host, idx) => {
                  const isConflict = preview.preview.conflicts.includes(
                    host.domain_names
                  )
                  return (
                    <tr key={idx} className="border-b border-gray-700">
                      <td className="px-4 py-3">{host.domain_names}</td>
                      <td className="px-4 py-3">
                        {host.forward_scheme}://{host.forward_host}
                      </td>
                      <td className="px-4 py-3">{host.forward_port}</td>
                      <td className="px-4 py-3">
                        {host.ssl_forced ? t('common.yes') : t('common.no')}
                      </td>
                      <td className="px-4 py-3">
                        {isConflict ? (
                          <span className="text-yellow-400">
                            {t('importJSON.conflict')}
                          </span>
                        ) : (
                          <span className="text-green-400">
                            {t('importJSON.new')}
                          </span>
                        )}
                      </td>
                      {preview.preview.conflicts.length > 0 && (
                        <td className="px-4 py-3">
                          {isConflict && (
                            <select
                              value={resolutions[host.domain_names] || 'skip'}
                              onChange={(e) =>
                                handleResolutionChange(
                                  host.domain_names,
                                  e.target.value
                                )
                              }
                              className="bg-gray-800 border border-gray-600 text-white rounded px-2 py-1 text-sm"
                              aria-label={`Resolution for ${host.domain_names}`}
                            >
                              <option value="skip">{t('importJSON.skip')}</option>
                              <option value="keep">{t('importJSON.keep')}</option>
                              <option value="replace">
                                {t('importJSON.replace')}
                              </option>
                            </select>
                          )}
                        </td>
                      )}
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>

          <div className="flex gap-4">
            <button
              type="button"
              onClick={handleCommit}
              disabled={committing}
              className="px-6 py-2 bg-green-600 hover:bg-green-700 text-white rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              {committing ? t('common.loading') : t('importJSON.import')}
            </button>
            <button
              type="button"
              onClick={handleCancel}
              className="px-6 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg font-medium transition-colors"
            >
              {t('common.cancel')}
            </button>
          </div>
        </div>
      )}

      <ImportSuccessModal
        visible={showSuccessModal}
        onClose={handleCloseSuccessModal}
        onNavigateDashboard={() => {
          handleCloseSuccessModal()
          navigate('/')
        }}
        onNavigateHosts={() => {
          handleCloseSuccessModal()
          navigate('/proxy-hosts')
        }}
        results={commitResult}
      />
    </div>
  )
}
