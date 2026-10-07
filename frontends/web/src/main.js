import { create } from '@bufbuild/protobuf';
import { Code, createClient } from '@connectrpc/connect';
import { createGrpcWebTransport } from '@connectrpc/connect-web';

import {
  AccountPasswordSchema,
  AccountSharingSchema,
  AccountStatus,
  AccountVisibility,
  CapChangePolicy,
  CoreService,
  DeliveryGoal,
  GetInstanceInfoRequestSchema,
  GetJobRequestSchema,
  GetLibraryRequestSchema,
  GetPlayInfoRequestSchema,
  HeartbeatRequestSchema,
  LinkAccountRequestSchema,
  ListAccountsRequestSchema,
  LiveSearchRequestSchema,
  LoginRequestSchema,
  PasswordLoginSchema,
  PasswordSignUpSchema,
  RemoveAccountRequestSchema,
  SignUpRequestSchema,
  StartDeliveryRequestSchema,
  SubscribeRequestSchema,
  UpdateAccountRequestSchema,
} from './gen/abcmovies/api/v1/core_pb.js';
import { EventType } from './gen/abcmovies/core/v1/event_pb.js';
import {
  accountCard,
  eventCard,
  eventTypeLabel,
  enrichmentPanel,
  jobCard,
  jobStatusLabel,
  libraryCard,
  liveSearchHitCard,
  metadataPanel,
  playMenu,
  registryPanel,
  sourceCachePanel,
} from './render.js';

const $ = (id) => document.getElementById(id);

const log = (line) => {
  const el = $('log');
  el.textContent += line + '\n';
  el.scrollTop = el.scrollHeight;
};

// The session token is held in memory only: a page reload requires login
// again, which keeps the M0 client free of storage-side token hygiene.
let token = null;
let username = null;

// The accounts the caller can deliver from, last seen from ListAccounts.
let accounts = [];
// The library's current page state.
let libraryQuery = '';
let nextPageToken = '';
// The delivery session the player is currently attached to.
let activeSessionId = null;

// Instance parameters published by GetInstanceInfo (PLAN.md §9.1): the core
// tells clients how often a play session must heartbeat — never hardcode it.
// The fallback applies only if the public call itself fails.
let heartbeatIntervalMs = 30000;
let heartbeatTimer = null;

const describe = (err) => {
  const name = typeof err?.code === 'number' ? Code[err.code] : undefined;
  return `${err?.message ?? String(err)}${name ? ` [${name}]` : ''}`;
};

const emptyHint = (text) => {
  const node = document.createElement('div');
  node.className = 'empty';
  node.textContent = text;
  return node;
};

const authInterceptor = (next) => async (req) => {
  if (token) req.header.set('Authorization', `Bearer ${token}`);
  return next(req);
};

const transport = createGrpcWebTransport({
  baseUrl: window.location.origin,
  interceptors: [authInterceptor],
});
const client = createClient(CoreService, transport);

// --- Session header ---

function updateSessionUI() {
  const loggedIn = token !== null;
  const chip = $('sessionChip');
  chip.textContent = loggedIn ? `logged in as ${username}` : 'anonymous';
  chip.className = `chip ${loggedIn ? 'in' : 'out'}`;
  $('logout').classList.toggle('hidden', !loggedIn);
  for (const id of [
    'getJob',
    'subscribeToggle',
    'probeJob',
    'refreshMetadata',
    'refreshRegistry',
    'refreshSourceCache',
    'refreshEnrichment',
    'linkAccount',
    'listAccounts',
    'browse',
    'liveSearch',
  ]) {
    $(id).disabled = !loggedIn;
  }
}

// --- Provider accounts (PLAN.md §7.5) ---

$('linkVisibility').addEventListener('change', () => {
  $('linkSharedWith').classList.toggle(
    'hidden',
    $('linkVisibility').value !== 'shared',
  );
});

async function refreshAccounts() {
  try {
    const res = await client.listAccounts(
      create(ListAccountsRequestSchema, {}),
    );
    accounts = res.accounts ?? [];
    const slot = $('accountList');
    slot.innerHTML = '';
    if (accounts.length === 0) {
      slot.append(emptyHint('no accounts this user can use'));
    }
    for (const acc of accounts) {
      slot.append(
        accountCard(acc, {
          onRemove: acc.callerLinked
            ? () => removeAccount(acc.accountId)
            : undefined,
          onUpdate: acc.callerLinked
            ? (spec) => updateAccount(acc.accountId, spec)
            : undefined,
        }),
      );
    }
    log(
      `accounts: ${accounts.length} (${accounts.map((a) => a.accountId).join(', ')})`,
    );
  } catch (err) {
    log(`list accounts failed: ${describe(err)}`);
  }
}

