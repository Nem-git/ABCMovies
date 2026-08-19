# Operations

This document fixes **how an instance of the project runs in production**. ENVIRONMENT.md covers a developer machine; CI-CD.md covers building and shipping; this document covers operating a live instance: deployment, the must-not-lose vault, key rotation, monitoring, and capacity. It stays **content-blind** by the same rule as the core (§1.3, §7.6 of PLAN.md): operational logs record volume and timing, never item identity.

Scope note: this describes v1 (SCOPE.md: M0–M6). The DRM slot, sidecar custody, and egress routing are v2 and out of scope here.

## 1. Deployment topology

v1 is a **single-instance deployment**: one process running the core, with the stores (§2.4 of PLAN.md) behind it. Everything else is optional and additive.

```text
            Internet / LAN
                 │  API transport (§8, TECHNICAL-DECISIONS.md §1.2)
        ┌────────▼─────────┐
        │      CORE        │   one static binary
        │ (registry, API,  │
        │ delivery engine) │
        └─┬──────┬──────┬──┘
          │      │      │
   ┌──────▼─┐ ┌──▼────┐ ┌▼───────┐
   │ stores │ │ vault │ │ caches │  durability per §2.4
   └────────┘ └───────┘ └────────┘
        │
   ┌────▼─────┐
   │ adapters │   built-in slots (first provider per TECHNICAL-DECISIONS.md §1.8, …), subprocess or network transports
   └──────────┘
```

- The core is the only always-on component. Slots are subprocesses or network services declared in config (§4 of PLAN.md).
- The API transport is the choice recorded in TECHNICAL-DECISIONS.md §1.2; transport encryption terminates at the core (or at a fronting reverse proxy — an acceptable deployment choice, since the core's API is the boundary).
- Store backends are chosen per class at implementation time (TECHNICAL-DECISIONS.md §2); durability requirements are per §2.4 and are the non-negotiable part.

## 2. The vault: backup and restore

The vault (§2.4: durable, **must not lose**; losing it logs everyone out) is the single most important operational object.

**Backup:**

- Back up the vault file and its key-encryption keys (owner KEKs, host relay key) together — a vault without its keys is a vault you cannot open.
- Run vault backups on a defined schedule; verify a restore from each backup at least once per release cycle (a backup that has never been restored is a hope, not a backup).
- Backup media inherits the same encryption-at-rest discipline as the live vault; it must never be stored in the clear.

**Restore:**

- Restore the vault file and the matching keys to a clean instance.
- Everything else rebuilds: the account source cache (library-class providers), metadata cache, derived library cache are all rebuildable (§2.4). History/playlists are per-user encrypted blobs backed up with the vault.
- After a vault restore, sessions with validity remaining work again; sessions that expired while offline surface as `account-session-expired` events and re-auth jobs (§7.5).

**Loss statement (recorded honestly):** losing the vault and its keys is a **logout for everyone**. There is no recovery path by design — the recovery key (§7.6) recovers a user's *data*, not the instance's vault. This is why vault backup is the operational discipline.

## 3. Key rotation

| Key | Rotation trigger | Effect | Procedure |
| --- | --- | --- | --- |
| **Host relay key** | Compromise suspicion, key-custodian change, or policy interval | Rotating it **kills every member relay at once** (§7.6) — members must re-link through the owner's session | Generate new relay key; re-encrypt each account session's session key under the new key; expect a wave of `account-session-expired` events and re-auth jobs; schedule during low-usage window |
| **Owner KEKs** | Password change / recovery | Re-wraps the user's DEK; data survives (§7.6) | Handled by the client (password change) or recovery flow, not by ops |
| **Content-key cache entries** | License validity (TTL) | Entry dropped on decrypt failure; fail-fast re-license (§6.6) | Automatic; v2 |
| **Credential-set rotation (DRM devices)** | Rejection by license server | Automatic within the slot; `awaiting-action` only at exhaustion (§6.6) | v2 |

The relay-key rotation is the operation that can cause a visible outage for members — it is the one key rotation an operator should treat as a change window.

## 4. Monitoring and alerting

Monitoring is **content-blind**: metrics describe the pipe, never what flows through it (§1.3). All of the following are safe to record by identity-free volume/timing:

- **Queue and admission:** job queue depth, queue wait, busy-answer rate, per-resource cap utilization (concurrent sessions, transcodes, aggregate bandwidth) (§6.5).
- **Sessions:** active play/download sessions, heartbeat timeouts, revocation kills, session TTL expirations (§9.1).
- **Providers:** per-provider availability (degraded/rejected), sync cadence health, 429/error backoff state, per-provider aggregate governor utilization (§5.4, §7.2).
- **Stores:** vault backup success/freshness, cache rebuild duration, store failures.
- **Bandwidth:** aggregate in/out per session-class (play vs download) — volume and timing only.
- **Secrets:** alerts for any log line matching secret-pattern rules (mirrors the CI gate, CI-CD.md §4).

Alert on: vault backup failure or age, queue saturation, provider degraded for a sustained period, relay-key age, revocation failures (a failed kill is a policy breach, §7.1).

## 5. Capacity

PLAN.md §6.5 is **proxy-only**: all bytes cross the instance. Capacity planning is therefore about bandwidth and transcode CPU, not storage (media bytes are never cached, §2.4). With the instance-local disk sink (v1, TECHNICAL-DECISIONS.md §1.13), instance storage is capacity-planned again: disk-sink deliverables are user-requested downloads owned by the sink — not §2.4 caches — and their retention is a per-sink ops configuration. The device sink (browser download) leaves no media bytes on the instance.

- **Bandwidth:** a 1080p stream is roughly 4–8 Mbps; the instance must carry, per concurrent stream, that plus protocol overhead. Provision WAN/egress for `concurrent_streams × stream_bitrate`, not for the library size.
- **CPU:** passthrough/remux are cheap; transcode is the expensive path and is bounded by operator encode-resolution caps (§6.3). Cap concurrent transcodes explicitly (§6.5).
- **Concurrency:** set the instance-default policy and per-account caps (§7.2) so `min(policy, provider_cap)` and the concurrency limits keep the box and the providers under their ceilings.
- **Planning inputs to record per deployment:** expected concurrent streams, mix of play vs download, whether transcode is enabled at all (it can be disabled — §6.3).

## 6. Logging and audit

- Core and ops logs are content-blind: volume and timing, never item identity (§1.3). Search/enrichment query payloads must not reach logs.
- Audit (§2.4: minimal) records usage volume and timing; nothing about titles.
- Log retention and rotation are deployment choices; the content-blind rule is not.

## 7. Operational health of the instance

- A provider marked degraded is a live signal: confirm the process/network slot is down or non-conforming before deciding whether to fix or redeclare (§4 of PLAN.md).
- A failed claim (slot rejected) surfaces in the registry; the operator fixes or re-declares the slot — never a downgrade (§2.5).
- Upgrades: image-tagged releases (CI-CD.md §5); graceful drain applies to slot reloads (§4 of PLAN.md) — never kill live sessions on reload.

## 8. Relationship to the other documents

- **PLAN.md §2.4, §7.6** — durability classes and trust model this document operationalizes (vault backup, relay-key rotation).
- **THREAT-MODEL.md** — the threats (T2 relay key, T9 operator, T12 logging) this document's procedures mitigate.
- **ENVIRONMENT.md / CI-CD.md** — dev and build; this document is the run side of the same artifact.
- **TECHNICAL-DECISIONS.md** — the deployment-relevant choices (language, transport, store backends).
