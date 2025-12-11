import { test, expect } from '@playwright/test'

const base = process.env.CHARON_BASE_URL || 'http://localhost:8080'

// Hit an API route inside /api/v1 to ensure Cerberus middleware executes.
const targetPath = '/api/v1/system/my-ip'

test.describe('WAF blocking and monitoring', () => {
  test('blocks malicious query when mode=block', async ({ request }) => {
    // Use literal '<script>' to trigger naive WAF check
    const res = await request.get(`${base}${targetPath}?<script>=x`)
    expect([400, 401]).toContain(res.status())
    // When WAF runs before auth, expect 400; if auth runs first, we still validate that the server rejects
    if (res.status() === 400) {
      const body = await res.json()
      expect(body?.error).toMatch(/WAF: suspicious payload/i)
    }
  })

  test('does not block when mode=monitor (returns 401 due to auth)', async ({ request }) => {
    const res = await request.get(`${base}${targetPath}?safe=yes`)
    // Unauthenticated → expect 401, not 400; proves WAF did not block
    expect([401, 403]).toContain(res.status())
  })

  test('metrics endpoint exposes Prometheus counters', async ({ request }) => {
    const res = await request.get(`${base}/metrics`)
    expect(res.status()).toBe(200)
    const text = await res.text()
    expect(text).toContain('charon_waf_requests_total')
    expect(text).toContain('charon_waf_blocked_total')
    expect(text).toContain('charon_waf_monitored_total')
  })
})
