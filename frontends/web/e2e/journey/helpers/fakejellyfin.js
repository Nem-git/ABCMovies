// A self-contained Jellyfin stand-in for the browser journey tests. It answers
// exactly the REST contract the jellyfin adapter speaks — authenticate, index,
// playback info, direct stream — the same contract the Go serving-layer fake
// (frontends/web/serving/fakejellyfin_test.go) and the adapter's own unit
// tests exercise, so the journey drives the unmodified adapter end to end.
//
// Deterministic (TESTING.md §8): a fixed three-item catalog and fixed stream
// bytes. Credentials are accepted unless they match the documented rejection
// rule, which drives the negative link test.

import { createServer } from 'node:http';

const ACCESS_TOKEN = 'srv-access-token';

// The rejection rule: these credentials are refused with 401. Everything
// else is accepted. Empty passwords are refused too, as a real Jellyfin does.
export const REJECTED_USERNAME = 'rejected-user';
export const REJECTED_PASSWORD = 'wrong';

// SEED_USERNAME is the operator's declared seed account (journey.spec.js). The
// jellyfin adapter refuses to build without at least one account, so the
// operator declares this one; the fake models it as a valid user with an empty
// library, so the seed satisfies the adapter without feeding the journey's
// library — the UI-linked account remains the only source of items.
export const SEED_USERNAME = 'journey.seed';

// userIdFor maps a login name onto the Jellyfin user id the item index is
// keyed by. The adapter passes the login's user id as the /Items userId
// parameter, so the fake can tell one account's library from another's.
function userIdFor(username) {
  return `jf-user-${username}`;
}

const catalog = [
  {
    id: 'movie-gondwana',
    type: 'Movie',
    name: 'The Last Gondwana Gardener',
    year: 2021,
    providers: { Imdb: 'tt-gondwana', Tmdb: '12' },
  },
  {
    id: 'movie-coral',
    type: 'Movie',
    name: 'Coral Skies',
    year: 2019,
    providers: { Tmdb: '23' },
  },
  {
    id: 'series-tidal',
    type: 'Series',
    name: 'Tidal Station',
    year: 2022,
    providers: { Tvdb: '99' },
  },
];

// Fixed provider stream bytes; the play journey asserts the relay serves
// exactly these through the core.
const streamBytes = {
  'movie-gondwana': 'P5-gondwana-mkv-bytes',
  'movie-coral': 'P5-coral-mkv-bytes',
  'series-tidal': 'P5-tidal-mkv-bytes',
};

function sendJson(res, status, body) {
  const payload = JSON.stringify(body);
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(payload);
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on('data', (c) => chunks.push(c));
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
    req.on('error', reject);
  });
}

// POST /Users/AuthenticateByName — Jellyfin 10.9+ carries the access token
// in the response body, not the X-Emby-Token header.
async function authenticate(req, res) {
  let creds;
  try {
    creds = JSON.parse(await readBody(req));
  } catch {
    return sendJson(res, 400, { message: 'invalid json' });
  }
  if (
    creds.Username === REJECTED_USERNAME ||
    creds.Pw === REJECTED_PASSWORD ||
    creds.Pw === ''
  ) {
    return sendJson(res, 401, { message: 'credentials rejected' });
  }
  return sendJson(res, 200, {
    AccessToken: ACCESS_TOKEN,
    User: { Id: userIdFor(creds.Username) },
  });
}

