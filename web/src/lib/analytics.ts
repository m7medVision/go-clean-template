import type { PostHog } from "posthog-js";

import { tokenSubject } from "#/auth/roles";
import { auth } from "#/auth/token";
import {
  analyticsEvents,
  type AnalyticsEvent,
  type AnalyticsProperties,
  type SignInMethodName,
} from "./analytics-events";

// The only file that knows the analytics vendor: switching it changes this file alone.

let posthog: PostHog | undefined;
let starting: Promise<void> | undefined;

const _vendorNames: Partial<Record<AnalyticsEvent, string>> = {
  [analyticsEvents.pageViewed]: "$pageview",
};

// initAnalytics starts PostHog when a key was built in; without one it loads nothing and sends
// nothing. Session replay masks every input and all text, so no typed or task text is recorded.
export function initAnalytics(): Promise<void> {
  const key = import.meta.env.VITE_POSTHOG_KEY;
  if (!key) return Promise.resolve();

  starting = import("posthog-js").then(({ default: loaded }) => {
    loaded.init(key, {
      api_host: import.meta.env.VITE_POSTHOG_HOST || "https://eu.i.posthog.com",
      capture_pageview: false,
      autocapture: false,
      person_profiles: "identified_only",
      session_recording: { maskAllInputs: true, maskTextSelector: "*" },
    });
    posthog = loaded;
    const id = tokenSubject(auth.token());
    if (id) loaded.identify(id);
  });

  return starting;
}

function whenReady(fn: (client: PostHog) => void): void {
  if (posthog) fn(posthog);
  else void starting?.then(() => posthog && fn(posthog));
}

export function capture<E extends AnalyticsEvent>(event: E, properties: AnalyticsProperties[E]) {
  whenReady((client) => client.capture(_vendorNames[event] ?? event, properties));
}

// signedIn identifies the user who just signed in by their ID, so their events join the server's.
export function signedIn(method: SignInMethodName): void {
  const id = tokenSubject(auth.token());
  whenReady((client) => {
    if (id) client.identify(id);
    client.capture(analyticsEvents.userLoggedIn, { method });
  });
}

export function signedOut(): void {
  whenReady((client) => client.reset());
}

export function pageview(path: string): void {
  capture(analyticsEvents.pageViewed, { path });
}
