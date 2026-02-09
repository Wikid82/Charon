export const buildCrowdsecExportFilename = (): string => {
  const timestamp = new Date().toISOString().replace(/:/g, '-')
  return `crowdsec-export-${timestamp}.tar.gz`
}

export const promptCrowdsecFilename = (defaultName = buildCrowdsecExportFilename()): string | null => {
  const input = window.prompt('Name your CrowdSec export archive', defaultName)
  if (input === null || typeof input === 'undefined') return null
  const trimmed = typeof input === 'string' ? input.trim() : ''
  const candidate = trimmed || defaultName
  const sanitized = candidate.replace(/[\\/]+/g, '-').replace(/\s+/g, '-')
  return sanitized.toLowerCase().endsWith('.tar.gz') ? sanitized : `${sanitized}.tar.gz`
}

export const downloadCrowdsecExport = (blob: Blob, filename: string) => {
  const url = window.URL.createObjectURL(new Blob([blob]))
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  window.URL.revokeObjectURL(url)
}
