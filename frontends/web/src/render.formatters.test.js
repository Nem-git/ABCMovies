import { describe, it, expect } from 'vitest';

import { fmtTimestamp, fmtDuration } from './render.js';

describe('fmtTimestamp', () => {
  it('renders a timestamp with seconds and nanos', () => {
    const ts = { seconds: 1700000000n, nanos: 500000000 };
    const result = fmtTimestamp(ts);
    expect(result).not.toBe('');
    expect(result).not.toContain('NaN');
  });

  it('returns empty string for falsy input', () => {
    expect(fmtTimestamp(null)).toBe('');
    expect(fmtTimestamp(undefined)).toBe('');
  });

  it('handles zero nanos gracefully', () => {
    const ts = { seconds: 0n, nanos: 0 };
    const result = fmtTimestamp(ts);
    expect(result).not.toBe('');
    expect(result).not.toContain('NaN');
  });
});

describe('fmtDuration', () => {
  it('renders hours, minutes, and seconds in compact form', () => {
    expect(fmtDuration({ seconds: 3661n })).toBe('1h1m1s');
  });

  it('returns empty string for falsy input', () => {
    expect(fmtDuration(null)).toBe('');
    expect(fmtDuration(undefined)).toBe('');
  });

  it('handles zero seconds', () => {
    expect(fmtDuration({ seconds: 0n })).toBe('0s');
  });

  it('formats values under an hour without leading hours', () => {
    expect(fmtDuration({ seconds: 65n })).toBe('1m5s');
  });

  it('formats values under a minute', () => {
    expect(fmtDuration({ seconds: 42n })).toBe('42s');
  });
});
