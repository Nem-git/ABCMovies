# Threat Model

This document states **what the project protects, against whom, and how** — the structured attacker model behind PLAN.md §7.6's trust classes. It exists so the security claims in PLAN.md are testable: every threat here maps to a test in TESTING.md (§5 storage-class, §6 vault/secrets) and an enforcement point in CI-CD.md §2. It is a living document: any change to PLAN.md §7.6 (or to the trust classes) updates this document, and a new threat is not accepted without a test.

PLAN.md §7.6's honesty principle governs everything here: there is no mathematical boundary; there is discipline and disclosure. This document is the disclosure, made concrete.

## 1. Assets and trust classes

| Asset | Where | Trust class (PLAN.md §7.6) | Loss consequence |
|---|---|---|---|
| Account sessions (tokens/cookies) | Vault | **Policy-not-proof**: host holds the relay key and can decrypt | Attacker can use accounts on the user's behalf; leaking them logs everyone out (§7.6) |
| Host relay key | Instance keystore | Disclosed server secret, scoped to account sessions only | Decrypts all member relays; rotation kills every member relay at once |
| User blobs (history, playlists, derived library) | Per-user encrypted stores | User's DEK; host unwraps during processing (process promise, not proof) | Loss = user's personal data; compromise = privacy break |
| Password / recovery-key material | Client only, never server | Client-side | Server compromise cannot recover it |
| Content keys | Content-key cache | Encrypted at rest; a hint, not a guarantee (§6.6) | Attacker could decrypt cached media keys; fail-fast re-license limits window |
| Media bytes | In flight only | Transient; never cached (§2.4) | Eavesdropping (mitigated by transport encryption) |
| Audit/usage logs | Ops stores | Content-blind: volume and timing only (§1.3) | Log leak reveals nothing about item identity |
| Session→account index | In-memory | Engine-internal | Underpins revocation; loss recoverable via queries (§2.4) |

## 2. Actors

| Actor | Trust granted | Can do |
|---|---|---|
| **Unauthenticated external attacker** | None | Probe API, attempt auth bypass, DoS |
| **Member / guest** | Their own scoped access | Use their library; must never observe another member (invariant, §2.2) |
| **Revoked member** | None after revocation | Must lose all active sessions instantly (§7.1) |
| **Provider / catalogue** | Sees its own requests and bytes | Legitimate partner; a hostile one can poison identity or throttle (§5.3, §5.4) |
| **Slot code** (third-party adapter) | Least privilege for its job; **sees what it carries** (accepted, §1.3) | Trusted like any dependency; isolated on subprocess/network transports (§4) |
| **Instance operator / compromised host** | Holds vault, relay key, DEKs during processing | The policy-not-proof actor: can decrypt the vault by design (§7.6, SCOPE.md sign-off) |
| **DRM licensor** (v2 only) | Sees license negotiation | Threat surface for v2 (§6.6); out of v1 scope |

## 3. Threats

