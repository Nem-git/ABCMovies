import { create } from '@bufbuild/protobuf';

import {
  AccountPasswordSchema,
  AccountSharingSchema,
  AccountStatus,
  AccountVisibility,
  CapChangePolicy,
  DeliveryGoal,
  GetInstanceInfoRequestSchema,
  GetJobRequestSchema,
  GetLibraryRequestSchema,
  GetPlayInfoRequestSchema,
  GetDeliveryOptionsRequestSchema,
  HeartbeatRequestSchema,
  LinkAccountRequestSchema,
  ListAccountsRequestSchema,
  ListJobsRequestSchema,
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
  deliveryDialog,
  downloadsPanel,
  eventCard,
  eventPayload,
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

// ---------------------------------------------------------------------------
// Pure functions (exported for testing)
// ---------------------------------------------------------------------------

export function parseScopes(raw) {
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
}

export function accountForDelivery(accounts, provider) {
  const usable = accounts.filter(
    (a) => a.provider === provider && a.status === AccountStatus.LINKED,
  );
  return (
    usable.find((a) => a.callerLinked) ??
    usable.find((a) => !a.callerLinked) ??
    null
  );
}

export function playSource(entry, accounts) {
  const keys = Object.keys(entry.coverage ?? {});
  for (const key of keys) {
    const sep = key.indexOf(':');
    const provider = sep === -1 ? key : key.slice(0, sep);
    const nativeId = sep === -1 ? key : key.slice(sep + 1);
    if (nativeId && accountForDelivery(accounts, provider)) {
      return { provider, nativeId, key };
    }
  }
  return null;
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

export function createApp({ client, doc, onTokenChange }) {
  const $ = (id) => doc.getElementById(id);

  const describe = (err) => {
    const name = typeof err?.code === 'number' ? err.code : undefined;
    return `${err?.message ?? String(err)}${name !== undefined ? ` [${name}]` : ''}`;
  };

  const emptyHint = (text) => {
    const node = doc.createElement('div');
    node.className = 'empty';
    node.textContent = text;
    return node;
  };

  // State
  let token = null;
  let username = null;
  let accounts = [];
  let libraryQuery = '';
  let nextPageToken = '';
  let activeSessionId = null;
  let heartbeatIntervalMs = 30000;
  let heartbeatTimer = null;
  let currentDeliveryOptions = null;
  let subAbort = null;

  const log = (line) => {
    const el = $('log');
    el.textContent += line + '\n';
    el.scrollTop = el.scrollHeight;
  };

  // --- Session header ---

  function setToken(newToken) {
    token = newToken;
    if (onTokenChange) onTokenChange(newToken);
  }

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
      'refreshDownloads',
    ]) {
      $(id).disabled = !loggedIn;
    }
    $('downloads').classList.toggle('hidden', !loggedIn);
  }

  // --- Provider accounts ---

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

  // --- Library ---

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
        const src = playSource(item.entry, accounts);
        slot.append(
          libraryCard(item, {
            onPlay: src ? () => startPlay(src.provider, src.nativeId) : undefined,
            onDownload: src
              ? () => startDownload(item.entry.id, src.provider, src.nativeId)
              : undefined,
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

  // --- Live search ---

  async function liveSearch() {
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
  }

  // --- Instance info ---

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

  // --- Heartbeat ---

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

  // --- Play ---

  async function startPlay(provider, nativeId) {
    const account = accountForDelivery(accounts, provider);
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

  // --- Download ---

  async function startDownload(entryId, provider, nativeId) {
    const account = accountForDelivery(accounts, provider);
    if (!account) {
      log(`start download: no deliverable account for provider ${provider}`);
      return;
    }
    try {
      const res = await client.getDeliveryOptions(
        create(GetDeliveryOptionsRequestSchema, { entryId }),
      );
      currentDeliveryOptions = res;
      const slot = $('deliveryDialog');
      slot.innerHTML = '';
      slot.append(
        deliveryDialog(res, {
          onConfirm: () => confirmDownload(res, account, provider, nativeId),
          onCancel: () => {
            slot.classList.add('hidden');
          },
        }),
      );
      slot.classList.remove('hidden');
      log('delivery options fetched — select recipe and sink');
    } catch (err) {
      log(`get delivery options failed: ${describe(err)}`);
    }
  }

  async function confirmDownload(opts, account, provider, nativeId) {
    const videoTrackId = $('recipeVideo')?.value || '';
    const audioTrackIds = Array.from(
      doc.querySelectorAll('[id^="audio-"]:checked'),
    ).map((cb) => cb.id.replace('audio-', ''));
    const subTrackIds = Array.from(
      doc.querySelectorAll('[id^="sub-"]:checked'),
    ).map((cb) => cb.id.replace('sub-', ''));
    const sink = $('sinkSelect')?.value || 'device';
    const container = $('containerSelect')?.value || '';
    const attachMetadata = $('metadataToggle')?.checked ?? false;

    try {
      const res = await client.startDelivery(
        create(StartDeliveryRequestSchema, {
          goal: DeliveryGoal.DOWNLOAD,
          provider,
          accountId: account.accountId,
          nativeId,
          sink,
          container: container || undefined,
        }),
      );
      const jobId = res.job?.id;
      if (!jobId) {
        log('start download: response carried no job id');
        return;
      }
      $('deliveryDialog').classList.add('hidden');
      log(
        `download started: ${jobId} (${provider}:${nativeId} via ${account.accountId}, sink ${sink})`,
      );
      await refreshDownloads();
    } catch (err) {
      log(`start download failed: ${describe(err)}`);
    }
  }

  async function refreshDownloads() {
    try {
      const res = await client.listJobs(create(ListJobsRequestSchema, {}));
      const downloadJobs = (res.jobs ?? []).filter(
        (j) => j.kind === 1 && j.delivery?.goal === 'download',
      );
      const slot = $('downloadsPanel');
      slot.innerHTML = '';
      slot.append(downloadsPanel(downloadJobs));
    } catch (err) {
      log(`list jobs failed: ${describe(err)}`);
    }
  }

  // --- Account ---

  function showRecoveryKey(recoveryKey) {
    const box = $('recovery');
    box.innerHTML = '';
    box.append(
      'Recovery key (shown once — store it safely): ',
      Object.assign(doc.createElement('strong'), { text: recoveryKey }),
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

  // --- State (debug views) ---

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

  // --- Events ---

  const feed = doc.getElementById('eventFeed');

  function applyFilter(selected) {
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
    applyFilter($('eventFilter').value);
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
        if (
          res.event?.type === EventType.DELIVERY_PLAY_MENU_READY &&
          eventPayload(res.event, 'playMenuReady')?.jobId === activeSessionId
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

  // --- Wire up DOM events ---

  function wireDOM() {
    $('linkVisibility').addEventListener('change', () => {
      $('linkSharedWith').classList.toggle(
        'hidden',
        $('linkVisibility').value !== 'shared',
      );
    });

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
          const src = playSource(item.entry, accounts);
          slot.append(
            libraryCard(item, {
              onPlay: src ? () => startPlay(src.provider, src.nativeId) : undefined,
              onDownload: src
                ? () => startDownload(item.entry.id, src.provider, src.nativeId)
                : undefined,
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

    $('liveSearch').addEventListener('click', liveSearch);

    $('logout').addEventListener('click', () => {
      stopSubscription();
      stopHeartbeat();
      setToken(null);
      username = null;
      activeSessionId = null;
      $('player').classList.add('hidden');
      $('playVideo').removeAttribute('src');
      $('deliveryDialog').classList.add('hidden');
      $('downloads').classList.add('hidden');
      $('accountList').innerHTML = '';
      $('libraryGrid').innerHTML = '';
      $('nextPage').classList.add('hidden');
      $('recovery').classList.add('hidden');
      $('copyRecovery').classList.add('hidden');
      updateSessionUI();
      log('logged out; token discarded');
    });

    $('refreshDownloads').addEventListener('click', refreshDownloads);

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
        setToken(res.token);
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
        setToken(res.token);
        await refreshAccounts();
        await refreshLibrary();
        updateSessionUI();
        log('logged in; session token stored in memory');
      } catch (err) {
        username = null;
        log(`login failed: ${describe(err)}`);
      }
    });

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

    // Event filter options
    for (const [name, value] of Object.entries(EventType)) {
      if (typeof value !== 'number' || value === EventType.UNSPECIFIED) continue;
      const option = doc.createElement('option');
      option.value = String(value);
      option.textContent = eventTypeLabel(value);
      $('eventFilter').append(option);
    }

    $('eventFilter').addEventListener('change', () => applyFilter($('eventFilter').value));

    $('subscribeToggle').addEventListener('click', () => {
      if (subAbort) stopSubscription();
      else startSubscription();
    });

    $('clearFeed').addEventListener('click', () => {
      feed.innerHTML = '';
    });
  }

  // Initialize
  wireDOM();
  updateSessionUI();
  loadInstanceInfo();

  return {
    // Pure functions (re-exported for tests)
    parseScopes,
    accountForDelivery,
    playSource,

    // Stateful API (for integration tests)
    refreshAccounts,
    refreshLibrary,
    refreshDownloads,
    refreshPlayInfo,
    startPlay,
    startDownload,
    showJob,
    loadInstanceInfo,
    startHeartbeat,
    stopHeartbeat,
    updateSessionUI,
    addEvent,
    applyFilter,
    liveSearch,

    // For testing: read internal state
    _state: () => ({
      token,
      username,
      accounts,
      libraryQuery,
      nextPageToken,
      activeSessionId,
      heartbeatIntervalMs,
    }),
  };
}
