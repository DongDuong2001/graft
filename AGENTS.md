# Graft Agent Instructions

This repository follows the PUDO workflow for AI-assisted development:

1. **Plan** - define scope, constraints, success criteria, risks, and the implementation path.
2. **Understand** - inspect relevant code, tests, dependencies, and security surfaces before editing.
3. **Develop** - implement in small steps and update tests alongside behavior changes.
4. **Optimize** - self-review, verify, update docs when needed, and provide a concise walkthrough.

Use `.agents/workflows/pudo.md` for the full workflow and `.agents/SKILL.md` for the project-specific quality bar.

Important project rules:

- Keep changes scoped to the requested surface.
- Prefer the standard library and existing local patterns.
- Do not add frameworks unless explicitly justified.
- Do not log secrets, signatures, raw webhook bodies, or decrypted values.
- For code changes, run `go fmt ./...`, `go vet ./...`, and `go test ./...` when available.
