import { describe, it, expect } from 'vitest'

import {
  bindChoiceFromSetting,
  bindSettingFromChoice,
  isBindChoiceComplete,
  isLoopbackListener,
} from '../externalProxyBind'

describe('bindChoiceFromSetting', () => {
  it('maps the empty setting to automatic', () => {
    expect(bindChoiceFromSetting('')).toEqual({ kind: 'auto', cidr: '', custom: '' })
  })

  it('maps 0.0.0.0 to all networks', () => {
    expect(bindChoiceFromSetting('0.0.0.0')).toEqual({ kind: 'all', cidr: '', custom: '' })
  })

  it('maps a subnet to a network choice', () => {
    expect(bindChoiceFromSetting('172.18.0.0/16')).toEqual({ kind: 'network', cidr: '172.18.0.0/16', custom: '' })
  })

  it('keeps any other address as a custom choice so it is not rewritten on save', () => {
    expect(bindChoiceFromSetting('::')).toEqual({ kind: 'custom', cidr: '', custom: '::' })
    expect(bindChoiceFromSetting('10.0.0.5')).toEqual({ kind: 'custom', cidr: '', custom: '10.0.0.5' })
  })
})

describe('bindSettingFromChoice', () => {
  it('round-trips every setting shape', () => {
    for (const setting of ['', '0.0.0.0', '172.18.0.0/16', '::', '10.0.0.5']) {
      expect(bindSettingFromChoice(bindChoiceFromSetting(setting))).toBe(setting)
    }
  })

  it('stores all networks as 0.0.0.0 and automatic as the empty string', () => {
    expect(bindSettingFromChoice({ kind: 'all', cidr: '', custom: '' })).toBe('0.0.0.0')
    expect(bindSettingFromChoice({ kind: 'auto', cidr: '', custom: '' })).toBe('')
  })

  it('trims a custom address', () => {
    expect(bindSettingFromChoice({ kind: 'custom', cidr: '', custom: '  10.0.0.5 ' })).toBe('10.0.0.5')
  })
})

describe('isBindChoiceComplete', () => {
  it('requires a non-blank custom value only for the custom choice', () => {
    expect(isBindChoiceComplete({ kind: 'custom', cidr: '', custom: '   ' })).toBe(false)
    expect(isBindChoiceComplete({ kind: 'custom', cidr: '', custom: '10.0.0.5' })).toBe(true)
    expect(isBindChoiceComplete({ kind: 'auto', cidr: '', custom: '' })).toBe(true)
    expect(isBindChoiceComplete({ kind: 'network', cidr: '10.0.0.0/8', custom: '' })).toBe(true)
  })
})

describe('isLoopbackListener', () => {
  it('detects IPv4 and IPv6 loopback listeners with or without a port', () => {
    expect(isLoopbackListener('127.0.0.1:2375')).toBe(true)
    expect(isLoopbackListener('127.0.0.1')).toBe(true)
    expect(isLoopbackListener('[::1]:2375')).toBe(true)
    expect(isLoopbackListener('::1')).toBe(true)
  })

  it('treats other addresses and empty values as not loopback', () => {
    expect(isLoopbackListener('0.0.0.0:2375')).toBe(false)
    expect(isLoopbackListener('172.18.0.4:2375')).toBe(false)
    expect(isLoopbackListener('')).toBe(false)
  })
})
