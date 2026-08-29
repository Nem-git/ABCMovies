import { toJsonString } from '@bufbuild/protobuf';

import {
  ActionType,
  JobKind,
  JobSchema,
  JobStatus,
} from './gen/abcmovies/core/v1/job_pb.js';
import {
  EventAudience,
  EventEnvelopeSchema,
  EventType,
} from './gen/abcmovies/core/v1/event_pb.js';

// tsEnum runtime objects carry a reverse name mapping (number -> name), so
// labels derive from the generated contracts instead of being restated here.
function enumLabel(tsEnum, prefix, value) {
  const name = tsEnum[value];
  if (name === undefined) return String(value);
  return name.slice(prefix.length).toLowerCase().replaceAll('_', '-');
}

export const jobKindLabel = (v) => enumLabel(JobKind, 'JOB_KIND_', v);
export const jobStatusLabel = (v) => enumLabel(JobStatus, 'JOB_STATUS_', v);
export const actionTypeLabel = (v) => enumLabel(ActionType, 'ACTION_TYPE_', v);
export const eventTypeLabel = (v) => enumLabel(EventType, 'EVENT_TYPE_', v);
export const audienceLabel = (v) =>
  enumLabel(EventAudience, 'EVENT_AUDIENCE_', v);

export function fmtTimestamp(ts) {
  if (!ts) return '';
  const ms = Number(ts.seconds) * 1000 + Math.floor((ts.nanos ?? 0) / 1e6);
  return new Date(ms).toLocaleString();
}

export function fmtDuration(d) {
  if (!d) return '';
  let s = Number(d.seconds);
  const h = Math.floor(s / 3600);
  s -= h * 3600;
  const m = Math.floor(s / 60);
  s -= m * 60;
  const parts = [];
  if (h) parts.push(`${h}h`);
  if (h || m) parts.push(`${m}m`);
  parts.push(`${s}s`);
  return parts.join('');
}

function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === undefined || v === null || v === false) continue;
    if (k === 'class') node.className = v;
    else if (k === 'text') node.textContent = v;
    else node.setAttribute(k, v === true ? '' : String(v));
  }
  for (const child of children.flat()) {
    if (child === undefined || child === null) continue;
    node.append(child);
  }
  return node;
}

function row(label, value) {
  if (value === undefined || value === null || value === '') return null;
  return el(
    'div',
    { class: 'row' },
    el('span', { class: 'label', text: label }),
    el('span', { text: String(value) }),
  );
}

function listRow(label, entries, format = ([k, v]) => `${k} = ${v}`) {
  if (!entries || entries.length === 0) return null;
  const list = el('ul', { class: 'kv' });
  for (const entry of entries) list.append(el('li', { text: format(entry) }));
  return el(
    'div',
    { class: 'row' },
    el('span', { class: 'label', text: label }),
    list,
  );
}

const mapEntries = (m) => Object.entries(m ?? {});

function badge(text, kind = '') {
  return el('span', { class: `badge ${kind}`.trim(), text });
}

function subSection(title, ...children) {
  return el(
    'div',
    { class: 'sub' },
    el('div', { class: 'sub-title', text: title }),
    ...children.filter(Boolean),
  );
}

