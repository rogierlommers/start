# Project AI Skill: start Backend Maintainer

## Mission

You are working on `start`, a Go backend for a personal home dashboard.

Your goals:

1. Keep the service reliable and secure.
2. Preserve existing behavior unless a change is explicitly requested.
3. Prefer small, reviewable changes with clear reasoning.
4. Keep code and docs maintainable for a single-operator personal project.

## Product Context

The backend serves:

1. HTML views and JSON APIs.
2. Authentication/login.
3. Bookmarks feature.
4. Storage/uploads endpoints.
5. Quick note sending by email (text and file attachments).
6. URL collection with title scraping and personal RSS feed publishing.

Core stack:

1. Go
2. Gin
3. SQLite

## Working Style

1. Read relevant files first; do not guess behavior.
2. Use idiomatic Go naming and structure.
3. Keep functions focused and avoid unnecessary abstractions.
4. Add concise comments only for non-obvious logic.
5. Update docs when behavior or configuration changes.

## Safety and Security Rules

1. Validate and sanitize all user input, especially file uploads and URL ingestion.
2. Use parameterized SQL queries; never build SQL with untrusted input.
3. Protect auth/session boundaries and avoid leaking sensitive details in errors.
4. Treat email, RSS, and scraper inputs/outputs as untrusted data.
5. Limit file handling risk: validate size, content type, and storage path.
6. Do not introduce secrets into source code, logs, tests, or docs.

## Data and Migration Discipline

1. Prefer additive schema changes.
2. If a migration is needed, include a forward-safe path and note rollback impact.
3. Keep SQLite compatibility in mind (types, constraints, transactions).
4. Preserve existing data semantics and timestamps.

## API and UX Contract

1. Keep error responses consistent and actionable.
2. Preserve existing route behavior when possible.
3. When breaking changes are unavoidable, document them clearly in README/changelog notes.

## Performance and Reliability

1. Avoid blocking operations in hot request paths when possible.
2. Add timeouts and cancellation for outbound operations (email, HTTP scraping).
3. Handle partial failures gracefully and return clear status codes.
4. Log enough context for debugging without exposing private data.

## Testing and Verification

Before finalizing changes, run the best available checks for the modified scope:

1. `go test ./...`
2. `go test -race ./...` for concurrency-sensitive changes (when practical).
3. `go vet ./...`
4. `gofmt` on changed files.

If a tool is unavailable or a check cannot run, state that clearly and explain impact.

## Change Delivery Checklist

For every non-trivial change:

1. Explain what changed and why.
2. List any API, DB, config, or behavior impacts.
3. Note risks and edge cases considered.
4. Identify follow-up tasks if needed.

## Preferred Response Format for AI Assistants

1. Start with a short outcome summary.
2. Provide concrete file-level changes.
3. Include commands run and key results.
4. End with optional next steps (numbered).

## Out of Scope Defaults

Unless requested, do not:

1. Rewrite large modules that are unrelated to the task.
2. Introduce heavy new dependencies.
3. Change auth/session behavior or storage paths broadly.
4. Alter HTML/API contracts in ways that break existing dashboard usage.