// GET /Items — the paged movie+series index. Pagination mirrors the Go fake:
// a page that would end before the catalog runs out is truncated to
// [start, start+limit); otherwise everything from start is returned, so the
// adapter's page size (500) returns the whole catalog in one request.
function index(req, res, url) {
  if (
    !String(req.headers.authorization ?? '').startsWith('MediaBrowser Token=')
  ) {
    return sendJson(res, 401, { message: 'missing authorization' });
  }
  const wantKinds = new Set(
    (url.searchParams.get('includeItemTypes') ?? '')
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean),
  );
  // The seed account is a valid user with an empty library; every other
  // account sees the fixed catalog.
  const library =
    url.searchParams.get('userId') === userIdFor(SEED_USERNAME) ? [] : catalog;
  const all = library.filter(
    (it) => wantKinds.size === 0 || wantKinds.has(it.type),
  );
  const start = Number(url.searchParams.get('startIndex') ?? 0) || 0;
  const limit = Number(url.searchParams.get('limit') ?? 0) || 0;
  let items;
  if (limit > 0 && start + limit < all.length) {
    items = all.slice(start, start + limit);
  } else if (start < all.length) {
    items = all.slice(start);
  } else {
    items = [];
  }
  return sendJson(res, 200, {
    Items: items.map((it) => ({
      Id: it.id,
      Type: it.type,
      Name: it.name,
      ProductionYear: it.year,
      ProviderIds: it.providers,
    })),
    TotalRecordCount: all.length,
    StartIndex: start,
  });
}

// POST /Items/{id}/PlaybackInfo — one mkv media source with a video, audio
// and subtitle stream, the same shape the Go fake returns.
function playbackInfo(req, res) {
  if (
    !String(req.headers.authorization ?? '').startsWith('MediaBrowser Token=')
  ) {
    return sendJson(res, 401, { message: 'missing authorization' });
  }
  return sendJson(res, 200, {
    MediaSources: [
      {
        Id: 'ms-movie-gondwana',
        Container: 'mkv',
        MediaStreams: [
          {
            Type: 'Video',
            Codec: 'hevc',
            Width: 1920,
            Height: 1080,
            Language: 'eng',
          },
          {
            Type: 'Audio',
            Codec: 'truehd',
            Channels: 8,
            ChannelLayout: '7.1',
            Language: 'eng',
          },
          { Type: 'Subtitle', Codec: 'srt', Language: 'eng' },
        ],
      },
    ],
  });
}

// GET /Videos/{id}/stream — the provider's bytes, counted so tests can
// assert the relay actually pulled them.
function stream(req, res, url, hits) {
  const params = new URLSearchParams(url.search);
  if (params.get('api_key') !== ACCESS_TOKEN) {
    return sendJson(res, 401, { message: 'missing api key' });
  }
  const id = url.pathname.replace(/^\/Videos\//, '').replace(/\/stream$/, '');
  hits.set(id, (hits.get(id) ?? 0) + 1);
  const body = Buffer.from(streamBytes[id] ?? '');
  res.writeHead(200, {
    'Content-Type': 'application/octet-stream',
    'Content-Length': String(body.length),
  });
  res.end(body);
}

// FAKE_PORT is the fixed loopback port the fake binds. It is fixed, not
// ephemeral, so its URL is stable across Playwright worker restarts: the core
// stores the fake's URL on the provider slot, and a restart that changed the
// port would leave the slot pointing at a dead server — the seed's sync would
// fail and retrip the shared pacing governor, and a newly linked account's
// server would no longer match the slot.
export const FAKE_PORT = 18444;

// startFakeJellyfin starts the fake on its fixed loopback port and resolves
// once it is listening.
export function startFakeJellyfin() {
  const hits = new Map();
  const server = createServer((req, res) => {
    const url = new URL(req.url, 'http://127.0.0.1');
    if (req.method === 'POST' && url.pathname === '/Users/AuthenticateByName') {
      authenticate(req, res).catch(() =>
        sendJson(res, 500, { message: 'boom' }),
      );
    } else if (req.method === 'GET' && url.pathname === '/Items') {
      index(req, res, url);
    } else if (
      req.method === 'POST' &&
      /^\/Items\/[^/]+\/PlaybackInfo$/.test(url.pathname)
    ) {
      playbackInfo(req, res);
    } else if (
      req.method === 'GET' &&
      /^\/Videos\/[^/]+\/stream$/.test(url.pathname)
    ) {
      stream(req, res, url, hits);
    } else {
      sendJson(res, 404, { message: 'not found' });
    }
  });
  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(FAKE_PORT, '127.0.0.1', () => {
      resolve({
        url: `http://127.0.0.1:${FAKE_PORT}`,
        hits: (id) => hits.get(id) ?? 0,
        close: () => new Promise((done) => server.close(() => done())),
      });
    });
  });
}
