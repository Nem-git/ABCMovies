import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';

import {
  EventEnvelopeSchema,
  EventAudience,
  EventType,
} from './gen/abcmovies/core/v1/event_pb.js';
import { JobStatus } from './gen/abcmovies/core/v1/job_pb.js';
import { eventPayload } from './render.js';

function envelope(type, payloadCase, payloadValue) {
  return create(EventEnvelopeSchema, {
    id: 'evt-1',
    type,
    audience: EventAudience.EVENT_AUDIENCE_ACCOUNT,
    payload: { case: payloadCase, value: payloadValue },
  });
}

describe('eventPayload (oneof decode)', () => {
  it('extracts job-status payload', () => {
    const env = envelope(EventType.EVENT_TYPE_JOB_STATUS, 'jobStatus', {
      jobId: 'job-9',
      status: JobStatus.JOB_STATUS_RUNNING,
    });
    const payload = eventPayload(env, 'jobStatus');
    expect(payload).toBeDefined();
    expect(payload.jobId).toBe('job-9');
  });

  it('extracts availability-changed payload', () => {
    const env = envelope(EventType.EVENT_TYPE_AVAILABILITY_CHANGED, 'availability', {
      accountId: 'acct-1',
      provider: 'jellyfin',
      entryId: 'entry-42',
      present: true,
    });
    const payload = eventPayload(env, 'availability');
    expect(payload).toBeDefined();
    expect(payload.entryId).toBe('entry-42');
    expect(payload.present).toBe(true);
  });

  it('extracts merge-conflict payload', () => {
    const env = envelope(EventType.EVENT_TYPE_MERGE_CONFLICT, 'mergeConflict', {
      provider: 'jellyfin',
      providerId: 'item-7',
      entryId: 'le_x',
      reason: 'contradictory identity proof',
    });
    const payload = eventPayload(env, 'mergeConflict');
    expect(payload).toBeDefined();
    expect(payload.providerId).toBe('item-7');
  });

  it('extracts provider-switched payload', () => {
    const env = envelope(EventType.EVENT_TYPE_PROVIDER_SWITCHED, 'providerSwitched', {
      fromProvider: 'providerA',
      toProvider: 'providerB',
      reason: 'dead entry',
      jobId: 'job-1',
    });
    const payload = eventPayload(env, 'providerSwitched');
    expect(payload).toBeDefined();
    expect(payload.toProvider).toBe('providerB');
  });

  it('extracts account-session payload for linked', () => {
    const env = envelope(EventType.EVENT_TYPE_ACCOUNT_SESSION_LINKED, 'accountSession', {
      accountId: 'acct-9',
      provider: 'stremio',
    });
    const payload = eventPayload(env, 'accountSession');
    expect(payload).toBeDefined();
    expect(payload.accountId).toBe('acct-9');
  });

  it('extracts account-session payload for expired', () => {
    const env = envelope(EventType.EVENT_TYPE_ACCOUNT_SESSION_EXPIRED, 'accountSession', {
      accountId: 'acct-3',
      provider: 'jellyfin',
    });
    const payload = eventPayload(env, 'accountSession');
    expect(payload).toBeDefined();
    expect(payload.accountId).toBe('acct-3');
  });

  it('extracts play-menu-ready payload', () => {
    const env = envelope(EventType.EVENT_TYPE_DELIVERY_PLAY_MENU_READY, 'playMenuReady', {
      jobId: 'job-42',
    });
    const payload = eventPayload(env, 'playMenuReady');
    expect(payload).toBeDefined();
    expect(payload.jobId).toBe('job-42');
  });

  it('extracts slot-health-transition payload', () => {
    const env = envelope(EventType.EVENT_TYPE_SLOT_HEALTH_TRANSITION, 'slotHealthTransition', {
      slotId: 'tmdb',
      healthy: 0,
      wasHealthy: 1,
    });
    const payload = eventPayload(env, 'slotHealthTransition');
    expect(payload).toBeDefined();
    expect(payload.slotId).toBe('tmdb');
  });

  it('returns undefined when case does not match', () => {
    const env = envelope(EventType.EVENT_TYPE_JOB_STATUS, 'jobStatus', {
      jobId: 'job-9',
      status: JobStatus.JOB_STATUS_RUNNING,
    });
    expect(eventPayload(env, 'mergeConflict')).toBeUndefined();
  });

  it('returns undefined for malformed payload', () => {
    const env = create(EventEnvelopeSchema, {
      id: 'evt-x',
      type: EventType.EVENT_TYPE_JOB_STATUS,
    });
    expect(eventPayload(env, 'jobStatus')).toBeUndefined();
  });
});
