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

Create a Basic auth header from your configured API credentials:

`echo "Authorization: Basic $(printf '%s:%s' "api_user" "api_password" | base64)"`

If `GUI_SESSION_SECRET` is not set, the server falls back to an in-memory random secret and existing login sessions are invalidated on each restart.

## AI Skill

This repository includes a project-specific AI maintenance skill in [.github/copilot-instructions.md](.github/copilot-instructions.md).

Use it to guide AI assistants when adding features, fixing bugs, reviewing code, or maintaining the Go + Gin + SQLite backend.

## OpenAPI

The generated API specification is available at [swagger-docs/swagger.yaml](swagger-docs/swagger.yaml).

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

### Incident manager notification

The service can download an iCalendar (`.ics`/`.ical`) feed once per day and email
the `VEVENT` entries that overlap the following day to `MAILER_EMAIL_WORK`.

- `INCIDENT_MANAGER_ICAL_URL` enables the notification worker and contains the HTTP(S) feed URL.
- `INCIDENT_MANAGER_NOTIFICATION_TIME` sets the daily local send time in `HH:MM` format (default `17:00`).

The notification schedule and target calendar day always use the
`Europe/Amsterdam` timezone.

The feed URL may contain a private calendar token and should be treated as a secret.
All-day, timed, and recurring (`RRULE`/`EXDATE`) events are supported; the email
includes each event's summary, time, location, and description when present.
Cancelled events are omitted. The worker is disabled when
`INCIDENT_MANAGER_ICAL_URL` is empty.

The protected `GET /api/incident-manager` endpoint returns duties overlapping
today and the next 13 days. The homepage exposes the same read-only overview in
the **Incident Manager** tab next to **Reading List**, including explicit empty
and feed-error states.

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
- `GET /api/bookmark-csv` (additional bookmark CSV text)
- `PUT /api/bookmark-csv` (save additional bookmark CSV text)
- `GET /api/bookmarks/alfred` (Alfred workflow format with regular bookmarks and valid additional CSV rows; for CSV rows, `title` is the trimmed first column and `arg` is the trimmed second column; regular bookmarks also include `id`; supports `?include_hidden=true`)
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
- `GET /api/reading-list/bookmarklet-input?url={encodedUrl}`

Reading-list cleanup environment variables:

- `READING_LIST_CLEANUP_DAYS` (optional, defaults to `30`; set to `0` to disable scheduled cleanup)

The bookmarklet endpoint adds the incoming `url` as a new reading-list item and displays a confirmation page with the saved site's details. It does not redirect to the saved site.

Bookmarklet one-liner:

`javascript:(()=>{const cur=location.href;location.href='http://127.0.0.1:3000/api/reading-list/bookmarklet-input?url='+encodeURIComponent(cur)+'&_='+Date.now()})()`
