import { describe, it, expect } from 'vitest'

import { urlBase64ToUint8Array } from '../webpush'

describe('urlBase64ToUint8Array', () => {
  it('decodes a base64url string with no padding required', () => {
    // 'Zm9vYmFy' -> "foobar" in standard base64, already a multiple of 4.
    const result = urlBase64ToUint8Array('Zm9vYmFy')
    expect(Array.from(result)).toEqual([102, 111, 111, 98, 97, 114])
  })

  it('decodes a base64url string that requires padding', () => {
    // 'Zm9v' decodes to "foo" (3 bytes) without needing padding, so use a
    // length that actually requires the base64 padding branch.
    const result = urlBase64ToUint8Array('Zm9vYg')
    expect(Array.from(result)).toEqual([102, 111, 111, 98])
  })

  it('replaces URL-safe characters (- and _) before decoding', () => {
    // Bytes 0xFB 0xFF encode to base64 "-_8=" -> base64url "-_8".
    const result = urlBase64ToUint8Array('-_8')
    expect(Array.from(result)).toEqual([0xfb, 0xff])
  })

  it('decodes an empty string to an empty array', () => {
    const result = urlBase64ToUint8Array('')
    expect(result.length).toBe(0)
  })

  it('round-trips a realistic VAPID-key-length value', () => {
    // A 65-byte uncompressed P-256 public key, base64url-encoded (no padding),
    // is the real-world shape this helper receives from the backend.
    const bytes = Array.from({ length: 65 }, (_, i) => i % 256)
    const base64url = btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
    const result = urlBase64ToUint8Array(base64url)
    expect(Array.from(result)).toEqual(bytes)
  })
})
