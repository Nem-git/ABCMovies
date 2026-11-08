// The browser journey suite: the real user journey through the real UI —
// sign up, link a provider account, browse the library, play a title —
// against the real server and core, with a fake Jellyfin provider standing
// in for the network (TESTING.md §8: providers are never contacted by any
// test). The API-level flows are covered by the Go serving-layer tests
// (frontends/web/serving/browser_test.go); this suite covers the UI wiring
// in between — the buttons, cards, menus and events an actual user touches.
//
// The suite runs against a fresh instance of its own
// (playwright.journey.config.js): the first signup is the bootstrap operator
// (core/internal/auth/auth.go — the first user becomes operator), which
// creates the provider and sink slots through the admin API, the same
// pattern serving/browser_test.go proves.

import { test, expect } from '@playwright/test';
import { create } from '@bufbuild/protobuf';
import { Code, createClient } from '@connectrpc/connect';
import { createGrpcWebTransport } from '@connectrpc/connect-web';

import {
  CoreService,
  CreateSlotRequestSchema,
  LoginRequestSchema,
  PasswordLoginSchema,
  PasswordSignUpSchema,
  SignUpRequestSchema,
} from '../../src/gen/abcmovies/api/v1/core_pb.js';
import {
  ProviderConfigSchema,
  SinkConfigSchema,
  SlotKind,
  SlotSchema,
} from '../../src/gen/abcmovies/core/v1/slot_pb.js';
import {
  SEED_USERNAME,
  startFakeJellyfin,
  REJECTED_USERNAME,
} from './helpers/fakejellyfin.js';

const BASE_URL = 'http://127.0.0.1:18443';
// The gondwana entry's stable identifier. The e2e instance runs no catalogue
// slots, so the metadata cache is empty and library cards render the entry's
// external identity rather than a display title — the identity is the stable
// handle for "the gondwana card".
const GONDWANA = 'tt-gondwana';

let fake;

test.beforeAll(async () => {
  fake = await startFakeJellyfin();
  await createOperatorSlots();
});

test.afterAll(async () => {
  await fake.close();
});

const OPERATOR_USERNAME = 'journey-operator';
const OPERATOR_PASSWORD = 'operator-password';

// createOperatorSlots creates the slots an operator would after D8 — through
// the admin API, not config. The provider slot carries one seed account: the
// jellyfin adapter refuses to build without at least one declared account, and
// the fake serves that seed an empty library, so it never feeds the journey's
// library — the account linked through the UI is what fills it. The seed is
// given a valid password (JOURNEY_SEED_PASSWORD in the webServer command) so it
// syncs cleanly; a seed that failed to log in would trip the slot's shared
// per-provider pacing governor and take the linked account down with it
// (PLAN.md §5.4: one throttled account throttles all). The sink slot is the
// relay pull sink under the id "device" — the sink id the UI's play and
// download flows hardcode — which stages the per-track relay tickets the
// serving layer mounts under /media/relay/.
async function createOperatorSlots() {
  const authed = createClient(
    CoreService,
    createGrpcWebTransport({
      baseUrl: BASE_URL,
      interceptors: [
        (next) => async (req) => {
          req.header.set('Authorization', `Bearer ${await operatorToken()}`);
          return next(req);
        },
      ],
    }),
  );
  await createSlotIgnoringExisting(
    authed,
    create(SlotSchema, {
      id: 'jf',
      kind: SlotKind.PROVIDER,
      config: {
        case: 'provider',
        value: create(ProviderConfigSchema, {
          adapter: 'jellyfin',
          server: fake.url,
          syncCadence: '5m',
          accounts: [
            {
              id: 'jf-seed',
              username: SEED_USERNAME,
              passwordEnv: 'JOURNEY_SEED_PASSWORD',
            },
          ],
        }),
      },
    }),
  );
  await createSlotIgnoringExisting(
    authed,
    create(SlotSchema, {
      id: 'device',
      kind: SlotKind.SINK,
      config: {
        case: 'sink',
        value: create(SinkConfigSchema, { adapter: 'relay' }),
      },
    }),
  );
}

