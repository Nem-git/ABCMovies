# ADR 0001: First-run admin, signup/invites, and recovery

- Status: Accepted
- Type: Product
- Date: 2026-11-08
- Owners: TBD
- GitHub: #16
- Affects: REQ-IDN-01, REQ-SEC-01, Build 1 bootstrap, member management (Build 2), future signup option

## Context

Build 1 needs a safe way to start a self-hosted instance. There is no existing user database on first run. The product must avoid public signup by default, make the first admin explicit, and define what happens when the admin loses access.

## Constraints

- Self-hosted household/admin model.
- No public signup by default unless explicitly decided.
- No secrets in repo, logs, or UI.
- Recovery has trust implications: operator-assisted recovery is convenient but means the operator/host can reset access; user-held recovery material is stronger for user control but can be lost.
- Build 1 needs only enough identity to create/administer the first admin and log in.

## Decision

Accept **Option A for Build 1**: first-run setup creates the initial admin; local accounts; **no public signup by default**.

Record **Option C (public signup) as a required future option**, not part of Build 1/2. When implemented, it will need its own ADR covering: enable/disable setting, abuse/spam controls, verification/invite policy, admin moderation burden, rate limits, and how it interacts with self-hosted household defaults.

## Recovery sub-decision

Accept **R1 (operator-assisted reset) for Build 1**, with explicit safeguards and an honest trust boundary.

**Trust boundary (must be stated in docs/UI):** operator-assisted recovery means someone with control of the instance host/runtime can perform recovery. ABCMovies cannot cryptographically prove, from outside that boundary, that "the real admin requested it" unless a prior user-controlled factor exists (for example a recovery key). Build 1 does not require a recovery key, so the practical guarantee is **host/operator control**, not remote identity proof.

**Safeguards to reduce accidental or casual misuse:**

- Recovery is **not available from the normal login UI** in Build 1.
- Recovery requires **local host/operator access** (for example a CLI/admin command run where the instance is installed, or an equivalent operator-only maintenance path chosen during implementation).
- The action should require an explicit confirmation step and affect only the targeted local admin credential/access path (never reveal existing secrets).
- The instance should record an **audit/event log entry** for the recovery action (without secrets) so operators can detect unexpected resets.
- The runbook must warn: protect host/OS access, backups, and instance config; anyone with sufficient host access can reset admin access.
- Optional later hardening (not required for Build 1): a break-glass token generated at setup and stored offline by the admin; time-delayed reset with out-of-band notice if contact channels exist; recovery key (R2) as a user-controlled option.

**Not chosen now:** R2 recovery key may be added later as a stronger user-controlled option, with clear warnings about loss.

## Consequences

- Build 1 is bootstrappable and safe by default.
- Public signup is explicitly deferred but planned as a configurable future capability.
- Recovery is practical but not zero-trust; the product must be honest about that boundary.
- Implementation must avoid exposing secrets and must document the operator recovery procedure.

## Follow-up

- Implement Build 1 bootstrap/login path per this ADR.
- Document operator recovery procedure without exposing secrets.
- Open a future ADR for public signup controls when scheduled.
- Revisit R2 recovery key if user-controlled admin recovery becomes a priority.

## Outcome

**Accepted** (A + R1) — first-run admin creates initial admin; no public signup by default; operator-assisted recovery (R1) with safeguards and honest trust boundary; Jellyfin source connection URL provided at runtime, not stored in env. Date: 2026-11-08. Owners: TBD (to be filled in implementation). Final recovery mechanism: operator-assisted reset via CLI/admin command with explicit confirmation and audit log entry. Affected issues/docs: #16, DEC-009 draft, foundation plan (#20/#21).