import { describe, it, expect } from 'vitest';

import { JobKind, JobStatus } from './gen/abcmovies/core/v1/job_pb.js';
import { EventAudience, EventType } from './gen/abcmovies/core/v1/event_pb.js';
import {
  jobKindLabel,
  jobStatusLabel,
  eventTypeLabel,
  audienceLabel,
} from './render.js';

describe('label helpers derive from generated contracts', () => {
  it('jobKindLabel renders known kinds in kebab-case', () => {
    expect(jobKindLabel(JobKind.REFRESH)).toBe('refresh');
    expect(jobKindLabel(JobKind.DELIVERY)).toBe('delivery');
    expect(jobKindLabel(JobKind.RE_AUTH)).toBe('re-auth');
  });

  it('jobStatusLabel renders known statuses', () => {
    expect(jobStatusLabel(JobStatus.QUEUED)).toBe('queued');
    expect(jobStatusLabel(JobStatus.RUNNING)).toBe('running');
    expect(jobStatusLabel(JobStatus.DONE)).toBe('done');
    expect(jobStatusLabel(JobStatus.FAILED)).toBe('failed');
  });

  it('eventTypeLabel renders known event types', () => {
    expect(eventTypeLabel(EventType.JOB_STATUS)).toBe('job-status');
    expect(eventTypeLabel(EventType.AVAILABILITY_CHANGED)).toBe(
      'availability-changed',
    );
    expect(eventTypeLabel(EventType.MERGE_CONFLICT)).toBe('merge-conflict');
  });

  it('audienceLabel renders known audiences', () => {
    expect(audienceLabel(EventAudience.USER)).toBe('user');
    expect(audienceLabel(EventAudience.OWNER)).toBe('owner');
    expect(audienceLabel(EventAudience.ACCOUNT)).toBe('account');
    expect(audienceLabel(EventAudience.INSTANCE)).toBe('instance');
  });

  it('unknown enum values fall back to their numeric string', () => {
    expect(jobKindLabel(999)).toBe('999');
    expect(jobStatusLabel(999)).toBe('999');
    expect(eventTypeLabel(999)).toBe('999');
    expect(audienceLabel(999)).toBe('999');
  });
});
