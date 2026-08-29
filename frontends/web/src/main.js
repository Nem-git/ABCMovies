import { create } from '@bufbuild/protobuf';
import { Code, createClient } from '@connectrpc/connect';
import { createGrpcWebTransport } from '@connectrpc/connect-web';

import {
  CoreService,
  GetJobRequestSchema,
  LoginRequestSchema,
  PasswordLoginSchema,
  PasswordSignUpSchema,
  SignUpRequestSchema,
  SubscribeRequestSchema,
} from './gen/abcmovies/api/v1/core_pb.js';
import { EventType } from './gen/abcmovies/core/v1/event_pb.js';
import {
  eventCard,
  eventTypeLabel,
  enrichmentPanel,
  jobCard,
  jobStatusLabel,
  metadataPanel,
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

const describe = (err) => {
  const name = typeof err?.code === 'number' ? Code[err.code] : undefined;
  return `${err?.message ?? String(err)}${name ? ` [${name}]` : ''}`;
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
  ]) {
    $(id).disabled = !loggedIn;
  }
}

$('logout').addEventListener('click', () => {
  stopSubscription();
  token = null;
  username = null;
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
    showRecoveryKey(res.recoveryKey);
    log(`signed up: ${res.userId}`);
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
  if (typeof value !== 'number' || value === EventType.EVENT_TYPE_UNSPECIFIED)
    continue;
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
