import type { ErrorEvent } from "@sentry/react";
import { describe, expect, it } from "vite-plus/test";

import { reportError, scrubEvent } from "./error-tracking";

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

describe("reportError", () => {
  it("does nothing before the SDK is started", () => {
    expect(() => reportError(new Error("boom"))).not.toThrow();
  });
});
