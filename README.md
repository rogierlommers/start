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
subject identifies the first matching duty as `📅 Today's IC: <calendar summary>`,
and the body includes each event's summary, time, location, and description when present.
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

## ING account balance

The homepage can show the available balances of personal ING Netherlands accounts through
[Enable Banking](https://enablebanking.com/). The integration is read-only and requests
balance access only. It does not receive ING login credentials or persist balances or
transactions. For each connected account, the SQLite database stores the Enable Banking
session ID, account ID, account label, currency, and consent expiry needed to refresh the
balance.

### Enable Banking setup

1. Create a production application in the
   [Enable Banking Control Panel](https://enablebanking.com/cp/applications). Production
   registration requires a description, GDPR contact email, privacy-policy URL, and
   terms-of-service URL. Activate personal testing by linking or whitelisting your own
   account in the Control Panel.
2. Register this exact HTTPS callback URL for the application:
   `https://your-dashboard.example/api/banking/callback`.
3. Store the downloaded RSA private key outside the repository and restrict its file
   permissions to the service account.
4. Configure the following environment variables:

```dotenv
ENABLE_BANKING_APPLICATION_ID='your-application-uuid'
ENABLE_BANKING_PRIVATE_KEY_PATH='/run/secrets/enable-banking.pem'
ENABLE_BANKING_CALLBACK_URL='https://your-dashboard.example/api/banking/callback'
ENABLE_BANKING_ASPSP_NAME='ING'
DATA_PROTECTION_EMAIL='you@example.com'
```

All three required values must be present or all must be empty. The callback must use
HTTPS. Enabling banking also requires `GUI_USERNAME`, `GUI_PASSWORD`, and a stable
`GUI_SESSION_SECRET` of at least 32 characters because the banking routes contain
sensitive financial data. `DATA_PROTECTION_EMAIL` must be the bare email address
published on the public `/privacy` and `/terms` pages. Register these URLs with Enable
Banking:

```text
https://your-dashboard.example/privacy
https://your-dashboard.example/terms
```

After restarting the service, sign in to the dashboard and select **Connect ING**. Enable
Banking redirects to ING for approval and then returns to the protected callback. The
application stores and displays every account returned by ING, in provider order.

Successful balance responses are cached independently in memory for five minutes. A
manual refresh bypasses that interval. If one account is temporarily unavailable after a
successful refresh, its card displays the cached value while the other accounts continue
to refresh. ING consent is requested for 180 days; the card prompts for reconnection after
expiry.

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
