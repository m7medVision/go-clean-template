import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createMemoryHistory, createRouter } from "@tanstack/react-router";
import { render } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { auth } from "#/auth/token";
import { routeTree } from "#/routeTree.gen";
import { routerDefaults } from "#/router-defaults";

// renderRoute mounts the app's real route tree at path, with a fresh query cache that never retries.
export function renderRoute(path: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0, gcTime: 0 } },
  });
  const router = createRouter({
    routeTree,
    context: { queryClient, auth },
    history: createMemoryHistory({ initialEntries: [path] }),
    defaultPendingMinMs: 0,
    ...routerDefaults,
  });

  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );

  return { router, queryClient, user: userEvent.setup() };
}
