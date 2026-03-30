---
description: Reviews Git commit messages to ensure they follow Conventional Commits
---

# Git Commit Review Workflow

This workflow is used to review the uncommitted changes and draft a Git commit message, or to review a requested commit message for compliance with the project's Conventional Commits standard.

## 1. Analyze the Changes
First, use the `run_command` tool to run:
```bash
git status
git diff --cached
git diff
```
Understand the scope of what has been changed. If changes include multiple logical boundaries, suggest that the user splits them into multiple commits.

## 2. Enforce Conventional Commits Format
Ensure the commit message follows this structure:
`<type>(<scope>): <subject>`

### Valid Types:
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `style`: Changes that do not affect the meaning of the code (formatting, missing semi-colons, etc.)
- `refactor`: A code change that neither fixes a bug nor adds a feature
- `perf`: A code change that improves performance
- `test`: Adding missing tests or correcting existing tests
- `chore`: Changes to the build process or auxiliary tools

### Scope (Optional but Recommended):
The scope should refer to the package or feature affected (e.g., `api`, `storage`, `transformer`, `agent`).

### Subject:
- Must be written in the imperative mood ("add feature" not "added feature").
- Do not capitalize the first letter.
- No period `.` at the end.

## 3. Formatting the Output
If you are generating a commit message for the user, provide it explicitly in a code block. If you are reviewing a user's proposed commit, explicitly state passes/fails for each criteria.

Example Output:
```
feat(storage): add sqlite migration support

- Adds automatic schema migrations on startup
- Fixes initialization race condition
```
