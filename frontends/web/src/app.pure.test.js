import { describe, it, expect } from 'vitest';

import { parseScopes, accountForDelivery, playSource } from './app.js';
import { AccountStatus } from './gen/abcmovies/api/v1/core_pb.js';

describe('parseScopes', () => {
  it('splits comma-separated scopes and trims whitespace', () => {
    expect(parseScopes('user:alice, account:netflix')).toEqual([
      'user:alice',
      'account:netflix',
    ]);
  });

  it('filters out empty entries', () => {
    expect(parseScopes('a,,b, ,c')).toEqual(['a', 'b', 'c']);
  });

  it('returns empty array for empty string', () => {
    expect(parseScopes('')).toEqual([]);
  });

  it('returns empty array for whitespace-only string', () => {
    expect(parseScopes('   ')).toEqual([]);
  });
});

describe('accountForDelivery', () => {
  const linked = (overrides = {}) => ({
    accountId: 'acct-1',
    provider: 'jellyfin',
    status: AccountStatus.LINKED,
    callerLinked: true,
    ...overrides,
  });

  it('returns null when no accounts match the provider', () => {
    expect(accountForDelivery([linked()], 'stremio')).toBeNull();
  });

  it('returns null when the account is not linked', () => {
    expect(
      accountForDelivery(
        [linked({ status: AccountStatus.EXPIRED })],
        'jellyfin',
      ),
    ).toBeNull();
  });

  it('returns the matching linked account', () => {
    const acct = linked();
    expect(accountForDelivery([acct], 'jellyfin')).toBe(acct);
  });

  it('prefers caller-linked accounts over operator-declared ones', () => {
    const operatorAcct = linked({ accountId: 'op-1', callerLinked: false });
    const callerAcct = linked({ accountId: 'caller-1', callerLinked: true });
    expect(accountForDelivery([operatorAcct, callerAcct], 'jellyfin')).toBe(
      callerAcct,
    );
  });

  it('falls back to operator-declared when no caller-linked exists', () => {
    const operatorAcct = linked({ accountId: 'op-1', callerLinked: false });
    expect(accountForDelivery([operatorAcct], 'jellyfin')).toBe(operatorAcct);
  });

  it('returns null for empty accounts array', () => {
    expect(accountForDelivery([], 'jellyfin')).toBeNull();
  });
});

describe('playSource', () => {
  const entryWithCoverage = (coverage) => ({ coverage });

  it('returns null when entry has no coverage', () => {
    expect(playSource({}, [])).toBeNull();
  });

  it('returns null when coverage map is empty', () => {
    expect(playSource(entryWithCoverage({}), [])).toBeNull();
  });

  it('returns null when no account covers the provider', () => {
    const entry = entryWithCoverage({ 'jellyfin:item-1': { present: true } });
    expect(playSource(entry, [])).toBeNull();
  });

  it('returns provider and nativeId when an account covers it', () => {
    const acct = {
      accountId: 'a1',
      provider: 'jellyfin',
      status: AccountStatus.LINKED,
      callerLinked: true,
    };
    const entry = entryWithCoverage({ 'jellyfin:item-1': { present: true } });
    const src = playSource(entry, [acct]);
    expect(src).toEqual({
      provider: 'jellyfin',
      nativeId: 'item-1',
      key: 'jellyfin:item-1',
    });
  });

  it('does not match when the no-colon key differs from the provider', () => {
    const acct = {
      accountId: 'a1',
      provider: 'jellyfin',
      status: AccountStatus.LINKED,
      callerLinked: true,
    };
    const entry = entryWithCoverage({ unknown: { present: true } });
    expect(playSource(entry, [acct])).toBeNull();
  });

  it('returns the first matching coverage key', () => {
    const acct = {
      accountId: 'a1',
      provider: 'jellyfin',
      status: AccountStatus.LINKED,
      callerLinked: true,
    };
    const entry = entryWithCoverage({
      'stremio:item-2': { present: true },
      'jellyfin:item-1': { present: true },
    });
    const src = playSource(entry, [acct]);
    expect(src.provider).toBe('jellyfin');
  });
});
