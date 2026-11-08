// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vite-plus/test";

import type { TasksQuery } from "#/graphql/graphql";
import { onOperation } from "#/testing/handlers";
import { renderRoute } from "#/testing/render";
import { signInAs } from "#/testing/session";

describe("the sidebar", () => {
  beforeEach(() => onOperation<TasksQuery>("Tasks", { tasks: { total: 0, tasks: [] } }));

  it("links to the admin screen for an admin", async () => {
    signInAs(["user", "admin"]);

    renderRoute("/tasks");

    expect(await screen.findByRole("link", { name: "All tasks" })).toBeTruthy();
  });

  it("does not link to the admin screen for anyone else", async () => {
    signInAs(["user"]);

    renderRoute("/tasks");

    await screen.findByRole("link", { name: "Profile" });
    expect(screen.queryByRole("link", { name: "All tasks" })).toBeNull();
  });
});
