import type { ProxyHost } from '../api/proxyHosts'

type SortColumn = 'name' | 'domain' | 'forward'
type SortDirection = 'asc' | 'desc'

export function compareHosts(a: ProxyHost, b: ProxyHost, sortColumn: SortColumn, sortDirection: SortDirection) {
  let aVal: string
  let bVal: string

  switch (sortColumn) {
    case 'name':
      aVal = (a.name || a.domain_names.split(',')[0] || '').toLowerCase()
      bVal = (b.name || b.domain_names.split(',')[0] || '').toLowerCase()
      break
    case 'domain':
      aVal = (a.domain_names.split(',')[0] || '').toLowerCase()
      bVal = (b.domain_names.split(',')[0] || '').toLowerCase()
      break
    case 'forward':
      aVal = `${a.forward_host}:${a.forward_port}`.toLowerCase()
      bVal = `${b.forward_host}:${b.forward_port}`.toLowerCase()
      break
    default:
      return 0
  }

  if (aVal < bVal) return sortDirection === 'asc' ? -1 : 1
  if (aVal > bVal) return sortDirection === 'asc' ? 1 : -1
  return 0
}

export default compareHosts