let operatorTokenCache;
let operatorTokenInFlight;

// operatorToken returns a session token for the bootstrap operator. On the
// first call it signs up (the first user becomes operator); if that user
// already exists — a Playwright worker restart re-runs beforeAll against the
// same server — it logs in instead. The token is cached for the worker's life.
function operatorToken() {
  if (operatorTokenCache) return Promise.resolve(operatorTokenCache);
  if (!operatorTokenInFlight) {
    operatorTokenInFlight = bootstrapOperator().then((token) => {
      operatorTokenCache = token;
      return token;
    });
  }
  return operatorTokenInFlight;
}

async function bootstrapOperator() {
  const client = createClient(
    CoreService,
    createGrpcWebTransport({ baseUrl: BASE_URL }),
  );
  const password = new TextEncoder().encode(OPERATOR_PASSWORD);
  const authMethod = (schema) => ({
    case: 'password',
    value: create(schema, { password }),
  });
  try {
    const res = await client.signUp(
      create(SignUpRequestSchema, {
        username: OPERATOR_USERNAME,
        authMethod: authMethod(PasswordSignUpSchema),
      }),
    );
    return res.token;
  } catch {
    const res = await client.login(
      create(LoginRequestSchema, {
        username: OPERATOR_USERNAME,
        authMethod: authMethod(PasswordLoginSchema),
      }),
    );
    return res.token;
  }
}

// createSlotIgnoringExisting creates one slot, tolerating a duplicate: a
// worker restart re-runs beforeAll against the same server, where the slots
// already exist.
async function createSlotIgnoringExisting(client, slot) {
  try {
    await client.createSlot(create(CreateSlotRequestSchema, { slot }));
  } catch (err) {
    if (err?.code !== Code.AlreadyExists) throw err;
  }
}

let userCounter = 0;

// signUpThroughPage drives the real signup form — the same path the smoke
// suite uses — and waits for the session chip to confirm the login.
async function signUpThroughPage(page) {
  userCounter += 1;
  const username = `journey_${Date.now()}_${userCounter}`;
  await page.goto('/');
  await page.fill('#username', username);
  await page.fill('#password', 'journey-password');
  await page.click('#signup');
  await expect(page.locator('#sessionChip')).toHaveText(
    `logged in as ${username}`,
    { timeout: 10_000 },
  );
  return username;
}

// linkAccountThroughPage fills and submits the provider-account form.
async function linkAccountThroughPage(page, { username, password } = {}) {
  await page.fill('#linkServer', fake.url);
  await page.fill('#linkUsername', username ?? 'journey.user');
  await page.fill('#linkPassword', password ?? 'journey-password');
  await page.click('#linkAccount');
}

// expectLinkedAccount waits until the linked account's card is in the list:
// the badge marks it caller-linked, and the server row shows the fake
// provider's URL.
async function expectLinkedAccount(page) {
  await expect(page.locator('#accountList')).toContainText('linked by you', {
    timeout: 10_000,
  });
  await expect(page.locator('#accountList')).toContainText(fake.url);
}

// browseUntilCard clicks Browse until the titled card renders. The first
// catalogue sync runs in the background after linking (FirstSync in
// core/app/slots.go), so the library fills in over the next seconds;
// re-clicking Browse re-fetches until the sync has landed.
async function browseUntilCard(page, title) {
  const card = page.locator('#libraryGrid .card', { hasText: title });
  await expect
    .poll(
      async () => {
        await page.click('#browse');
        return card.isVisible();
      },
      { timeout: 15_000, message: `library never showed "${title}"` },
    )
    .toBe(true);
}