| # | Threat | Target | Attacker | Mitigation (PLAN.md ref) | Verified by (TESTING.md ref) |
|---|---|---|---|---|---|
| T1 | Vault decryption from stolen instance storage | Account sessions | Physical / external | Encrypted at rest; per-session keys wrapped by owner KEKs + relay key (§2.4, §7.6) | §6 "encrypted at rest" |
| T2 | Relay-key compromise decrypts all member relays | Account sessions | Operator / compromised host | Relay key scoped to sessions only, never user blobs; rotating it kills member relays (§7.6) | §6 relay-key scoping (negative: relay key cannot decrypt a user blob) |
| T3 | Credential capture during relay | Session material in memory | Malicious slot code | Slots see what they carry (accepted, §1.3); least privilege; decrypted material in-memory only, never logged (§7.6) | §6 "nothing logged" (negative) |
| T4 | User-blob decryption at rest | History, playlists, derived library | Operator / physical | DEK wrapped by password-KEK + recovery-KEK; unwrap only during processing; recovery key is the sole reset path (§7.6) | §6; §5 watch-history store class ("user's key, encrypted") |
| T5 | Password/recovery-key theft via server | Login credentials | External | Client-side key derivation; server stores derived material only, never plaintext (IMPLEMENTATION.md §1.3) | §6 "key derivation is client-side; server never sees raw secret" |
| T6 | Member observes another member | Sessions, history, quota, library | Member | Member-scoping invariant (§2.2); per-user libraries (§5.1); events tenancy-routed and scope-filtered (§9.2) | M6 "member-scoping invariant is tested, not asserted"; §9.2 event-routing tests |
| T7 | Revoked member keeps streaming | Active delivery sessions | Revoked member | Engine-side kill via session→account index; in-progress downloads discarded (§7.1) | M6 "revocation kills live sessions" |
| T8 | Guest mints identities to exceed policy | Policy / concurrency caps | Guest | Device identity with session TTL; guest rate limits + per-instance concurrency cap (§2.2) | §5 derived-library guest cache; guest rate-limit tests |
| T9 | Vault decryption by the operator | Account sessions | Instance operator | **Accepted by design**: server-held is policy-not-proof; the provable option is the sidecar (v2, §7.4). Disclosed in §7.6 and signed in SCOPE.md | SCOPE.md sign-off; §6 owner-only ops still hold |
| T10 | Content-key cache compromise | DRM keys | Operator | Encrypted at rest; fail-fast re-license drops the entry; credential rotation never purges (§6.6) | §5 content-key cache class; M8 (v2) |
| T11 | Media interception in transit | Media bytes | Network | Transport encryption on all transports; egress routing separates control from bulk (v2, §7.3) | Integration tests over real transports (§4.3) |
| T12 | Audit/log leak reveals item identity | Logs | Anyone with log access | Content-blind logging: volume and timing only (§1.3, §7.6) | §6 "nothing logged"; CI secret-leak gate (CI-CD.md §4) |
| T13 | Malicious or non-conforming slot admitted | Any core capability | Slot provider | Fixture suite + negative fixtures; reject, never downgrade (§2.5); isolation on subprocess/network (§4); user slots tenancy-scoped and revocable (§4.1) | TESTING.md §3 fixture gate; negative fixtures mandatory |
| T14 | Identity/merge poisoning via bad metadata | Library integrity | Hostile provider / enrichment | Provenance on every external ID; corroboration required for heuristic IDs; coverage never drives identity (§5.3) | M2/M3 fixtures (negative: no merge without corroboration) |
| T15 | Request floods / queue exhaustion | Instance availability | External | Admission control, per-resource caps, backpressure, busy answers carry queue position (§6.5); pacing budgets (§7.2) | Integration tests on queue behavior; M6 pacing |

## 4. Trust boundaries

```
 Client (browser/CLI)          holds password + recovery key; derives KEKs (client-side)
        │  API (chosen transport, §8) derived material, never raw secrets (T5)
        ▼
 CORE                          holds relay key, DEKs during processing, vault access;
                              content-blind logs (T12); session→account index
        │  contract traffic (§2.1)  slots see carried traffic by design (T3)
        ▼
 SLOTS (least privilege)      subprocess/network isolation (§4); user slots tenancy-scoped (§4.1)
        │  clear traffic (§1.3)     search/enrichment queries are visible to providers
        ▼
 PROVIDERS / CATALOGUES       see their own requests; hostile ones ⇒ T14
```

The boundary that matters is **client ⇄ core**: crossing it never transmits a raw secret. Every other boundary is either accepted-by-design (operator, slots-see-traffic) or transport-level encryption.

## 5. Accepted risks (recorded, not mitigated)

- **Operator can decrypt the vault** in server-held mode (T9). The honest mitigation is the sidecar, which is v2.
- **Slots see what they carry** (T3) — trust-like-any-dependency, not a guarantee.
- **Search/enrichment queries travel in the clear to providers** (§1.3).
- **Member relays need no owner presence** via the relay key — that is a feature and a risk; the relay key's scope is the mitigation (T2).

## 6. Maintenance

- Any change to PLAN.md §7.6 updates §1/§3 here.
- A new threat requires a new row **and** a test (TESTING.md §5 or §6); a threat without a test is not accepted.
- The vault/secrets suite (TESTING.md §6) is the enforcement of T1–T5, T12 and runs in CI on every change — never skipped (CI-CD.md §2, §8).

## 7. Relationship to the other documents

- **PLAN.md §7.6, §1.3** — the trust model this document operationalizes.
- **TESTING.md §5, §6** — the tests that verify each row; the vault suite is T1–T5/T12's enforcement.
- **CI-CD.md §2, §4** — where the tests gate merges and the secret-leak check runs.
- **SCOPE.md §3** — the operator sign-offs that accept T9 and the content-blind scope.