# Testing Strategy

This document fixes *how the project is tested* and *what "tested" means*. PLAN.md fixes what the system is; IMPLEMENTATION.md fixes how it is built; this document fixes how we know the build is correct. It turns IMPLEMENTATION.md §1.1 (testable requirements), §4.2 (the conformance gate), and §6 (Definition of Done) into concrete, running practice.

The load-bearing idea: **a requirement without a fixture is not done** (IMPLEMENTATION.md §1.1). Everything in this document exists to make that rule mechanically enforceable.

## 1. Principles

1. **Fixture-first.** The fixture suite for a contract is written *before* the implementation that satisfies it (IMPLEMENTATION.md §1.2). A milestone is not done when it compiles; it is done when its fixtures pass and its store classes behave per §2.4 of PLAN.md.
2. **Reject, never downgrade** (§2.5 of PLAN.md). A slot that fails the fixture suite of the version it declares is rejected, never downgraded. The test suite must be the first place a failed claim shows up, not the registry.
3. **Drift is caught at build time, not production.** Contract drift is the biggest build-time risk (IMPLEMENTATION.md §7). Tests and CI exist so that drift surfaces before anything runs in production.
4. **Negative fixtures are mandatory.** Anything that accepts input — merge matching, manifest validation, session start — requires fixtures the schema *must reject*. Vacuous permissiveness is a test failure (§4.2 of IMPLEMENTATION.md).
5. **Storage behavior is tested, not assumed.** Every storage class's durability and who-reads semantics (§2.4 of PLAN.md) has a dedicated test set.
6. **Secrets discipline is tested.** The vault contract has its own suite proving encrypted-at-rest, in-memory-only during use, and nothing logged (IMPLEMENTATION.md §1.3).

## 2. The test pyramid, adapted

The classic pyramid maps onto this system in a specific way. The names are chosen to mean something here, not to match a generic book:

| Level | What it proves | What it is | Where it lives |
| --- | --- | --- | --- |
| **Fixture suites** | A slot implements the exact contract version it declares | The conformance gate; per contract, per version | `fixtures/<contract>/v<version>/` |
| **Round-trip / fuzz tests** | Schema encode→decode is lossless; unknown fields are preserved (§3.4 of PLAN.md) | Exercised on the load-bearing contracts first | per schema |
| **Unit tests** | Pure logic, no I/O: normalization (§5.3), matching heuristics (§5.3), target resolution (§6.3) | Fast, hermetic | colocated with code |
| **Integration / vertical-slice tests** | One request enters the API, crosses the core, touches one store, returns (IMPLEMENTATION.md §4.3) | The thinnest end-to-end path per capability | per milestone |

The **fixture suite is the top tier and the quality floor** (IMPLEMENTATION.md §4.2). The other three levels exist to keep it meaningful: round-trips prove the wire format, units prove the heuristics, slices prove the wiring.

### 2.1 Why the pyramid is inverted here

In a normal codebase, most tests are small unit tests. Here, the *contract* is the unit of correctness — a slot is any process in any language that speaks the schema. The fixture suite therefore carries the most weight; it is the only level that can say "this external component is correct." The other levels protect the code that implements and consumes those contracts.

## 3. Fixture suite mechanics

A fixture suite is **data, not code**: a schema descriptor plus sample requests and the responses the schema says they should produce (§2.5 of PLAN.md). Because it is data, any adapter in any language can run it — the runner is a small generic CLI that takes a suite and a transport and checks the responses. This is what keeps the conformance gate language-neutral (§2.1 of PLAN.md). API fixtures (`proto/abcmovies/api/v1`) test the core's API server, not any client; a frontend's own behavior is tested in `frontends/`, never in `fixtures/`.

A suite contains:

- **The contract descriptor** — the schema for the exact version under test.
- **Positive fixtures** — sample requests and their expected responses.
- **Negative fixtures** — requests the schema must reject. A slot that accepts them has not passed.
- **The handshake fixture** — the meta-contract (CapabilityQuery, §3.3 of PLAN.md) every slot must answer first.

