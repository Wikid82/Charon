import { describe, it, expect } from 'vitest';

import { parseJsonArray } from '../parseJsonArray';

describe('parseJsonArray', () => {
  it('returns an empty array for an empty value', () => {
    expect(parseJsonArray<string>('')).toEqual([]);
  });

  it('parses a valid JSON array', () => {
    expect(parseJsonArray<{ id: number }>('[{"id":1}]')).toEqual([{ id: 1 }]);
  });

  it('returns an empty array for malformed JSON', () => {
    expect(parseJsonArray<string>('{not json')).toEqual([]);
  });
});
