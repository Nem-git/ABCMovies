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
| product/0001 | First-run admin, signup/invites, and recovery | Product | Accepted | #16 |
| product/0002 | Source/server identity (Jellyfin-first) | Product | Accepted | #15 |
| product/0003 | DRM/protected-content stance | Product | Accepted (expected to be superseded later) | #17 |
