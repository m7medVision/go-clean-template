import { createFileRoute, redirect } from "@tanstack/react-router";

// Tasks is the home screen; the _authed layout sends a signed-out visitor on to /login.
export const Route = createFileRoute("/")({
  beforeLoad: () => {
    throw redirect({ to: "/tasks" });
  },
});
