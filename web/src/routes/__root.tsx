import type { QueryClient } from "@tanstack/react-query";
import { Link, Outlet, createRootRouteWithContext, useRouter } from "@tanstack/react-router";
import { TanStackRouterDevtoolsPanel } from "@tanstack/react-router-devtools";
import { TanStackDevtools } from "@tanstack/react-devtools";

import type { TokenStore } from "#/auth/token";
import "../styles.css";

export type RouterContext = {
  queryClient: QueryClient;
  auth: TokenStore;
};

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootComponent,
});

function RootComponent() {
  const router = useRouter();
  const { auth, queryClient } = Route.useRouteContext();
  const signedIn = auth.token() !== null;

  const logout = () => {
    auth.clear();
    queryClient.clear();
    void router.navigate({ to: "/login" });
  };

  return (
    <>
      <header className="flex items-center gap-4 border-b px-6 py-3">
        <Link to="/" className="font-semibold">
          Go Clean Template
        </Link>
        <nav className="ml-auto flex gap-4 text-sm">
          {signedIn ? (
            <>
              <Link to="/profile" className="[&.active]:underline">
                Profile
              </Link>
              <button type="button" onClick={logout}>
                Log out
              </button>
            </>
          ) : (
            <>
              <Link to="/login" className="[&.active]:underline">
                Log in
              </Link>
              <Link to="/register" className="[&.active]:underline">
                Register
              </Link>
            </>
          )}
        </nav>
      </header>
      <main className="mx-auto max-w-md p-6">
        <Outlet />
      </main>
      <TanStackDevtools
        config={{ position: "bottom-right" }}
        plugins={[{ name: "TanStack Router", render: <TanStackRouterDevtoolsPanel /> }]}
      />
    </>
  );
}
