# ABCMovies

A self-hosted media hub: one catalog to search and browse across many streaming services, and one place to watch, download, and manage access. ABCMovies aggregates your own legitimate access — it orchestrates, it does not invent content or bundle secrets.

## The plan

The project is specified before it is built. The documents are designed to be reusable references: PLAN.md fixes *what* the system is, and the implementation-specific choices are kept out of it.

| Document | Fixes |
| --- | --- |
| [PLAN.md](docs/PLAN.md) | The spec: what the project is, contracts, how parts work together; decision log in §11 |
| [IMPLEMENTATION.md](docs/IMPLEMENTATION.md) | How it is built and kept buildable; milestone roadmap M0–M9 |
| [ENVIRONMENT.md](docs/ENVIRONMENT.md) | What a developer machine must contain; reproducibility (local == CI) |
| [TESTING.md](docs/TESTING.md) | What "tested" means: fixture suites, test pyramid, vault/secrets suite |
| [CI-CD.md](docs/CI-CD.md) | How changes are integrated, verified, and shipped |
| [SCOPE.md](docs/SCOPE.md) | What v1 is (M0–M6), what is deferred, operator sign-offs |
| [TECHNICAL-DECISIONS.md](docs/TECHNICAL-DECISIONS.md) | The implementation choices the references stay agnostic about |
| [RESEARCH.md](docs/RESEARCH.md) | Feasibility evidence behind decisions (spike findings) |
| [THREAT-MODEL.md](docs/THREAT-MODEL.md) | What is protected, against whom, and the tests that verify it |
| [OPERATIONS.md](docs/OPERATIONS.md) | Running a live instance: deployment, vault, key rotation, capacity |

Implementation-specific choices (core language, API transport, tooling) live in **TECHNICAL-DECISIONS.md** — not in the reference documents, so those stay usable for anyone building a different implementation of the same spec.

## Getting started

1. Read PLAN.md §1 and SCOPE.md first — understand what this is and what v1 delivers.
2. Set up the environment: [ENVIRONMENT.md](docs/ENVIRONMENT.md).
3. Build and verify: run the standard verify recipe locally — CI runs the identical recipes ([ENVIRONMENT.md](docs/ENVIRONMENT.md) §5, [CI-CD.md](docs/CI-CD.md)).
4. Contribute per [CONTRIBUTING.md](CONTRIBUTING.md).

## Status

Planning is complete (Phase 0–3: decisions, scope, research scaffolding, threat model, ops). The project has not started implementation; work begins at milestone M0 (walking skeleton) per IMPLEMENTATION.md §2.

## License

AGPL-3.0 — see [LICENSE](LICENSE).
