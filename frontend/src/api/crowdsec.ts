import client from './client'

export async function startCrowdsec() {
  const resp = await client.post('/admin/crowdsec/start')
  return resp.data
}

export async function stopCrowdsec() {
  const resp = await client.post('/admin/crowdsec/stop')
  return resp.data
}

export async function statusCrowdsec() {
  const resp = await client.get('/admin/crowdsec/status')
  return resp.data
}

export async function importCrowdsecConfig(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  const resp = await client.post('/admin/crowdsec/import', fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return resp.data
}

export async function exportCrowdsecConfig() {
  const resp = await client.get('/admin/crowdsec/export', { responseType: 'blob' })
  return resp.data
}

export async function listCrowdsecFiles() {
  const resp = await client.get<{ files: string[] }>('/admin/crowdsec/files')
  return resp.data
}

export async function readCrowdsecFile(path: string) {
  const resp = await client.get<{ content: string }>(`/admin/crowdsec/file?path=${encodeURIComponent(path)}`)
  return resp.data
}

export async function writeCrowdsecFile(path: string, content: string) {
  const resp = await client.post('/admin/crowdsec/file', { path, content })
  return resp.data
}

export default { startCrowdsec, stopCrowdsec, statusCrowdsec, importCrowdsecConfig, exportCrowdsecConfig, listCrowdsecFiles, readCrowdsecFile, writeCrowdsecFile }
