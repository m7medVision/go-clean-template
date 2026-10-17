import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { $api } from "#/api/client";
import { problemMessage } from "#/api/problem";
import { safeRedirect } from "#/auth/redirect";

type LoginSearch = { redirect?: string };

export const Route = createFileRoute("/login")({
  validateSearch: (search: Record<string, unknown>): LoginSearch =>
    typeof search.redirect === "string" ? { redirect: search.redirect } : {},
  component: Login,
});

function Login() {
  const { auth } = Route.useRouteContext();
  const { redirect } = Route.useSearch();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  const login = $api.useMutation("post", "/v1/auth/login", {
    onSuccess: ({ token }) => {
      auth.set(token);
      void navigate({ to: safeRedirect(redirect) });
    },
  });

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    login.mutate({ body: { email, password } });
  };

  return (
    <form onSubmit={submit} className="space-y-4">
      <h1 className="text-2xl font-bold">Log in</h1>
      <label className="block">
        Email
        <input
          type="email"
          required
          autoComplete="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          className="mt-1 block w-full rounded border px-3 py-2"
        />
      </label>
      <label className="block">
        Password
        <input
          type="password"
          required
          autoComplete="current-password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          className="mt-1 block w-full rounded border px-3 py-2"
        />
      </label>
      {login.isError && (
        <p role="alert" className="text-red-700">
          {problemMessage(login.error)}
        </p>
      )}
      <button
        type="submit"
        disabled={login.isPending}
        className="rounded bg-black px-4 py-2 text-white"
      >
        {login.isPending ? "Logging in…" : "Log in"}
      </button>
    </form>
  );
}