$('listAccounts').addEventListener('click', refreshAccounts);

$('linkAccount').addEventListener('click', async () => {
  const provider = $('linkProvider').value;
  const baseUrl = $('linkServer').value.trim();
  const linkUser = $('linkUsername').value.trim();
  const linkPass = $('linkPassword').value;
  const visibilityName = $('linkVisibility').value.toUpperCase();
  const visibility = AccountVisibility[visibilityName];
  if (!baseUrl || !linkUser || !linkPass) {
    log('link account: server url, username and password are required');
    return;
  }
  try {
    const res = await client.linkAccount(
      create(LinkAccountRequestSchema, {
        provider,
        baseUrl,
        visibility,
        sharedWith:
          visibility === AccountVisibility.SHARED
            ? parseScopes($('linkSharedWith').value)
            : [],
        authMethod: {
          case: 'password',
          value: create(AccountPasswordSchema, {
            username: linkUser,
            password: new TextEncoder().encode(linkPass),
          }),
        },
      }),
    );
    log(`account linked: ${res.accountId}`);
    $('linkPassword').value = '';
    await refreshAccounts();
  } catch (err) {
    log(`link account failed: ${describe(err)}`);
  }
});

async function updateAccount(accountId, spec) {
  try {
    const sharing = spec.visibility
      ? create(AccountSharingSchema, {
          visibility: AccountVisibility[spec.visibility.toUpperCase()],
          sharedWith: spec.sharedWith,
        })
      : undefined;
    if (spec.visibility && sharing.visibility === undefined) {
      log(`update account: unknown visibility ${spec.visibility}`);
      return;
    }
    const capChangePolicy =
      spec.capChangePolicy === 'inherit'
        ? CapChangePolicy.UNSPECIFIED
        : spec.capChangePolicy === 'new-sessions-only'
          ? CapChangePolicy.NEW_SESSIONS_ONLY
          : spec.capChangePolicy === 'enforce-now'
            ? CapChangePolicy.ENFORCE_NOW
            : undefined;
    const res = await client.updateAccount(
      create(UpdateAccountRequestSchema, {
        accountId,
        sharing,
        maxConcurrentStreams: spec.maxConcurrentStreams,
        capChangePolicy,
      }),
    );
    log(
      `account updated: ${res.account?.accountId ?? accountId} (${spec.visibility}, cap ${spec.maxConcurrentStreams ?? 'unchanged'}, policy ${spec.capChangePolicy})`,
    );
    await refreshAccounts();
    await refreshLibrary();
  } catch (err) {
    log(`update account failed: ${describe(err)}`);
  }
}

async function removeAccount(accountId) {
  try {
    await client.removeAccount(
      create(RemoveAccountRequestSchema, { accountId }),
    );
    log(`account removed: ${accountId}`);
    await refreshAccounts();
    await refreshLibrary();
  } catch (err) {
    log(`remove account failed: ${describe(err)}`);
  }
}

// --- Library (PLAN.md §5, §8.1) ---

// accountForDelivery picks the account a play request should use for a
// coverage provider, preferring accounts the caller linked over operator-
// declared ones.
function accountForDelivery(provider) {
  const usable = accounts.filter(
    (a) => a.provider === provider && a.status === AccountStatus.LINKED,
  );
  return (
    usable.find((a) => a.callerLinked) ??
    usable.find((a) => !a.callerLinked) ??
    null
  );
}

// playSource derives a delivery source from a coverage key ("provider:nativeId",
// PLAN.md §5.3) when an account the caller can use covers it.
function playSource(entry) {
  const keys = Object.keys(entry.coverage ?? {});
  for (const key of keys) {
    const sep = key.indexOf(':');
    const provider = sep === -1 ? key : key.slice(0, sep);
    const nativeId = sep === -1 ? key : key.slice(sep + 1);
    if (nativeId && accountForDelivery(provider)) {
      return { provider, nativeId, key };
    }
  }
  return null;
}

