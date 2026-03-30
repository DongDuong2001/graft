# Contributing to Graft

First off, thank you for considering contributing to Graft! It's people like you that make Graft a great webhook bridge.

The following is a set of guidelines for contributing to Graft. These are **strict rules** meant to ensure the security, quality, and maintainability of the project.

## 1. Branching Strategy

- **`main` is protected:** You cannot push directly to `main`. All changes must go through a Pull Request.
- **Feature Branches:** Create a branch for your work from `main`. Use a descriptive name, prefixed with the type of work:
  - `feat/your-feature-name`
  - `fix/your-bugfix-name`
  - `docs/your-doc-update`
  - `chore/your-maintenance-task`

## 2. Commit Message Convention

We enforce the [Conventional Commits](https://www.conventionalcommits.org/) specification strictly. This enables automated changelog generation.

### Format:
`<type>(<scope>): <subject>`

### Allowed Types:
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `style`: Changes that do not affect the meaning of the code (formatting, missing semi-colons, etc.)
- `refactor`: A code change that neither fixes a bug nor adds a feature
- `perf`: A code change that improves performance
- `test`: Adding missing tests or correcting existing tests
- `chore`: Changes to the build process or auxiliary tools

*Example: `feat(api): add new webhook validation rule`*

## 3. Pull Request Requirements

Before creating a PR, ensure the following criteria are met. **PRs failing these checks will be blocked**:

1. **Passes Formatting:** Code must be formatted using `go fmt ./...`.
2. **Passes Linting:** Code must pass `go vet ./...` without warnings.
3. **Passes Tests:** All unit and integration tests must pass (`make test` or `go test -race ./...` with `CGO_ENABLED=1`).
4. **Maintains Coverage:** No PR should reduce the overall code coverage. Tests **must** be added for new features or bug fixes. Please consult `.agents/docs/FEATURE-TEST-MATRIX.md` for guidance on tests.
5. **Clear Description:** The PR description must clearly explain *what* is being changed and *why*. Link any relevant issues.

## 4. Coding Standards

Graft follows specific architectural and style choices. By contributing, you agree to adhere to these:

- **Standard Library Preference:** We lean heavily on the Go standard library (e.g., `net/http`). Do NOT introduce large external web frameworks (like Gin, Echo, or Fiber) unless explicitly discussed and approved.
- **Dependency Injection:** Use manual dependency injection (e.g., in `internal/app/app.go`).
- **Database:** SQLite is the only supported DB engine right now. Schema changes must update `internal/storage/sqlite.go`.
- **Error Handling:** Always check for `err != nil`. Wrap errors using `%w` (e.g., `fmt.Errorf("context: %w", err)`) to preserve error chains.

*For more details, see [`.agents/docs/coding-standards.md`](.agents/docs/coding-standards.md).*

## 5. Security Requirements

Security is paramount for this project.

- **No Raw Logging:** Never log raw webhook bodies, API keys, or decrypted secrets.
- **Admin Endpoints:** Any new admin functionality must be placed under `/api/v1/` and secured using `middleware.AdminAuth`.
- **Data at Rest:** Any sensitive configuration or destination secrets must be encrypted using the `MasterKey` in the storage layer.

*For full security guidelines, see [`.agents/docs/security-guidelines.md`](.agents/docs/security-guidelines.md).*

## 6. How to Contribute

1. **Fork the repo** and create your branch from `main`.
2. **Make your changes**, adhering to the standards above.
3. **Add or update tests** to cover your changes.
4. **Ensure the CI suite passes locally** (`make test`, `make vet`, `go fmt`).
5. **Open a Pull Request** with a detailed description.

By contributing, you agree that your contributions will be licensed under the MIT License of this project.
