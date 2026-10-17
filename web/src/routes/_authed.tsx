import { Outlet, createFileRoute, redirect } from "@tanstack/react-router";

// Layout route for every screen that needs a signed-in user. The API is the real check:
// an expired token still reaches it, gets a 401, and the client sends the user back here.
export const Route = createFileRoute("/_authed")({
  beforeLoad: ({ context, location }) => {
    if (!context.auth.token()) {
      throw redirect({ to: "/login", search: { redirect: location.href } });
    }
  },
  component: Outlet,
});