async function refreshLibrary() {
  const busy = $('browse');
  busy.disabled = true;
  try {
    const res = await client.getLibrary(
      create(GetLibraryRequestSchema, { query: libraryQuery }),
    );
    nextPageToken = res.nextPageToken ?? '';
    const slot = $('libraryGrid');
    slot.innerHTML = '';
    for (const item of res.items ?? []) {
      const src = playSource(item.entry);
      slot.append(
        libraryCard(item, {
          onPlay: src ? () => startPlay(src.provider, src.nativeId) : undefined,
          payload: src,
        }),
      );
    }
    if ((res.items ?? []).length === 0) {
      slot.append(
        emptyHint(
          libraryQuery
            ? `no titles match "${libraryQuery}"`
            : 'library is empty',
        ),
      );
    }
    $('nextPage').classList.toggle('hidden', !nextPageToken);
    log(`library: ${(res.items ?? []).length} items`);
  } catch (err) {
    log(`browse library failed: ${describe(err)}`);
  } finally {
    busy.disabled = !token;
  }
}

$('browse').addEventListener('click', () => {
  libraryQuery = $('query').value.trim();
  nextPageToken = '';
  refreshLibrary();
});

$('nextPage').addEventListener('click', async () => {
  if (!nextPageToken) return;
  try {
    const res = await client.getLibrary(
      create(GetLibraryRequestSchema, {
        query: libraryQuery,
        pageToken: nextPageToken,
      }),
    );
    nextPageToken = res.nextPageToken ?? '';
    const slot = $('libraryGrid');
    slot.innerHTML = '';
    for (const item of res.items ?? []) {
      const src = playSource(item.entry);
      slot.append(
        libraryCard(item, {
          onPlay: src ? () => startPlay(src.provider, src.nativeId) : undefined,
          payload: src,
        }),
      );
    }
    $('nextPage').classList.toggle('hidden', !nextPageToken);
    log(`library: page of ${(res.items ?? []).length} more items`);
  } catch (err) {
    log(`browse library failed: ${describe(err)}`);
  }
});

// --- Live search (PLAN.md §5.4: explicit, user-triggered lazy-provider refresh) ---

$('liveSearch').addEventListener('click', async () => {
  const query = $('liveQuery').value.trim();
  if (!query) {
    log('live search: query is required');
    return;
  }
  const providers = parseScopes($('liveProviders').value);
  const rawLimit = $('livePageLimit').value.trim();
  const pageLimit = rawLimit === '' ? 0 : Number(rawLimit);
  if (rawLimit !== '' && (!Number.isInteger(pageLimit) || pageLimit < 0)) {
    log('live search: page limit must be a non-negative integer');
    return;
  }
  const busy = $('liveSearch');
  busy.disabled = true;
  try {
    const res = await client.liveSearch(
      create(LiveSearchRequestSchema, { query, providers, pageLimit }),
    );
    const results = $('liveResults');
    results.innerHTML = '';
    for (const hit of res.hits ?? []) {
      results.append(
        liveSearchHitCard(hit, {
          onPlay: () => startPlay(hit.provider, hit.nativeId),
        }),
      );
    }
    if ((res.hits ?? []).length === 0) {
      results.append(emptyHint(`no live hits for "${query}"`));
    }
    for (const [slotId, err] of Object.entries(res.rejected ?? {})) {
      log(`live search: ${slotId} rejected: ${err}`);
    }
    log(
      `live search: ${(res.hits ?? []).length} hits; results ingested into the library cache`,
    );
  } catch (err) {
    log(`live search failed: ${describe(err)}`);
  } finally {
    busy.disabled = !token;
  }
});

// --- Instance info (public; TECHNICAL-DECISIONS.md §1.43) ---

async function loadInstanceInfo() {
  try {
    const res = await client.getInstanceInfo(
      create(GetInstanceInfoRequestSchema, {}),
    );
    const d = res.heartbeatInterval;
    if (d) {
      heartbeatIntervalMs =
        Number(d.seconds) * 1000 + Math.round((d.nanos ?? 0) / 1e6);
    }
    log(
      `instance: contract ${res.contractVersion}; auth methods: ${(res.authMethods ?? []).join(', ')}; heartbeat every ${heartbeatIntervalMs / 1000}s`,
    );
  } catch (err) {
    log(
      `get instance info failed: ${describe(err)} — heartbeat stays at ${heartbeatIntervalMs / 1000}s`,
    );
  }
}

// A play session stays alive only while heartbeated (PLAN.md §9.1); a paused
// video still heartbeats. The loop stops on the first failed beat — the
// session is gone or the token is — and on logout.
function startHeartbeat(sessionId) {
  stopHeartbeat();
  heartbeatTimer = setInterval(async () => {
    try {
      await client.heartbeat(create(HeartbeatRequestSchema, { sessionId }));
    } catch (err) {
      log(`heartbeat failed: ${describe(err)} — stopping`);
      stopHeartbeat();
    }
  }, heartbeatIntervalMs);
}

