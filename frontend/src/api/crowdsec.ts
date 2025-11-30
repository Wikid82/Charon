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

export default { startCrowdsec, stopCrowdsec, statusCrowdsec, importCrowdsecConfig }
