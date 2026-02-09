import {
  isValidEmail,
  isIPv4,
  isPrivateOrDockerIP,
  isLikelyDockerContainerIP,
} from '../validation'

describe('validation utils', () => {
  describe('isValidEmail', () => {
    it('returns true for valid emails', () => {
      expect(isValidEmail('test@example.com')).toBe(true)
      expect(isValidEmail('user.name@domain.co.uk')).toBe(true)
      expect(isValidEmail('user+regex@domain.com')).toBe(true)
    })

    it('returns false for invalid emails', () => {
      expect(isValidEmail('invalid')).toBe(false)
      expect(isValidEmail('invalid@')).toBe(false)
      expect(isValidEmail('@domain.com')).toBe(false)
      expect(isValidEmail('user@domain')).toBe(false)
      expect(isValidEmail('')).toBe(false)
    })
  })

  describe('isIPv4', () => {
    it('returns true for valid IPv4 addresses', () => {
      expect(isIPv4('192.168.1.1')).toBe(true)
      expect(isIPv4('10.0.0.1')).toBe(true)
      expect(isIPv4('0.0.0.0')).toBe(true)
      expect(isIPv4('255.255.255.255')).toBe(true)
    })

    it('returns false for invalid IPv4 addresses', () => {
      expect(isIPv4('256.0.0.1')).toBe(false)
      expect(isIPv4('1.2.3')).toBe(false)
      expect(isIPv4('1.2.3.4.5')).toBe(false)
      expect(isIPv4('1.2.3.4.')).toBe(false)
      expect(isIPv4('abc')).toBe(false)
      expect(isIPv4('192.168.1.a')).toBe(false)
      expect(isIPv4('')).toBe(false)
    })
  })

  describe('isPrivateOrDockerIP', () => {
    it('returns true for private IP ranges', () => {
      expect(isPrivateOrDockerIP('10.0.0.1')).toBe(true)
      expect(isPrivateOrDockerIP('10.255.255.255')).toBe(true)
      expect(isPrivateOrDockerIP('192.168.0.1')).toBe(true)
      expect(isPrivateOrDockerIP('192.168.255.255')).toBe(true)
      expect(isPrivateOrDockerIP('172.16.0.1')).toBe(true) // Start of 172.16.x.x
      expect(isPrivateOrDockerIP('172.31.255.255')).toBe(true) // End of 172.31.x.x
    })

    it('returns false for public or non-private IP ranges', () => {
      expect(isPrivateOrDockerIP('8.8.8.8')).toBe(false)
      expect(isPrivateOrDockerIP('1.1.1.1')).toBe(false)
      expect(isPrivateOrDockerIP('172.15.0.1')).toBe(false) // Below 172.16...
      expect(isPrivateOrDockerIP('172.32.0.1')).toBe(false) // Above 172.31...
      expect(isPrivateOrDockerIP('192.167.1.1')).toBe(false)
      expect(isPrivateOrDockerIP('192.169.1.1')).toBe(false)
    })

    it('returns false for invalid IPs', () => {
      expect(isPrivateOrDockerIP('invalid')).toBe(false)
      expect(isPrivateOrDockerIP('999.999.999.999')).toBe(false)
    })
  })

  describe('isLikelyDockerContainerIP', () => {
    it('returns true for likely Docker IPs', () => {
      // Docker default bridge: 172.17.x.x
      expect(isLikelyDockerContainerIP('172.17.0.2')).toBe(true)
      // Docker user-defined: 172.18.x.x - 172.31.x.x
      expect(isLikelyDockerContainerIP('172.18.0.1')).toBe(true)
      expect(isLikelyDockerContainerIP('172.31.255.255')).toBe(true)
    })

    it('returns false for non-Docker IPs', () => {
      expect(isLikelyDockerContainerIP('172.16.0.1')).toBe(false) // Private but often not Docker default
      expect(isLikelyDockerContainerIP('192.168.1.1')).toBe(false)
      expect(isLikelyDockerContainerIP('10.0.0.1')).toBe(false)
      expect(isLikelyDockerContainerIP('8.8.8.8')).toBe(false)
    })

    it('returns false for invalid IPs', () => {
      expect(isLikelyDockerContainerIP('invalid')).toBe(false)
    })
  })
})
