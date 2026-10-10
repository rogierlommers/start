---
title: Use a licensed aggregator for personal PSD2 account data
date: 2026-10-10
last_updated: 2026-10-10
category: architecture-patterns
module: ING account balance
problem_type: architecture_pattern
component: service_layer
severity: high
applies_when:
  - A personal dashboard needs read-only balance or transaction data from a European bank
  - Direct bank production APIs require regulated third-party-provider credentials
retire_when: "ING permits private individuals to use its production Account Information API directly; check the ING PSD2 production guide and support eligibility before changing this design"
tags: [open-banking, psd2, ing, enable-banking, financial-data, oauth]
---

# Use a licensed aggregator for personal PSD2 account data

## Context

ING exposes balance and transaction APIs under PSD2, but its production onboarding is aimed at licensed third-party providers. A personal dashboard should not imitate that status, collect ING credentials, or scrape the banking website. As of 2026-10-10, Enable Banking permits a private individual to link personal accounts while Enable Banking acts as the authorized account-information provider.

## Guidance

Keep the bank-specific protocol behind the provider client defined in `internal/banking/client.go`. The service layer owns consent state, account selection, caching, and persistence; HTTP handlers only translate redirects and response models.

Request only the data the current UI uses. The account overview now sends `balances: true` and `transactions: true`; connections created by the earlier balance-only version are marked as needing renewed consent rather than repeatedly attempting unauthorized transaction calls.

Treat the authorization callback as a security boundary:

- Generate at least 256 bits of random state.
- Store state server-side with a short expiry and consume it exactly once.
- Require an HTTPS callback and the existing authenticated dashboard session.
- Keep the provider RSA key outside the repository and browser.

Persist only the references needed to resume access: provider session ID, account ID, display label, currency, consent expiry, and whether transaction permission was granted. Treat every account returned by an authorization session as part of one atomic replacement so reconnecting cannot leave removed accounts behind. Do not persist balances or transactions unless a later product requirement needs historical data. Cache each successful balance and recent-transaction response independently, use the same configured interval for cache freshness and background refreshes, and label only that account as stale when its provider refresh fails.

Store user-defined account aliases separately from replaceable session rows. Key aliases by Enable Banking's stable `identification_hash`, not its session account UUID, and expose only a SHA-256-derived account key to the browser. This keeps aliases through consent renewal without exposing the provider identifier in dashboard URLs.

## Why This Matters

The aggregator supplies the regulated production connection while the application retains a small, read-only integration surface. Provider isolation prevents PSD2 signing, redirect, and response details from leaking into handlers or the homepage. One-time callback state prevents login CSRF, and least-privilege consent limits the impact of either application or provider credential exposure.

## When to Apply

- Personal or single-operator tools reading PSD2 payment-account data.
- Banks whose production API requires AISP or TPP onboarding.
- Financial data that should remain usable during short provider outages without being permanently copied into the application database.

## Examples

`internal/banking/client.go` signs Enable Banking requests with RS256 and bounds provider responses. `internal/service/banking.go` manages one-time consent state, selects the available balance for every connected account, and serves independent balance and transaction caches per account. The cache and background refresh interval defaults to 60 minutes and is configured with `ENABLE_BANKING_CACHE_MINUTES`. Migration 5 backfills the former singleton connection into an ordered multi-account table; migration 6 records transaction-consent capability; migration 7 adds stable account identification and separately stored aliases without adding balance or transaction rows.

## Related

- Operator setup and current configuration are documented in `README.md`.
- Provider behavior was checked against the ING PSD2 production guide, DNB PSD2 guidance, and Enable Banking's API documentation and terms on 2026-10-10.
