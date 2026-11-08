import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vite-plus/test";

import { RouteError, RouteNotFound } from "./route-error";

const noop = () => {};
const view = (error: unknown) =>
  renderToStaticMarkup(<RouteError error={error as Error} reset={noop} />);

describe("RouteError", () => {
  it("shows the API's problem as an alert", () => {
    const html = view({ status: 500, detail: "the database is down" });

    expect(html).toContain("the database is down");
    expect(html).toContain('role="alert"');
  });

  it("falls back to a plain line for an error without a problem body", () => {
    expect(view(null)).toContain("This page could not be loaded.");
  });

  it("never shows the message of a plain Error", () => {
    const html = view(new Error("boom at /srv/app/secret.ts:12"));

    expect(html).not.toContain("boom");
    expect(html).not.toContain("secret.ts");
    expect(html).toContain("This page could not be loaded.");
  });

  it("offers Try again", () => {
    expect(view(null)).toContain("Try again");
  });
});

describe("RouteNotFound", () => {
  it("links back to the tasks", async () => {
    const root = createRootRoute({ component: RouteNotFound });
    const tasks = createRoute({ getParentRoute: () => root, path: "/tasks" });
    const router = createRouter({
      routeTree: root.addChildren([tasks]),
      history: createMemoryHistory({ initialEntries: ["/tasks"] }),
    });
    await router.load();

    const html = renderToStaticMarkup(<RouterProvider router={router} />);

    expect(html).toContain("Page not found");
    expect(html).toContain('href="/tasks"');
  });
});
