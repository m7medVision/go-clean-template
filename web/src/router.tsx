import { QueryClient } from "@tanstack/react-query";
import { createRouter } from "@tanstack/react-router";

import { onUnauthorized } from "#/api/client";
import { auth } from "#/auth/token";
import { routeTree } from "./routeTree.gen";

export const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000 } },
});

export const router = createRouter({
  routeTree,
  context: { queryClient, auth },
  defaultPreload: "intent",
  // TanStack Query owns freshness, so the router always asks it.
  defaultPreloadStaleTime: 0,
  scrollRestoration: true,
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
