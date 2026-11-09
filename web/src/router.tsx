import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";
import { createRouter } from "@tanstack/react-router";

import { onUnauthorized } from "#/api/client";
import { auth } from "#/auth/token";
import { pageview } from "#/lib/analytics";
import { reportError } from "#/lib/error-tracking";
import { routerDefaults } from "./router-defaults";
import { routeTree } from "./routeTree.gen";

export const queryClient = new QueryClient({
  queryCache: new QueryCache({ onError: reportError }),
  mutationCache: new MutationCache({ onError: reportError }),
  defaultOptions: { queries: { staleTime: 30_000 } },
});

export const router = createRouter({
  routeTree,
  context: { queryClient, auth },
  defaultPreload: "intent",
  // TanStack Query owns freshness, so the router always asks it.
  defaultPreloadStaleTime: 0,
  scrollRestoration: true,
  ...routerDefaults,
});

// The route template (/tasks/$id), not the path, so a page view carries no ID.
router.subscribe("onResolved", () => {
  const match = router.state.matches.at(-1);
  if (match) pageview(match.fullPath);
});

onUnauthorized(() => {
  queryClient.clear();
  // A query still settling after logout can 401 on /login; leave the redirect it already has.
  const { pathname, href } = router.state.location;
  if (pathname === "/login") return;
  void router.navigate({ to: "/login", search: { redirect: href } });
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }

  // title names a route in the breadcrumb trail of the signed-in layout.
  interface StaticDataRouteOption {
    title?: string;
  }
}