Rules:

- A slot **declares only versions it has passed the fixture suite for** (§3.4 of PLAN.md). Declaration and verified behavior must agree.
- A **failed claim is a rejection**, surfaced in the registry, never a silent downgrade.
- Suites are versioned and immutable once a contract version ships; a breaking change is a new version with a new suite (§3.4 of PLAN.md).

### 3.1 The load-bearing contracts get the first suites

The load-bearing contracts (PLAN.md §2.3) get the first suites, the first round-trip tests, and the most careful review (IMPLEMENTATION.md §1.2). Their schemas are treated as frozen once approved. Cost of a late change here is the highest in the system, so their suites are the approval gate: nothing downstream is designed against a load-bearing contract until its fixtures pass.

## 4. Levels in detail

### 4.1 Round-trip / fuzz tests

The schema encoding's unknown-field preservation makes the additive-versioning rule (§3.4 of PLAN.md) native, but only if the consumer genuinely ignores and preserves what it does not understand. Round-trip tests prove:

- encode(parse(encode(x))) == encode(x) — the wire format is lossless.
- A message with unknown fields, passed through, still carries those fields — so an older consumer does not destroy a newer message.
- Fuzzing over the load-bearing contracts — random mutations must parse or reject cleanly, never panic or corrupt.

### 4.2 Unit tests

Hermetic, no I/O. The trickiest pure logic gets the densest coverage:

- **Title normalization** (§5.3): Unicode normalization, diacritics, articles, whitespace.
- **Matching heuristics** (§5.3): the merge rule (normalized title + year + corroborating signal), the ranked corroborating signals, the "otherwise they stay separate" default.
- **Target resolution** (§6.3): native-first, encode-only-as-fallback, never-upscale, feasibility bounds.
- **Pipeline selection** (§6.3): defaults and the precedence ladder (§6.1).

These are fast and are what `make test-unit` runs.

### 4.3 Integration / vertical-slice tests

One request enters the API, crosses the core, touches one store, returns (IMPLEMENTATION.md §4.3). These are the milestone acceptance criteria made runnable: M1's "a real provider's catalogue lands in the account source cache," M4's "one passthrough play session and one remux download session end-to-end," M6's "revocation kills live sessions." They run against real transports (in-process and, where the milestone adds one, subprocess/network) but against **mocked provider bytes** — providers are never contacted in the test suite.

## 5. Storage class tests

One test set per row of the §2.4 table, proving the class's defining property — this is what "tested, not assumed" (IMPLEMENTATION.md §6) means mechanically:

| Store class | The property the test set proves |
| --- | --- |
| Account source cache | Rebuildable from the provider for library-class providers; best-effort, rebuild-by-usage for lazy providers (§2.4); keyed by streaming account, not the instance user |
| Metadata cache | Rebuildable from catalogues and providers; global across users; one TitleMetadata record per title with external-ID-to-record lookup |
| Content-key cache | Keyed by content (provider, contentId, keyId); entries carry only material + validity (TTL = license validity); encrypted at rest; fail-fast — a decrypt failure drops the entry and re-licenses; credential rotation never purges (§6.6) |
| Vault | Must not lose; per-session keys wrapped by owner's KEKs + host relay key; encrypted at rest (§6 below) |
| Watch history / playlists | Must not lose; user's key; encrypted |
| Derived library cache | Rebuildable from source caches + enrichment; per-user — including guests, keyed `guest:<deviceId>` with a device-session TTL |
| Job/session state | Checkpointed; loss recoverable via queries |
| Event bus | Ephemeral; a lost notification is free to lose because the job object is queryable |

The storage-agnostic rule from §2.4 is also tested: **media bytes are never cached** — no store holds audio/video data; a "download" is a delivery session, not a copy into a cache.

## 6. Vault / secrets discipline tests

The trust model of PLAN.md §7.6 carries implementation duties (IMPLEMENTATION.md §1.3), and the discipline is enforced in the vault's test suite from day one:

