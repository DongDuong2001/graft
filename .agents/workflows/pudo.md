---
description: PUDO workflow for AI-assisted work in Graft: Plan, Understand, Develop, Optimize.
---

# PUDO Workflow

PUDO is the default workflow for meaningful changes in this repository. It keeps AI-assisted work structured and makes larger updates easier to review.

Use the cycle:

1. Plan
2. Understand
3. Develop
4. Optimize

For tiny tasks, compress the phases into a short note. For medium and large tasks, make the phases explicit and track progress.

## 1. Plan

Define what will change before touching code.

Checklist:

- State the goal and user-visible outcome.
- Identify in-scope and out-of-scope work.
- Define success criteria.
- Name constraints such as compatibility, security, storage migrations, and test coverage.
- Draft the implementation path and call out risks.

Exit criteria:

- The scope is specific enough to implement.
- The expected verification commands are known.
- Unknowns are either resolved or listed as assumptions.

## 2. Understand

Read the relevant code before editing.

Checklist:

- Locate the affected packages, handlers, models, storage queries, config, docs, and tests.
- Follow existing patterns for constructors, DTOs, validation, errors, and JSON field names.
- Map the dependency path from HTTP/API entry point to storage or engine behavior.
- Check security-sensitive surfaces: admin auth, webhook signatures, secrets, rate limits, body handling, and audit logging.
- Identify existing tests that should be updated.

Exit criteria:

- The blast radius is clear.
- The local pattern to follow is known.
- Any plan changes are surfaced before coding.

## 3. Develop

Implement in small, reviewable steps.

Checklist:

- Update the task checklist as work progresses.
- Keep edits scoped to the planned area.
- Add or update tests alongside behavior changes.
- Use `apply_patch` or focused edits; avoid unrelated refactors.
- If implementation reveals a bad assumption, loop back to Plan or Understand.

Exit criteria:

- Code is implemented.
- Tests or docs matching the change are updated.
- No unrelated files were changed.

## 4. Optimize

Review before closing the task.

Checklist:

- Run `go fmt ./...`, `go vet ./...`, and `go test ./...` for code changes when available.
- Self-review for readability, naming, duplication, error handling, and security.
- Confirm documentation is updated when behavior, APIs, workflows, or release expectations change.
- Summarize what changed, why, and any remaining risk.

Exit criteria:

- Verification has passed, or any skipped/failed checks are explained.
- The final answer includes a concise walkthrough.

## Phase Loops

PUDO is a cycle. Return to an earlier phase when new information changes the work:

- Understand -> Plan when the original scope is wrong.
- Develop -> Understand when code behavior is unclear.
- Develop -> Plan when the approach is not feasible.
- Optimize -> Develop when review finds a bug.
- Optimize -> Plan when the solution needs a larger rethink.

## Graft-Specific Notes

- Read `.agents/SKILL.md` first for the project quality bar.
- Use `.agents/docs/FEATURE-TEST-MATRIX.md` to decide where tests belong.
- Follow `.agents/workflows/test-and-lint.md` for verification.
- Security-sensitive changes must also follow `.agents/docs/security-guidelines.md`.
