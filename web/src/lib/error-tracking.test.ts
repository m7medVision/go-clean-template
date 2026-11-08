import type { ErrorEvent } from "@sentry/react";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";

import { auth } from "#/auth/token";
import { initErrorTracking, reportError, scrubEvent } from "./error-tracking";

const event = (extra: Partial<ErrorEvent>): ErrorEvent => ({ type: undefined, ...extra });

describe("scrubEvent", () => {
  it("removes the query string, headers, cookies and body from the request", () => {
    const scrubbed = scrubEvent(
      event({
        request: {
          url: "http://localhost/tasks?status=DONE&token=abc",
          query_string: "status=DONE&token=abc",
          headers: { Authorization: "Bearer secret" },
          cookies: { a: "b" },
          data: { password: "x" },
        },
      }),
    );

    expect(scrubbed?.request).toEqual({ url: "http://localhost/tasks" });
  });

  it("strips query strings from request breadcrumbs", () => {
    const scrubbed = scrubEvent(
      event({
        breadcrumbs: [{ category: "fetch", data: { url: "/graphql?x=1", method: "POST" } }],
      }),
    );

    expect(scrubbed?.breadcrumbs?.[0]?.data).toEqual({ url: "/graphql", method: "POST" });
  });

  it("strips query strings from navigation breadcrumbs", () => {
    const scrubbed = scrubEvent(
      event({
        breadcrumbs: [{ category: "navigation", data: { from: "/a?x=1", to: "/tasks?secret=2" } }],
      }),
    );

    expect(scrubbed?.breadcrumbs?.[0]?.data).toEqual({ from: "/a", to: "/tasks" });
  });

  it("drops console breadcrumbs, which can hold anything", () => {
    const scrubbed = scrubEvent(
      event({
        breadcrumbs: [
          { category: "console", message: 'route {"secret":"hunter2"}' },
          { category: "ui.click", message: "button" },
        ],
      }),
    );

    expect(scrubbed?.breadcrumbs).toEqual([{ category: "ui.click", message: "button" }]);
  });

  it("keeps only the user's id", () => {
    const scrubbed = scrubEvent(event({ user: { id: "u1", email: "a@b.co", username: "jane" } }));

    expect(scrubbed?.user).toEqual({ id: "u1" });
  });

  it("drops an event whose cause is not worth reporting", () => {
    expect(scrubEvent(event({}), { originalException: { status: 401 } })).toBeNull();
    expect(scrubEvent(event({}), { originalException: new Error("boom") })).not.toBeNull();
  });
});

const captured: unknown[] = [];
const users: unknown[] = [];
const sdkInit = vi.hoisted(() => vi.fn());

vi.mock("@sentry/react", () => ({
  init: sdkInit,
  withScope: (callback: (scope: { setUser: (user: unknown) => void }) => void) =>
    callback({ setUser: (user) => users.push(user) }),
  captureException: (error: unknown) => captured.push(error),
}));

const jwt = (payload: unknown) =>
  ["e30", btoa(JSON.stringify(payload)).replaceAll("=", ""), "sig"].join(".");

describe("reportError", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    auth.clear();
  });

  it("does nothing before the SDK is started", () => {
    expect(() => reportError(new Error("boom"))).not.toThrow();
    expect(captured).toHaveLength(0);
  });

  it("loads and starts nothing when no DSN was built in", async () => {
    vi.stubEnv("VITE_SENTRY_DSN", "");

    await initErrorTracking();
    reportError(new Error("boom"));
    await Promise.resolve();

    expect(sdkInit).not.toHaveBeenCalled();
    expect(captured).toHaveLength(0);
  });

  it("sends an error reported while the SDK is still loading, once, and skips expected ones", async () => {
    vi.stubEnv("VITE_SENTRY_DSN", "http://key@localhost:9/1");
    const started = initErrorTracking();
    const crash = new Error("early");

    reportError(crash);
    reportError(crash);
    reportError({ status: 401 });
    await started;
    await Promise.resolve();

    expect(sdkInit).toHaveBeenCalledTimes(1);
    expect(captured).toEqual([crash]);
  });

  it("tags the report with the signed-in user's id and nothing else", async () => {
    auth.set(jwt({ sub: "user-123", email: "jane@example.com", roles: ["user"] }));

    reportError(new Error("later"));

    expect(users).toEqual([{ id: "user-123" }]);
  });

  it("sets no user when nobody is signed in", () => {
    users.length = 0;

    reportError(new Error("anonymous"));

    expect(users).toEqual([]);
  });
});
