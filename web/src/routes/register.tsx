import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { $api } from "#/api/client";
import { problemMessage } from "#/api/problem";

export const Route = createFileRoute("/register")({ component: Register });

// Mirrors the Huma constraints on request.Register; the server stays the authority.
const _minUsername = 3;
const _minPassword = 6;

function Register() {
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  const register = $api.useMutation("post", "/v1/auth/register", {
    onSuccess: () => void navigate({ to: "/login" }),
  });

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    register.mutate({ body: { username, email, password } });
  };

  return (
    <form onSubmit={submit} className="space-y-4">
      <h1 className="text-2xl font-bold">Register</h1>
      <label className="block">
        Username
        <input
          required
          minLength={_minUsername}
          autoComplete="username"
          value={username}
          onChange={(event) => setUsername(event.target.value)}
          className="mt-1 block w-full rounded border px-3 py-2"
        />
      </label>
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
          minLength={_minPassword}
          autoComplete="new-password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          className="mt-1 block w-full rounded border px-3 py-2"
        />
      </label>
      {register.isError && (
        <p role="alert" className="text-red-700">
          {problemMessage(register.error)}
        </p>
      )}
      <button
        type="submit"
        disabled={register.isPending}
        className="rounded bg-black px-4 py-2 text-white"
      >
        {register.isPending ? "Creating account…" : "Create account"}
      </button>
    </form>
  );
}
