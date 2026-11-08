import type { ErrorEvent, EventHint } from "@sentry/react";

import { shouldReport } from "#/api/report";
import { tokenSubject } from "#/auth/roles";
import { auth } from "#/auth/token";

type Sentry = typeof import("@sentry/react");

let sentry: Sentry | undefined;
const _reported = new WeakSet<object>();

const withoutQuery = (url: string) => url.split("?")[0] ?? url;

const _urlKeys = ["url", "from", "to"];

function scrubUrls(data: Record<string, unknown>): Record<string, unknown> {
  const scrubbed = { ...data };
  for (const key of _urlKeys) {
    const value = scrubbed[key];
    if (typeof value === "string") scrubbed[key] = withoutQuery(value);
  }

  return scrubbed;
}

// scrubEvent keeps what a person typed or a token carried out of the report: query strings,
// console output, credentials and the user's address never leave the browser. It drops an event whose cause is not
// worth reporting (an unhandled rejection of a 401, say).
export function scrubEvent(event: ErrorEvent, hint?: EventHint): ErrorEvent | null {
  if (hint && "originalException" in hint && !shouldReport(hint.originalException)) return null;

  if (event.request?.url) event.request.url = withoutQuery(event.request.url);
  if (event.request) {
    delete event.request.query_string;
    delete event.request.cookies;
    delete event.request.data;
    delete event.request.headers;
  }
  if (event.breadcrumbs) {
    event.breadcrumbs = event.breadcrumbs
      .filter((crumb) => crumb.category !== "console")
      .map((crumb) => (crumb.data ? { ...crumb, data: scrubUrls(crumb.data) } : crumb));
  }
  if (event.user) event.user = { id: event.user.id };

  return event;
}

// initErrorTracking starts the SDK when a DSN was built in; without one it loads nothing and sends
// nothing.
export async function initErrorTracking(): Promise<void> {
  const dsn = import.meta.env.VITE_SENTRY_DSN;
  if (!dsn) return;

  const loaded = await import("@sentry/react");
  loaded.init({
    dsn,
    release: import.meta.env.VITE_RELEASE || undefined,
    environment: import.meta.env.MODE,
    beforeSend: scrubEvent,
  });
  sentry = loaded;
}

// reportError sends an unexpected failure once, with the signed-in user's id and nothing else.
export function reportError(error: unknown): void {
  if (!sentry || !shouldReport(error)) return;
  if (typeof error === "object" && error !== null) {
    if (_reported.has(error)) return;
    _reported.add(error);
  }

  const client = sentry;
  client.withScope((scope) => {
    const id = tokenSubject(auth.token());
    if (id) scope.setUser({ id });
    client.captureException(error);
  });
}