function stopHeartbeat() {
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer);
    heartbeatTimer = null;
  }
}

// --- Minimal play (PLAN.md §6, §9.1: menu-ready events, get-play-info, relay) ---

async function startPlay(provider, nativeId) {
  const account = accountForDelivery(provider);
  if (!account) {
    log(`start delivery: no deliverable account for provider ${provider}`);
    return;
  }
  try {
    const res = await client.startDelivery(
      create(StartDeliveryRequestSchema, {
        goal: DeliveryGoal.PLAY,
        provider,
        accountId: account.accountId,
        nativeId,
        sink: 'device',
      }),
    );
    activeSessionId = res.job?.id;
    if (!activeSessionId) {
      log('start delivery: response carried no job id');
      return;
    }
    $('player').classList.remove('hidden');
    startHeartbeat(activeSessionId);
    log(
      `delivery started: ${activeSessionId} (${provider}:${nativeId} via ${account.accountId}) — waiting for the play menu`,
    );
    await refreshPlayInfo();
  } catch (err) {
    log(`start delivery failed: ${describe(err)}`);
  }
}

async function refreshPlayInfo() {
  if (!activeSessionId) return;
  try {
    const res = await client.getPlayInfo(
      create(GetPlayInfoRequestSchema, { sessionId: activeSessionId }),
    );
    $('playTrackList').innerHTML = '';
    $('playTrackList').append(playMenu(res));
    const video = res.tracks?.find((t) => t.media?.case === 'video');
    const hint = $('playerHint');
    if (video) {
      const url = window.location.origin + video.relayUrl;
      const elVideo = $('playVideo');
      elVideo.src = url;
      const container = res.container?.toLowerCase() ?? '';
      const native = container && container !== 'mp4' && container !== 'webm';
      hint.classList.toggle('hidden', !native);
      hint.textContent =
        'Container not natively playable in this browser (raw provider passthrough); treating the relay pull as the delivery proof.';
      log(`play menu ready: video track ${video.trackId} -> ${url}`);
    } else {
      hint.classList.add('hidden');
    }
  } catch (err) {
    log(`get play info failed: ${describe(err)}`);
  }
}

$('logout').addEventListener('click', () => {
  stopSubscription();
  stopHeartbeat();
  token = null;
  username = null;
  activeSessionId = null;
  $('player').classList.add('hidden');
  $('playVideo').removeAttribute('src');
  $('accountList').innerHTML = '';
  $('libraryGrid').innerHTML = '';
  $('nextPage').classList.add('hidden');
  $('recovery').classList.add('hidden');
  $('copyRecovery').classList.add('hidden');
  updateSessionUI();
  log('logged out; token discarded');
});

// --- Account ---

$('signup').addEventListener('click', async () => {
  const name = $('username').value;
  const password = $('password').value;
  try {
    const res = await client.signUp(
      create(SignUpRequestSchema, {
        username: name,
        authMethod: {
          case: 'password',
          value: create(PasswordSignUpSchema, {
            password: new TextEncoder().encode(password),
          }),
        },
      }),
    );
    token = res.token;
    username = name;
    showRecoveryKey(res.recoveryKey);
    log(`signed up: ${res.userId} (session active)`);
    await refreshAccounts();
    await refreshLibrary();
    updateSessionUI();
  } catch (err) {
    log(`sign up failed: ${describe(err)}`);
  }
});

function showRecoveryKey(recoveryKey) {
  const box = $('recovery');
  box.innerHTML = '';
  box.append(
    'Recovery key (shown once — store it safely): ',
    Object.assign(document.createElement('strong'), { text: recoveryKey }),
  );
  box.classList.remove('hidden');
  const copy = $('copyRecovery');
  copy.classList.remove('hidden');
  copy.textContent = 'Copy recovery key';
  copy.onclick = async () => {
    await navigator.clipboard.writeText(recoveryKey);
    copy.textContent = 'Copied!';
    setTimeout(() => {
      copy.textContent = 'Copy recovery key';
    }, 1500);
  };
}

$('login').addEventListener('click', async () => {
  username = $('username').value;
  const password = $('password').value;
  try {
    const res = await client.login(
      create(LoginRequestSchema, {
        username,
        authMethod: {
          case: 'password',
          value: create(PasswordLoginSchema, {
            password: new TextEncoder().encode(password),
          }),
        },
      }),
    );
    token = res.token;
    await refreshAccounts();
    await refreshLibrary();
    updateSessionUI();
    log('logged in; session token stored in memory');
  } catch (err) {
    username = null;
    log(`login failed: ${describe(err)}`);
  }
});

