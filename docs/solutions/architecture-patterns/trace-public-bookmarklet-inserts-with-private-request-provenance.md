---
title: Trace public bookmarklet inserts with private request provenance
date: 2026-11-03
last_updated: 2026-11-03
category: architecture-patterns
module: reading list
problem_type: architecture_pattern
component: api_layer
severity: medium
applies_when:
  - Diagnosing repeated reading-list URLs in a public RSS feed
  - Capturing request provenance for a public ingestion endpoint
tags: [reading-list, rss, bookmarklet, diagnostics, privacy]
---

# Trace public bookmarklet inserts with private request provenance

## Context

Several reading-list RSS items can have the same URL and GUID but different publication dates. The RSS handler is read-only: it lists database rows, so distinct dates indicate distinct inserts rather than a feed reader inventing new records. The public bookmarklet GET route inserts a row on every visit; a browser retry, visited bookmarklet URL, or other requester could have produced the additional visits, but the historical rows alone cannot identify which one did.

## Guidance

Capture provenance at the request boundary of **both** add routes, before calling the service: the network peer from `Request.RemoteAddr`, a bounded user agent, and only the origin of the optional referrer. Carry it through the service into SQLite alongside the URL and timestamp. Include the diagnostics in the reading-list JSON endpoint and show them behind an expandable detail in the dashboard (both protected when GUI authentication is configured); keep them out of the public RSS feed and bookmarklet confirmation page. The additive migration in `internal/repository/sqlite_migrations.go` gives existing rows empty fields rather than guessing historical values. The dashboard renders an explicit no-details state for those earlier rows rather than implying the missing fields identify a requester.

Do not treat arbitrary `X-Forwarded-For` values as client identities. `RemoteAddr` is the directly connected peer; behind a reverse proxy it can be the proxy address. To identify original clients there, correlate timestamps with the proxy's access logs or configure a trust boundary explicitly. A user agent or referrer may be missing or fabricated; even a real IP may represent a shared connection.

## Why This Matters

RSS items are public, while IPs and browser details can identify visitors. Putting diagnostics in RSS would turn an investigation aid into a privacy leak. Configure GUI authentication before exposing the JSON API to the internet; without it, that endpoint is not private either. Capturing diagnostics only when new rows are created also makes clear why a migration cannot answer who created earlier duplicates.

## When to Apply

- When the same URL appears multiple times in the feed, compare its `pubDate` values and stored rows before blaming the RSS reader.
- When adding HTTP provenance to any public write path, decide separately which authenticated read path may expose it.

## Examples

The reading-list request handler extracts the connection peer with `net.SplitHostPort(r.RemoteAddr)` and stores the referrer as `scheme://host` rather than persisting query strings that might contain tokens. `/api/reading-list/items` includes `client_ip`, `user_agent`, and `referer`; the Reading List tab renders those untrusted strings with HTML escaping in a collapsible **Request details** section. `/api/reading-list/rss` continues to use only the URL, title, and creation time.

## Related

- `README.md` documents the current response fields, trust limitations, and historical backfill behavior.
