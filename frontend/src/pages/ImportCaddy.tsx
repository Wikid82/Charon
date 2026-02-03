import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { AxiosError } from 'axios'
import { createBackup } from '../api/backups'
import { useImport } from '../hooks/useImport'
import ImportBanner from '../components/ImportBanner'
import ImportReviewTable from '../components/ImportReviewTable'
import ImportSitesModal from '../components/ImportSitesModal'
import ImportSuccessModal from '../components/dialogs/ImportSuccessModal'

/** Response data structure for import API errors containing warnings */
interface ImportErrorResponse {
  error?: string;
  warning?: string;
}

export default function ImportCaddy() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { session, preview, loading, error, upload, commit, cancel, commitResult, clearCommitResult } = useImport()
  const [content, setContent] = useState('')
  const [showReview, setShowReview] = useState(false)
  const [showMultiModal, setShowMultiModal] = useState(false)
  const [showSuccessModal, setShowSuccessModal] = useState(false)
  // Warning extracted from 400 error responses (e.g., file_server detection)
  const [warningFromError, setWarningFromError] = useState<string | null>(null)

  const handleUpload = async () => {
    if (!content.trim()) {
      alert(t('importCaddy.enterCaddyfileContent'))
      return
    }

    // Clear any previous warning from error responses
    setWarningFromError(null)

    try {
      await upload(content)
      setShowReview(true)
    } catch (err) {
      // Check if error response contains a warning (e.g., file_server detected)
      const axiosErr = err as AxiosError<ImportErrorResponse>
      if (axiosErr.response?.data?.warning) {
        setWarningFromError(axiosErr.response.data.warning)
      }
      // Other error handling is done by hook
    }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    const text = await file.text()
    setContent(text)
  }

  const handleCommit = async (resolutions: Record<string, string>, names: Record<string, string>) => {
    try {
      // Create a backup before committing import to allow rollback
      await createBackup()
      await commit(resolutions, names)
      setContent('')
      setShowReview(false)
      setShowSuccessModal(true)
    } catch {
      // Error is already set by hook
    }
  }

  const handleCloseSuccessModal = () => {
    setShowSuccessModal(false)
    clearCommitResult()
  }

  const handleCancel = async () => {
    if (confirm(t('importCaddy.cancelConfirm'))) {
      try {
        await cancel()
        setShowReview(false)
      } catch {
        // Error is already set by hook
      }
    }
  }

  return (
    <div className="p-8">
      <h1 className="text-3xl font-bold text-white mb-6">{t('importCaddy.title')}</h1>

      {session && (
        <div data-testid="import-banner">
          <ImportBanner
            session={session}
            onReview={() => setShowReview(true)}
            onCancel={handleCancel}
          />
        </div>
      )}

      {error && (
        <div className="bg-red-900/20 border border-red-500 text-red-400 px-4 py-3 rounded mb-6">
          {error}
        </div>
      )}

      {/* Backend-provided warning (e.g. file_server-only) */}
      {preview?.warning && (
        <div className="bg-yellow-900/20 border border-yellow-500 text-yellow-400 px-4 py-3 rounded mb-6">
          <p className="font-bold">{t('importCaddy.warningTitle')}</p>
          <p className="text-sm mt-1">{preview.warning}</p>
        </div>
      )}

      {/* Warning extracted from 400 error response (e.g., file_server detection) */}
      {warningFromError && (
        <div className="bg-yellow-900/20 border border-yellow-500 text-yellow-400 px-4 py-3 rounded mb-6" data-testid="import-warning-banner">
          <p className="font-bold">{t('importCaddy.warningTitle')}</p>
          <p className="text-sm mt-1">{warningFromError}</p>
        </div>
      )}

      {/* Show warning if preview is empty but session exists (e.g. mounted file was empty or invalid) */}
      {session && preview && preview.preview && preview.preview.hosts.length === 0 && (
        <div className="bg-yellow-900/20 border border-yellow-500 text-yellow-400 px-4 py-3 rounded mb-6">
          <p className="font-bold">{t('importCaddy.noDomainsFound')}</p>
          <p className="text-sm mt-1">
            {t('importCaddy.emptyFileWarning')}
          </p>
        </div>
      )}

      {!session && (
        <div className="bg-dark-card rounded-lg border border-gray-800 p-6">
          <div className="mb-6">
            <h2 className="text-xl font-semibold text-white mb-2">{t('importCaddy.uploadOrPaste')}</h2>
            <p className="text-gray-400 text-sm">
              {t('importCaddy.description')}
            </p>
          </div>

          <div className="space-y-4">
            {/* File Upload */}
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">
                {t('importCaddy.uploadCaddyfile')}
              </label>
              <input
                type="file"
                accept=".caddyfile,.txt,text/plain"
                onChange={handleFileUpload}
                className="w-full text-sm text-gray-400 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-sm file:font-medium file:bg-blue-active file:text-white hover:file:bg-blue-hover file:cursor-pointer cursor-pointer"
                data-testid="import-dropzone"
              />
            </div>

            {/* Or Divider */}
            <div className="flex items-center gap-4">
              <div className="flex-1 border-t border-gray-700" />
              <span className="text-gray-500 text-sm">{t('importCaddy.orPasteContent')}</span>
              <div className="flex-1 border-t border-gray-700" />
            </div>

            {/* Text Area */}
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">
                {t('importCaddy.caddyfileContent')}
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
              {loading ? t('importCaddy.processing') : t('importCaddy.parseAndReview')}
            </button>
            <button
              onClick={() => setShowMultiModal(true)}
              className="ml-4 px-4 py-2 bg-gray-800 text-white rounded-lg"
              data-testid="multi-file-import-button"
            >
              {t('importCaddy.multiSiteImport')}
            </button>
          </div>
        </div>
      )}

      {showReview && preview && preview.preview && (
        <div data-testid="import-review-table">
          <ImportReviewTable
            hosts={preview.preview.hosts}
            conflicts={preview.preview.conflicts}
            conflictDetails={preview.conflict_details}
            errors={preview.preview.errors}
            caddyfileContent={preview.caddyfile_content}
            onCommit={handleCommit}
            onCancel={() => setShowReview(false)}
          />
        </div>
      )}

      <ImportSitesModal
        visible={showMultiModal}
        onClose={() => setShowMultiModal(false)}
        onUploaded={() => setShowReview(true)}
      />

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