export function jobCard(job) {
  const card = el('div', { class: 'card' });
  card.append(
    el(
      'div',
      { class: 'card-head' },
      badge(jobKindLabel(job.kind), 'kind'),
      badge(jobStatusLabel(job.status), `status-${job.status}`),
      el('code', { text: job.id }),
    ),
  );

  const body = el('div', { class: 'card-body' });
  for (const r of [
    row('owner', job.ownerUserId),
    row('idempotency key', job.idempotencyKey),
    row(
      'progress',
      job.progress ? `${job.progress.percent}% — ${job.progress.message}` : '',
    ),
    row('error', job.error),
  ]) {
    if (r) body.append(r);
  }

  const d = job.delivery;
  if (d) {
    body.append(
      subSection(
        'delivery context',
        row('provider', d.provider),
        row('account', d.accountId),
        row('member', d.memberUserId),
        row('sink', d.sink),
        row('target', d.selectedTarget),
        row('container', d.container),
        row('manifest ref', d.manifestRef),
        listRow('policy', mapEntries(d.policy?.limits)),
        listRow('provider cap', mapEntries(d.providerCap?.limits)),
        listRow('track markers', mapEntries(d.trackMarkers)),
      ),
    );
  }

  const aa = job.awaitingAction;
  if (aa) {
    const what = aa.passthrough
      ? `passthrough prompt from "${aa.passthrough.adapter}": ${aa.passthrough.descriptor}`
      : actionTypeLabel(aa.common);
    body.append(
      subSection(
        'awaiting action',
        row('action', what),
        row('who must act', aa.actorUserId),
      ),
    );
  }

  card.append(body);
  card.append(
    el(
      'details',
      {},
      el('summary', { text: 'raw JSON' }),
      el('pre', { text: toJsonString(JobSchema, job) }),
    ),
  );
  return card;
}

function payloadRows(ev) {
  switch (ev.type) {
    case EventType.JOB_STATUS: {
      const p = ev.jobStatus;
      return [row('job', p?.jobId), row('status', jobStatusLabel(p?.status))];
    }
    case EventType.MERGE_CONFLICT: {
      const p = ev.mergeConflict;
      return [
        row('provider', p?.provider),
        row('provider item', p?.providerId),
        row('entry', p?.entryId),
        row('reason', p?.reason),
      ];
    }
    case EventType.PROVIDER_SWITCHED: {
      const p = ev.providerSwitched;
      return [
        row('from provider', p?.fromProvider),
        row('to provider', p?.toProvider),
        row('reason', p?.reason),
        row('resume point', fmtDuration(p?.resumePoint)),
        row('job', p?.jobId),
      ];
    }
    case EventType.ACCOUNT_SESSION_LINKED:
    case EventType.ACCOUNT_SESSION_EXPIRED:
    case EventType.ACCOUNT_SESSION_REVOKED: {
      const p = ev.accountSession;
      return [row('account', p?.accountId), row('provider', p?.provider)];
    }
    case EventType.AVAILABILITY_CHANGED: {
      const p = ev.availability;
      return [
        row('account', p?.accountId),
        row('provider', p?.provider),
        row('entry', p?.entryId),
        row('present', p ? String(p.present) : ''),
      ];
    }
    default:
      return [row('payload', 'unknown event type')];
  }
}

export function eventCard(ev) {
  const card = el('div', { class: 'card event', 'data-type': ev.type });
  card.append(
    el(
      'div',
      { class: 'card-head' },
      badge(eventTypeLabel(ev.type), `evt-${ev.type}`),
      badge(`audience: ${audienceLabel(ev.audience)}`, 'aud'),
      el('span', { class: 'when', text: fmtTimestamp(ev.emittedAt) }),
    ),
  );

  const body = el('div', { class: 'card-body' });
  for (const r of [
    ...payloadRows(ev),
    row('routed to user', ev.userId),
    row('routed to account', ev.accountId),
    listRow(
      'scopes',
      (ev.scopes ?? []).map((s, i) => [`scope ${i + 1}`, s]),
    ),
  ]) {
    if (r) body.append(r);
  }
  card.append(body);

  card.append(
    el(
      'details',
      {},
      el('summary', { text: 'raw JSON' }),
      el('pre', { text: toJsonString(EventEnvelopeSchema, ev) }),
    ),
  );
  return card;
}

// --- State panels (debug/observability views) ---
//
// These render the JSON served by the frontend's own /debug/* read-only
// endpoints. The data is a plain-Go projection (no generated protobuf), so
// it is styled with the same row/badge helpers but has no schema to import.

const kindLabel = (k) => (typeof k === 'number' ? `kind ${k}` : String(k));

function cap(n) {
  const s = String(n);
  return s.length > 120 ? s.slice(0, 117) + '…' : s;
}

function claimsList(claims = []) {
  if (claims.length === 0) return null;
  const list = el('ul', { class: 'kv' });
  for (const c of claims) {
    list.append(
      el('li', {
        text: `${c.Namespace}:${c.Value} (${(c.Suppliers ?? []).join(', ') || 'no suppliers'})`,
      }),
    );
  }
  return list;
}

