# Quality

## Definition of Ready (issue/slice)

- Problem/outcome is clear.
- Requirement IDs and/or decision IDs are linked.
- Acceptance examples cover happy path, failure/empty state, and permission case where relevant.
- Out of scope is stated.
- Blocking decisions are identified.

## Definition of Done

- Acceptance examples pass.
- Negative/error cases are covered for anything that accepts input or can fail.
- Relevant docs are updated.
- No secrets in code, tests, fixtures, logs, screenshots, or examples.
- Automated checks run (CI/test recipe) when they exist; failures are fixed, not downgraded.

## Verification approach

- Prefer behavior tests: end-to-end journeys for user-visible flows, contract/integration tests for boundaries, focused unit tests for logic.
- Every user-visible capability needs at least one happy-path and one failure/permission example.
- Keep requirement IDs traceable from docs to issues/tests where practical.
