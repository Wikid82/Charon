export const isValidEmail = (email: string): boolean => {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
  return emailRegex.test(email)
}

/**
 * Checks if a string is a valid IPv4 address
 */
export const isIPv4 = (value: string): boolean => {
  const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}$/
  if (!ipv4Regex.test(value)) return false
  const parts = value.split('.')
  return parts.every(part => {
    const num = parseInt(part, 10)
    return num >= 0 && num <= 255
  })
}

/**
 * Checks if an IP is in a private/Docker range that may change on restart
 */
export const isPrivateOrDockerIP = (ip: string): boolean => {
  if (!isIPv4(ip)) return false
  const parts = ip.split('.').map(p => parseInt(p, 10))

  // 10.0.0.0/8
  if (parts[0] === 10) return true

  // 172.16.0.0/12 (172.16.x.x - 172.31.x.x) - includes Docker bridge networks
  if (parts[0] === 172 && parts[1] >= 16 && parts[1] <= 31) return true

  // 192.168.0.0/16
  if (parts[0] === 192 && parts[1] === 168) return true

  return false
}

/**
 * Checks if the value looks like a raw IP that could be a Docker container IP
 */
export const isLikelyDockerContainerIP = (value: string): boolean => {
  if (!isIPv4(value)) return false
  const parts = value.split('.').map(p => parseInt(p, 10))

  // Docker default bridge: 172.17.x.x
  // Docker user-defined: 172.18.x.x - 172.31.x.x
  if (parts[0] === 172 && parts[1] >= 17 && parts[1] <= 31) return true

  return false
}
