import { describe, expect, it } from 'vitest'

import {
  GITHUB_BUG_REPORT_URL,
  GITHUB_FEATURE_REQUEST_URL,
  GITHUB_REPO_URL,
} from '../links'

describe('links constants', () => {
  it('builds GitHub issue URLs under the repository', () => {
    expect(GITHUB_REPO_URL).toBe('https://github.com/Wikid82/Charon')
    expect(GITHUB_BUG_REPORT_URL).toBe(`${GITHUB_REPO_URL}/issues/new?template=bug_report.md`)
    expect(GITHUB_FEATURE_REQUEST_URL).toBe(
      `${GITHUB_REPO_URL}/issues/new?template=feature_request.md`
    )
  })
})
