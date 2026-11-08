// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vite-plus/test";

import { auth } from "#/auth/token";
import type { TasksQuery } from "#/graphql/graphql";
import { onOperation, onProblem, onRest } from "#/testing/handlers";
import { renderRoute } from "#/testing/render";

const token = ["e30", btoa(JSON.stringify({ roles: ["user"] })).replaceAll("=", ""), "sig"].join(
  ".",
);

async function fillAndSubmit(user: ReturnType<typeof renderRoute>["user"]) {
  await user.type(await screen.findByLabelText(/Email address/), "jane@example.com");
  await user.type(screen.getByLabelText(/Password/), "correct horse battery");
  await user.click(screen.getByRole("button", { name: "Sign in" }));
}

describe("the sign-in screen", () => {
  beforeEach(() => onRest("get", "/v1/auth/oidc", { enabled: false }));

  it("signs in and lands on the tasks", async () => {
    const login = onRest("post", "/v1/auth/login", { token });
    onRest("get", "/v1/user/profile", {
      id: "550e8400-e29b-41d4-a716-446655440000",
      email: "jane@example.com",
      username: "jane",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    });
    onOperation<TasksQuery>("Tasks", { tasks: { total: 0, tasks: [] } });
    const { user } = renderRoute("/login");

    await fillAndSubmit(user);

    expect(await screen.findByRole("heading", { name: "Tasks" })).toBeTruthy();
    expect(auth.token()).toBe(token);
    expect(login.sent).toEqual([{ email: "jane@example.com", password: "correct horse battery" }]);
  });

  it("shows the server's refusal for a wrong password and stays signed out", async () => {
    onProblem("post", "/v1/auth/login", 401, "invalid email or password");
    const { user } = renderRoute("/login");

    await fillAndSubmit(user);

    expect((await screen.findByRole("alert")).textContent).toContain("invalid email or password");
    expect(auth.token()).toBeNull();
  });

  it("shows the rate limit's message", async () => {
    onProblem("post", "/v1/auth/login", 429, "too many attempts, try again later");
    const { user } = renderRoute("/login");

    await fillAndSubmit(user);

    expect((await screen.findByRole("alert")).textContent).toContain("too many attempts");
    expect(auth.token()).toBeNull();
  });
});