// --- Jobs ---

async function showJob(jobId) {
  try {
    const res = await client.getJob(create(GetJobRequestSchema, { jobId }));
    if (!res.job) {
      log('job not found');
      return;
    }
    const slot = $('jobCard');
    slot.innerHTML = '';
    slot.append(jobCard(res.job));
    log(
      `job ${res.job.id}: status=${jobStatusLabel(res.job.status)} kind=${res.job.kind}`,
    );
  } catch (err) {
    log(`get job failed: ${describe(err)}`);
  }
}

$('getJob').addEventListener('click', () => {
  const jobId = $('jobId').value.trim();
  if (jobId) showJob(jobId);
});

$('probeJob').addEventListener('click', async () => {
  try {
    const res = await fetch('/debug/job', {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const { job_id: jobId } = await res.json();
    log(`probe job created: ${jobId}`);
    $('jobId').value = jobId;
    await showJob(jobId);
  } catch (err) {
    log(`probe failed: ${describe(err)}`);
  }
});

// --- State (debug/observability views) ---

// loadState fetches one of the /debug/* read-only JSON endpoints and renders
// it into the named panel. These routes require the same bearer token as the
// RPC surface; a 401 simply logs and leaves the panel as-is.
async function loadState(endpoint, panelId, renderer) {
  const panel = $(panelId);
  try {
    const res = await fetch(endpoint, {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    panel.innerHTML = '';
    panel.append(renderer(data));
  } catch (err) {
    log(`${endpoint} failed: ${describe(err)}`);
  }
}

$('refreshMetadata').addEventListener('click', () =>
  loadState('/debug/metadata', 'metadataPanel', metadataPanel),
);
$('refreshRegistry').addEventListener('click', () =>
  loadState('/debug/registry', 'registryPanel', registryPanel),
);
$('refreshSourceCache').addEventListener('click', () =>
  loadState('/debug/sourcecache', 'sourceCachePanel', sourceCachePanel),
);
$('refreshEnrichment').addEventListener('click', () =>
  loadState('/debug/enrichment', 'enrichmentPanel', enrichmentPanel),
);

// --- Events ---

const feed = $('eventFeed');
let subAbort = null;

for (const [name, value] of Object.entries(EventType)) {
  if (typeof value !== 'number' || value === EventType.UNSPECIFIED) continue;
  const option = document.createElement('option');
  option.value = String(value);
  option.textContent = eventTypeLabel(value);
  $('eventFilter').append(option);
}

$('eventFilter').addEventListener('change', () => applyFilter());

function applyFilter() {
  const selected = $('eventFilter').value;
  for (const card of feed.querySelectorAll('.card.event')) {
    card.classList.toggle(
      'hidden',
      selected !== 'all' && card.dataset.type !== selected,
    );
  }
}

function addEvent(ev) {
  feed.prepend(eventCard(ev));
  while (feed.children.length > 200) feed.lastChild.remove();
  applyFilter();
}

function parseScopes(raw) {
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
}

async function startSubscription() {
  subAbort = new AbortController();
  $('subscribeToggle').textContent = 'Unsubscribe';
  try {
    for await (const res of client.subscribe(
      create(SubscribeRequestSchema, {
        scopes: parseScopes($('scopes').value),
      }),
      { signal: subAbort.signal },
    )) {
      addEvent(res.event);
      // The play menu stages asynchronously; the delivery-play-menu-ready
      // event announces the session's menu is available (§9.1). Poll once
      // when the announced job is the session the player is attached to.
      if (
        res.event?.type === EventType.DELIVERY_PLAY_MENU_READY &&
        res.event?.playMenuReady?.jobId === activeSessionId
      ) {
        refreshPlayInfo();
      }
    }
  } catch (err) {
    if (!subAbort.signal.aborted) log(`subscription failed: ${describe(err)}`);
  } finally {
    subAbort = null;
    $('subscribeToggle').textContent = 'Subscribe';
  }
}

function stopSubscription() {
  if (subAbort) subAbort.abort();
}

$('subscribeToggle').addEventListener('click', () => {
  if (subAbort) stopSubscription();
  else startSubscription();
});

$('clearFeed').addEventListener('click', () => {
  feed.innerHTML = '';
});

updateSessionUI();
loadInstanceInfo();
