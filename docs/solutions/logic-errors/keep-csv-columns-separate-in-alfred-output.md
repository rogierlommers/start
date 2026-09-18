---
title: Keep CSV columns separate in Alfred bookmark output
date: 2026-09-18
category: logic-errors
module: bookmarks API
problem_type: logic_error
component: api_layer
symptoms:
  - CSV-derived Alfred titles combined the label and URL columns
root_cause: logic_error
resolution_type: code_fix
severity: low
tags: [bookmarks, alfred, csv, api-response]
---

# Keep CSV columns separate in Alfred bookmark output

## Problem

Additional bookmarks are stored as two-column CSV rows. The Alfred response combined both columns into `title`, even though the URL already had its own `arg` field.

## Symptoms

- A row such as `"docs","https://example.com"` produced the title `docs - https://example.com`.

## What Didn't Work

- Formatting CSV rows like database bookmarks obscured the CSV contract. Database bookmarks may compose tags and titles, but CSV rows already map directly from two input columns to two Alfred fields.

## Solution

In `internal/httpapi/bookmarks.go`, trim both CSV values and map them without concatenation:

```go
tag := strings.TrimSpace(record[0])
url := strings.TrimSpace(record[1])

item := alfredBookmarkItemResponse{
	Title: tag,
	Arg:   url,
}
```

Regression coverage in `internal/httpapi/handlers_test.go` verifies the helper and HTTP response use only column one for `title`, while column two remains `arg`.

## Why This Works

The output now preserves the intended one-to-one field mapping: CSV column one is the Alfred display title and CSV column two is the actionable URL. Regular database bookmarks continue using their separate title-formatting path.

## Prevention

- When an import format has positional columns, assert each output field independently and include a negative assertion against accidental concatenation.

## Related Issues

- The current endpoint contract is summarized in `README.md`.
