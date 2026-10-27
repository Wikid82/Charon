import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { SecurityEnforcementNote } from '../SecurityEnforcementNote'

describe('SecurityEnforcementNote', () => {
  it('renders nothing without a state', () => {
    const { container } = render(<SecurityEnforcementNote moduleName="waf" />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing when enforced is omitted', () => {
    const { container } = render(<SecurityEnforcementNote moduleName="waf" state={{ reason: 'no_ruleset' }} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing when enforced is true', () => {
    const { container } = render(<SecurityEnforcementNote moduleName="waf" state={{ enforced: true }} />)
    expect(container).toBeEmptyDOMElement()
  })

  it.each([
    ['waf', 'no_ruleset', 'The WAF is on, but no rule set is available to inspect requests. Add a rule set on the WAF page.'],
    ['waf', 'ruleset_not_selected', 'The WAF is on, but several rule sets exist and none is selected. Choose the rule set on the WAF page.'],
    ['waf', 'ruleset_unavailable', 'The WAF is on, but the selected rule set could not be loaded. Check it on the WAF page.'],
    ['rate_limit', 'thresholds_unset', 'Rate limiting is on, but no limits are set yet. Set the requests and window on the Rate Limiting page.'],
  ] as const)('renders the %s line for reason %s', (moduleName, reason, text) => {
    render(<SecurityEnforcementNote moduleName={moduleName} state={{ enforced: false, reason }} />)
    expect(screen.getByTestId(`${moduleName}-enforcement-note`)).toHaveTextContent(text)
  })

  it('renders the generic line for an unknown reason', () => {
    render(<SecurityEnforcementNote moduleName="waf" state={{ enforced: false, reason: 'something_new' }} />)
    expect(screen.getByText('Not applied to every host yet. Check this module\'s settings page.')).toBeInTheDocument()
  })

  it('renders the generic line when enforced is false without a reason', () => {
    render(<SecurityEnforcementNote moduleName="rate_limit" state={{ enforced: false }} />)
    expect(screen.getByTestId('rate_limit-enforcement-note')).toHaveTextContent('Not applied to every host yet.')
  })

  it('renders the generic line when a reason belongs to the other module', () => {
    render(<SecurityEnforcementNote moduleName="waf" state={{ enforced: false, reason: 'thresholds_unset' }} />)
    expect(screen.getByTestId('waf-enforcement-note')).toHaveTextContent('Not applied to every host yet.')
  })
})
