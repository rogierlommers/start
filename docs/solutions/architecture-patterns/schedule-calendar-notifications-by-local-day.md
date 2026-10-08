---
title: Schedule calendar notifications by local day
date: 2026-10-08
last_updated: 2026-10-08
category: architecture-patterns
module: incident manager notifications
problem_type: architecture_pattern
component: background_job
severity: medium
applies_when:
  - A background job reports on a future calendar day at a configured local time
  - An iCalendar feed may contain recurring events rather than expanded instances
tags: [icalendar, scheduling, timezones, recurrence, background-jobs]
---

# Schedule calendar notifications by local day

## Context

A daily calendar notification has two separate time concerns: when the job runs and which local day it reports. Treating either as a fixed 24-hour duration causes drift across daylight-saving changes. Treating every `VEVENT` as a concrete occurrence misses feeds that represent a duty rotation with `RRULE` and `EXDATE`.

## Guidance

Compute each next run as a local wall-clock timestamp in the business timezone. In this app, incident-manager notifications consistently use `Europe/Amsterdam`. After a run, construct the next day's local midnight boundaries with calendar arithmetic, not `24*time.Hour`:

Bundle Go's `time/tzdata` package when the deployment image may not include an operating-system timezone database. A timezone constant alone does not make `time.LoadLocation` self-contained in a minimal container.

```go
dayStart := time.Date(year, month, day, 0, 0, 0, 0, location)
dayEnd := dayStart.AddDate(0, 0, 1)
```

Parse iCalendar timestamps in their declared `TZID`, falling back to the notification timezone for floating times and all-day values. Expand recurring events only around the target window, preserving each event's duration and excluding matching `EXDATE` values. Then select events with interval overlap rather than equal dates:

```go
event.Start.Before(dayEnd) && event.End.After(dayStart)
```

Keep feed retrieval bounded separately from parsing. `internal/service/incident_manager.go` uses an HTTP timeout, rejects non-success responses, and limits the response body before parsing. A malformed or non-calendar response must fail the run instead of becoming a misleading “no event found” email.

When the same calendar powers more than one presentation, expose normalized occurrences from one service path. The email notification and read-only homepage overview both use the same bounded fetch, recurrence expansion, cancellation filtering, overlap rule, and chronological sort; the API and browser only map and present those results.

## Why This Matters

Calendar feeds commonly mix UTC, named timezones, floating local timestamps, all-day values, and recurrence rules. Keeping schedule calculation, target-day boundaries, recurrence expansion, and overlap filtering explicit prevents notifications from moving by an hour, selecting the wrong day, or silently omitting a recurring duty assignment.

## When to Apply

- Daily or weekly reports whose meaning follows a business timezone.
- Calendar-backed reminders that must include overnight or multi-day events.
- Subscribed feeds from providers that publish recurrence masters instead of expanded occurrences.

## Examples

The incident-manager worker calculates its next wall-clock run with `nextDailyRun`, expands recurrences with `occurrencesBetween`, and has regression coverage for a daylight-saving transition, a weekly rule, and an excluded occurrence in `internal/service/incident_manager_test.go`.

## Related

- Runtime configuration and operator setup are documented in `README.md`.