- **Key derivation is client-side**: the server never sees the raw password or recovery key. The test suite proves login submits derived material — which the server then holds as the unwrapping key — never a plaintext secret.
- **Encrypted at rest.** A vault file on disk is ciphertext; the suite proves plaintext is not recoverable from the file without the key.
- **In-memory only during use.** The suite proves decrypted material is held only for the duration of a relay.
- **Nothing logged.** A negative test proves decrypted material never reaches a logger — this is the "never logged" half of the discipline, tested as a failure condition.
- **Relay-key scoping.** The host-held relay key unwraps account sessions and never user blobs — a negative test proves the relay key cannot decrypt a user blob, and a positive test proves a member relay succeeds without the owner's KEK present.
- **Owner-only ops.** Re-link, re-auth, and revoke fail without the owner's KEK even when the relay key is present — the owner remains the only principal who can change what is stored.

This suite runs in CI on every change; it is not optional and cannot be skipped for speed (CI-CD.md §2).

## 7. The DRM exemption

Real decryption cannot be a fixture (decrypting real content is not something a test can do) — so the DRM slot is admitted on **contract-only mock fixtures** covering the license-negotiation message shapes (challenge, response, keys — §6.6 of PLAN.md), plus a **documented manual verification path** (§2.5 of PLAN.md). The **composition logic — acquire-keys → decrypt → compose — is not exempt**: it is gated by **synthetic encrypted fixtures** (test media encrypted with known keys) so the pipeline itself is verified end-to-end without any live license server (§6.6 of PLAN.md). The exemption is narrow: mock fixtures prove the message shapes, synthetic fixtures prove the pipeline, and both are gated by the same "reject, never downgrade" rule. What is exempted is only *live device-level decryption*, never the contract and never the composition.

## 8. Test data and mocks

- **Fixtures over real bytes, never real services.** Mock providers return fixture manifests (the abridged manifest example in §6.2 of PLAN.md is a fixture). Real providers and catalogues are never contacted by any test.
- **Deterministic data.** Every suite is reproducible: fixed seeds for anything randomized (jitter, backoff — §5.4 of PLAN.md), so a passing suite passes forever until something actually changes.
- **Fixture factories.** Sample manifests, entries, and events are generated from factories, not copy-pasted blobs — so a field added to a schema (§3.4 additive rule) updates every fixture uniformly.

## 9. Layout and naming conventions

- **`fixtures/<contract>/v<version>/`** — the immutable per-version suites, plus `handshake/` and `negative/` subdirectories.
- **`tests/`** — mirrors the milestone structure (M0–M9) so a milestone's acceptance criteria are findable next to its number.
- **Unit tests colocated with code** — the normal convention, since they test pure logic.
- **One requirement → one fixture** (IMPLEMENTATION.md §1.1): each requirement keyword (MUST/SHOULD/MAY) is traceable to a fixture by name, so "is this done?" has a mechanical answer.

## 10. Running the suite

The check recipe runs everything — lint, build, unit, round-trip, integration, and the fixture suites for every built-in adapter (ENVIRONMENT.md §5). CI runs the identical recipes (CI-CD.md §2). There is no separate "CI-only" test set; anything worth testing in CI is worth running locally, and vice versa.

## 11. Relationship to the other documents

- **ENVIRONMENT.md** — provides the reproducible environment and the check-recipe entry point that runs everything in §4–§7.
- **CI-CD.md** — runs the same suites as a gate (§2) and refuses to ship a milestone whose fixtures do not pass (§3). The conformance gate in CI (CI-CD.md §2) is this document's §3 made mechanical.
- **IMPLEMENTATION.md §6** — the Definition of Done this document operationalizes; every checkbox there that says "tested" or "fixtures pass" points here.
- **THREAT-MODEL.md** — the attacker model behind the vault/secrets suite (§6) and the storage-class tests (§5); each threat row maps to a test here. **TECHNICAL-DECISIONS.md** — the implementation choices (core language, API transport) that determine which languages the fixture runner must support.
