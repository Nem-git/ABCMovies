# Decisions

Durable decision records for ABCMovies.

- Product/product-boundary decisions: `product/`
- Technical/implementation decisions: `tech/` (kept out of product docs)

## Rules

- One decision per file. IDs are never reused.
- Use `template.md`. Status: Proposed, Accepted, Superseded, Rejected, Withdrawn.
- GitHub issues/discussions hold discussion and status; the accepted ADR holds the durable outcome. Link both ways.
- Accepted decisions change only by a superseding ADR.

## Index

| ID | Title | Type | Status | GitHub |
|---|---|---|---|---|
| 0000 | Open items carried from prior plan | Product | Proposed | — |
| tech/0001 | Core language and runtime | Technical | Accepted | #22 |
| tech/0002 | Contract/schema format | Technical | Proposed (migration draft) | #22 |
| tech/0003 | API and browser transport | Technical | Proposed (migration draft) | #22 |
| tech/0004 | Storage baseline | Technical | Accepted | #22 |
| tech/0005 | First integrations | Technical | Accepted | #22 |
| tech/0006 | Repository layout | Technical | Accepted | #22 |
| tech/0007 | CI and dependency updates | Technical | Accepted | #22 |
| tech/0008 | Config and secrets | Technical | Accepted | #22 |
| tech/0009 | Frontend packaging | Technical | Proposed (migration draft) | #22 |
| tech/0010 | Frontend/core split — gRPC core, HTTP session edge | Technical | Accepted | #34 |
| tech/0011 | Store layout | Technical | Accepted | #25 |
| product/0001 | First-run admin, signup/invites, and recovery | Product | Accepted | #16 |
| product/0002 | Source/server identity (Jellyfin-first) | Product | Accepted | #15 |
| product/0003 | DRM/protected-content stance | Product | Accepted (expected to be superseded later) | #17 |
| product/0004 | Source library freshness | Product | Accepted | #25 |
