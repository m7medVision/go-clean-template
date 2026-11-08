// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { describe, expect, it } from "vite-plus/test";

import type { TasksQuery } from "#/graphql/graphql";
import { onOperation } from "#/testing/handlers";
import { renderRoute } from "#/testing/render";
import { signInAs } from "#/testing/session";

describe("the tasks screen", () => {
  it("lists the caller's tasks in the table", async () => {
    signInAs();
    onOperation<TasksQuery>("Tasks", {
      tasks: {
        total: 1,
        tasks: [
          {
            id: "1",
            title: "Write the spec",
            description: "",
            status: "TODO" as never,
            createdAt: "2026-11-08T00:00:00Z",
            updatedAt: "2026-11-08T00:00:00Z",
          },
        ],
      },
    });

    renderRoute("/tasks");

    expect(await screen.findByText("Write the spec")).toBeTruthy();
  });
});
