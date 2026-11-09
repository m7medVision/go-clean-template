# Spec: product analytics in PostHog

Module id: `posthog` · third of four (see [README.md](README.md#observability-and-messaging)) ·
Depends on `nats-queue`, `task-attributes` · Status: **spec written 2026-11-08, awaiting review**

## Objective

Product sees what users do, in PostHog, without the template storing analytics or leaking PII.
Optional: with no PostHog key, nothing is captured and nothing else changes. Done means:

- seven typed events reach PostHog, enough for an activation funnel
  (`user_registered` → `task_created` → `task_status_changed` to done);
- the web client sends page views, identifies the user after sign-in, and records sessions with
  every input and all task text masked;
- no event carries an email, a name, task text or an amount, and the types make that impossible.

## Design (decided)

1. **PostHog stores; we only send.** No analytics in Postgres or Mongo.
2. **A typed catalogue** in `internal/entity`: one struct per event with typed properties,
   `entity.AnalyticsEvents()`, a docs table, a test that every mapped domain event is listed. No
   `map[string]any`, no free-form `Capture`. The web client takes its names from one file,
   `web/src/lib/analytics-events.ts`.
3. **Launch events:**

   | Event | Source | Properties |
   | --- | --- | --- |
   | `user_registered` | `UserRegistered` | `method` |
   | `user_logged_in` | the sign-in command | `method` |
   | `task_created` | `TaskCreated` | — |
   | `task_status_changed` | `TaskStatusChanged` | `from`, `to` |
   | `task_attributes_updated` | `SetTaskAttributes` (no event) | `fields` (names, not values) |
   | `currency_converted` | `CurrencyConverted` | `from`, `to` (currencies, not amount) |
   | `$pageview` | web, every route change | `path` |

4. **Delivery through the queue.** Domain events: an event handler writes the outbox. The two
   command-side events: the command writes the outbox through a `repo.Analytics` port in its
   transaction. The worker's `analytics` topic sends to PostHog (`posthog-go`); give-up drops the
   event and logs it.
5. **`distinct_id` is the user ID** on both sides, so server and web events join.
6. **Optional by presence:** `POSTHOG_API_KEY` and `POSTHOG_HOST` for the worker; without them the
   `analytics` topic is not registered and the outbox writes for it are skipped by the port.
   `VITE_POSTHOG_KEY` / `VITE_POSTHOG_HOST` for the web; without them `posthog-js` is not loaded.
7. **No consent banner and no GDPR delete** in this module.

## Project structure

| Where | Change |
| --- | --- |
| `internal/entity/analytics.go` | the event types, the catalogue, the queue topic |
| `internal/repo/analytics.go` | `Analytics` port (+ `go:generate`) |
| `internal/repo/analytics/` | the outbox-writing implementation; the PostHog sender the worker uses |
| `internal/usecase/{user,task,exchange}/event/capture_*.go` | event handlers mapping domain events |
| `internal/usecase/user/command/log_in.go`, `task/command/set_task_attributes.go` | one capture each |
| `internal/controller/worker/v1/analytics.go` | the topic handler |
| `internal/app/*.go`, `config/`, `.env.example`, `docker-compose.yml` | wiring and settings |
| `web/src/lib/analytics.ts`, `analytics-events.ts`, `main.tsx`, `router.tsx`, `auth/` | init, page views, identify/reset, replay masking |
| `docs/analytics.md`, `docs/configuration.md`, `docs/worker.md`, `docs/web.md` | the catalogue and the rules |

## Testing strategy

- **Entity:** the catalogue test; no event type has a string field outside an allowed list.
- **Event handlers and commands:** against `memstore` / mocks: the right event, the right properties.
- **Worker handler:** a fake PostHog server: one request per event, a 4xx is permanent.
- **Web:** `analytics.test.ts`: no key, no load; page view on navigation; masking options set.
- **Integration:** none (decided 2026-11-08). No fake PostHog in the stack; the unit tests above are
  the coverage.

## Boundaries

- **Always:** user ID only; masked replay; the catalogue updated with every new event.
- **Ask first:** a new event; person properties beyond `signup_method`; surveys, experiments,
  group analytics.
- **Never:** PII in a property; sending from the app process directly; analytics stored by us.

## Success criteria

1. With the keys set, creating and finishing a task produces the funnel's three events for one
   person in PostHog, and page views for the same person.
2. With no keys, no outbox rows for `analytics`, no PostHog traffic, nothing else changes.
3. A replay shows no typed text.
4. Checks green; the catalogue test fails when an event is mapped but not listed.

## Open questions

1. Should `$pageview` paths drop IDs (`/tasks/:id`) to keep them PII-free and groupable?
