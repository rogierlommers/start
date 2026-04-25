# START

A go backend for a home dashboard: serves HTML and APIs for login, bookmarks and storage/uploads with Gin + SQLite. It also sends quick notes by email (text/files) and includes a URL collector that scrapes titles and publishes a personal RSS feed.

## GUI Login

When `GUI_USERNAME` and `GUI_PASSWORD` are configured, the HTML dashboard, `/docs`, and the JSON API require a GUI login.
The health check endpoint at `/api/health`, reading-list RSS feed at `/api/reading-list/rss`, and bookmarklet input endpoint at `/api/reading-list/bookmarklet-input` remain public.

Login routes:

- `GET /login`
- `POST /login`
- `POST /logout`

GUI authentication environment variables:

- `GUI_USERNAME`
- `GUI_PASSWORD`
- `GUI_SESSION_SECRET` (recommended; set this to a stable secret of at least 32 characters to keep login sessions valid across server restarts)

API basic auth environment variables (for `curl`/non-browser API access):

- `API_USERNAME`
- `API_PASSWORD`

If `API_USERNAME` and `API_PASSWORD` are set, protected API routes also accept HTTP Basic auth.

If `GUI_SESSION_SECRET` is not set, the server falls back to an in-memory random secret and existing login sessions are invalidated on each restart.

## AI Skill

This repository includes a project-specific AI maintenance skill in [.github/copilot-instructions.md](.github/copilot-instructions.md).

Use it to guide AI assistants when adding features, fixing bugs, reviewing code, or maintaining the Go + Gin + SQLite backend.

## OpenAPI

The generated API specification is available at [docs/swagger.yaml](docs/swagger.yaml).

Regenerate it from handler annotations with:

`just generate-openapi`

Interactive API reference UI is available at `/docs` (served via gin-openapi).

## Mailer API

JSON-only endpoint:

- `POST /api/mail/send`

Request body:

```json
{
	"to": "person@example.com",
	"subject": "Quick note",
	"body": "Hello from start"
}
```

SMTP environment variables:

- `SMTP_HOST`
- `SMTP_PORT` (optional, defaults to `587`)
- `SMTP_USERNAME` (optional)
- `SMTP_PASSWORD` (optional)
- `SMTP_FROM`

If `SMTP_HOST` or `SMTP_FROM` are not configured, the mail endpoint returns `503`.

## Storage Upload API

- `POST /api/storage/upload` (multipart form field: `file`)
- `POST /api/storage/uploads` (multipart form field: `files`, repeat for multiple files)
- `GET /api/storage/files` (list uploaded files)
- `GET /api/storage/files/{filename}` (download a specific uploaded file)

Storage environment variables:

- `STORAGE_UPLOAD_DIR` (optional, defaults to `uploads`)
- `STORAGE_MAX_UPLOAD_MB` (optional, defaults to `100`)
- `STORAGE_CLEANUP_DAYS` (optional, defaults to `30`; set to `0` to disable scheduled cleanup)

## Bookmarks API

- `GET /api/bookmarks`
- `GET /api/bookmarks/alfred` (Alfred workflow format with `cache.seconds` and `items[]` containing `uid`, `id`, `title`, `arg`; supports `?include_hidden=true`)
- `POST /api/bookmarks`
- `PATCH /api/bookmarks/{id}`
- `PATCH /api/bookmarks/{id}/hidden`
- `PATCH /api/bookmarks/reorder`
- `DELETE /api/bookmarks/{id}`

## Database

Persistence is backed by SQLite.

Database environment variables:

- `SQLITE_PATH` (optional, defaults to `start.db`)

On startup, the backend automatically applies lightweight, versioned SQLite migrations.
Applied migration versions are tracked in the `schema_migrations` table.

## Reading List Bookmarklet

Reading-list endpoints:

- `POST /api/reading-list/items`
- `GET /api/reading-list/items`
- `GET /api/reading-list/rss`
- `GET /api/reading-list/bookmarklet-input?url={encodedUrl}&return_to={encodedUrl}`

Reading-list cleanup environment variables:

- `READING_LIST_CLEANUP_DAYS` (optional, defaults to `30`; set to `0` to disable scheduled cleanup)

The bookmarklet endpoint adds the incoming `url` as a new reading-list item.
If `return_to` is provided, it redirects back to that URL after saving.

Bookmarklet one-liner:

`javascript:(()=>{const cur=location.href;location.href='http://127.0.0.1:3000/api/reading-list/bookmarklet-input?url='+encodeURIComponent(cur)+'&return_to='+encodeURIComponent(cur)+'&_='+Date.now()})()`