export function metadataPanel(data) {
  const body = el('div', {});
  const records = data?.records ?? [];
  const aliases = data?.aliases ?? {};
  body.append(row('records', `${records.length}`));
  body.append(
    listRow(
      'refs',
      records.map((r, i) => [`record ${i + 1}`, cap(r)]),
    ),
  );
  const aliasKeys = Object.keys(aliases);
  body.append(
    listRow(
      'aliases',
      aliasKeys.map((k) => [cap(k), cap(aliases[k])]),
    ),
  );
  if (records.length === 0 && aliasKeys.length === 0) {
    body.append(el('div', { class: 'empty', text: 'no metadata cached yet' }));
  }
  return body;
}

export function registryPanel(data) {
  const body = el('div', {});
  const entries = data?.entries ?? [];
  const mappings = data?.mappings ?? [];
  const conflicts = data?.conflicts ?? [];

  body.append(
    el('div', { class: 'sub-title', text: `entries (${entries.length})` }),
  );
  if (entries.length === 0) {
    body.append(el('div', { class: 'empty', text: 'no entries underway' }));
  }
  for (const e of entries) {
    const head = el('div', { class: 'row' }, el('code', { text: cap(e.ID) }));
    head.append(
      el('span', {
        text: `${e.Title} (${e.Year}) — ${e.Kind}`,
        class: 'label',
      }),
    );
    body.append(head);
    body.append(claimsList(e.Claims));
  }

  body.append(
    el('div', { class: 'sub-title', text: `mappings (${mappings.length})` }),
  );
  if (mappings.length === 0) {
    body.append(
      el('div', { class: 'empty', text: 'no provider mappings yet' }),
    );
  }
  const mList = el('ul', { class: 'kv' });
  for (const m of mappings) {
    mList.append(
      el('li', {
        text: `${m.Provider}/${m.NativeID} → ${cap(m.EntryID)} (gen ${m.Generation})`,
      }),
    );
  }
  if (mappings.length) body.append(mList);

  body.append(
    el('div', { class: 'sub-title', text: `conflicts (${conflicts.length})` }),
  );
  if (conflicts.length === 0) {
    body.append(
      el('div', { class: 'empty', text: 'no merge conflicts recorded' }),
    );
  }
  const cList = el('ul', { class: 'kv' });
  for (const c of conflicts) {
    cList.append(
      el('li', {
        text: `${c.Provider}/${c.NativeID} left ${cap(c.EntryID)} — ${c.Reason}`,
      }),
    );
  }
  if (conflicts.length) body.append(cList);
  return body;
}

export function sourceCachePanel(data) {
  const body = el('div', {});
  const accounts = data?.accounts ?? [];
  if (accounts.length === 0) {
    body.append(
      el('div', { class: 'empty', text: 'no provider accounts linked' }),
    );
  }
  const list = el('ul', { class: 'kv' });
  for (const a of accounts) {
    list.append(
      el('li', {
        text: `${a.provider}/${a.accountId} — ${a.items} items cached`,
      }),
    );
  }
  if (accounts.length) body.append(list);
  body.append(row('linked accounts', `${accounts.length}`));
  return body;
}

export function enrichmentPanel(data) {
  const body = el('div', {});
  const pending = data?.pending ?? [];
  const catalogues = data?.catalogues ?? [];
  body.append(row('pending items', `${pending.length}`));
  body.append(
    listRow(
      'pending',
      pending.map((p, i) => [`queue ${i + 1}`, cap(p)]),
    ),
  );
  body.append(
    listRow(
      'catalogue slots',
      catalogues.map((c, i) => [`slot ${i + 1}`, cap(c)]),
    ),
  );
  body.append(row('cached records', data?.recordCount ?? 0));
  if (pending.length === 0 && catalogues.length === 0) {
    body.append(
      el('div', {
        class: 'empty',
        text: 'queue idle; no catalogue slots enabled',
      }),
    );
  }
  return body;
}