test.describe('frontend journey', () => {
  test('link a provider account through the UI', async ({ page }) => {
    await signUpThroughPage(page);
    await linkAccountThroughPage(page);
    await expectLinkedAccount(page);
    await expect(page.locator('#log')).toContainText('account linked:');
  });

  test('linking with rejected credentials fails cleanly', async ({ page }) => {
    await signUpThroughPage(page);
    await linkAccountThroughPage(page, { username: REJECTED_USERNAME });
    await expect(page.locator('#log')).toContainText('link account failed', {
      timeout: 10_000,
    });
    // The failed link added nothing: no caller-linked card appears. (The
    // operator's public seed account is visible to every user, so the
    // list's empty hint is not a valid signal here.)
    await expect(page.locator('#accountList')).not.toContainText(
      'linked by you',
    );
  });

  test('browse the library after linking', async ({ page }) => {
    await signUpThroughPage(page);
    await linkAccountThroughPage(page);
    await expectLinkedAccount(page);
    await page.click('#browse');
    await browseUntilCard(page, GONDWANA);
    await expect(page.locator('#libraryGrid .card')).toHaveCount(3);
  });

  test('play a title through the UI', async ({ page }) => {
    await signUpThroughPage(page);
    await linkAccountThroughPage(page);
    await expectLinkedAccount(page);

    // Subscribe before playing so the play-menu-ready event reaches the
    // page. Probe-job status events only land in the feed once the
    // bus has the subscriber registered (the bus is ephemeral), so
    // re-create probe jobs until one lands — a deterministic readiness
    // check made only of UI actions.
    await page.click('#subscribeToggle');
    await expect
      .poll(
        async () => {
          await page.click('#probeJob');
          return page.locator('#eventFeed .card.event').count();
        },
        {
          timeout: 15_000,
          message: 'subscription never went live (no probe events in the feed)',
        },
      )
      .toBeGreaterThan(0);

    await browseUntilCard(page, GONDWANA);
    const card = page.locator('#libraryGrid .card', { hasText: GONDWANA });
    await card.getByRole('button', { name: 'Play' }).click();

    // The delivery stages its menu, the event refreshes the page's menu,
    // and the video element gets the relay URL — the UI's own delivery
    // proof (the mkv container does not decode in headless Chromium). A
    // WHOLE_MUX source surfaces one located track: its carrier carries the
    // audio and subtitle streams, so they are read off it, not listed as
    // their own relay entries (PLAN.md §6.2).
    await expect(page.locator('#player')).toBeVisible();
    await expect(page.locator('#playTrackList .track')).toHaveCount(1, {
      timeout: 15_000,
    });
    await expect(page.locator('#playTrackList .track')).toContainText(
      'container',
    );
    const video = page.locator('#playVideo');
    await expect(video).toHaveAttribute('src', /\/media\/relay\//);

    // Pull the relay URL the way the browser would: the bytes are the
    // provider's, served by the core without a second credential.
    const src = await video.getAttribute('src');
    const res = await page.request.get(src);
    expect(await res.text()).toBe('P5-gondwana-mkv-bytes');
    expect(fake.hits('movie-gondwana')).toBeGreaterThan(0);
  });

  test('download dialog through the UI', async ({ page }) => {
    await signUpThroughPage(page);
    await linkAccountThroughPage(page);
    await expectLinkedAccount(page);
    await browseUntilCard(page, GONDWANA);

    const card = page.locator('#libraryGrid .card', { hasText: GONDWANA });
    await card.getByRole('button', { name: 'Download' }).click();

    // The dialog is the delivery-options surface: recipe (video/audio/
    // subtitle), destination sink, and container selects.
    const dialog = page.locator('#deliveryDialog');
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('#recipeVideo')).toBeVisible();
    await expect(dialog.locator('#sinkSelect')).toBeVisible();
    await expect(dialog.locator('#containerSelect')).toBeVisible();

    await dialog.getByRole('button', { name: 'Start Download' }).click();

    // The download lands in the downloads panel, and the log records it.
    await expect(page.locator('#log')).toContainText('download started:', {
      timeout: 15_000,
    });
    await expect(page.locator('#downloadsPanel .card')).toHaveCount(1, {
      timeout: 15_000,
    });
  });
});
