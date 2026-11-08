// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import type { TasksQuery } from "#/graphql/graphql";
import { onOperation, onOperationFails } from "#/testing/handlers";
import { renderRoute } from "#/testing/render";
import { signInAs } from "#/testing/session";

const reportError = vi.hoisted(() => vi.fn());

vi.mock("#/lib/error-tracking", () => ({ reportError, initErrorTracking: vi.fn() }));

describe("a screen that crashes while rendering", () => {
  beforeEach(() => {
    reportError.mockClear();
    signInAs();
  });

  it("shows the error view inside the layout and reports the crash", async () => {
    // The API answers without the list the table reads, so the component throws on render.
    onOperation<unknown>("Tasks", { tasks: null });

    renderRoute("/tasks");

    expect(await screen.findByRole("heading", { name: "Something went wrong" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Try again" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Profile" })).toBeTruthy();
    expect(reportError).toHaveBeenCalledWith(expect.any(TypeError));
  });

  it("loads the screen when Try again is pressed after the API is back", async () => {
    onOperationFails("Tasks");
    const { user } = renderRoute("/tasks");
    await screen.findByRole("heading", { name: "Something went wrong" });

    onOperation<TasksQuery>("Tasks", {
      tasks: {
        total: 1,
        tasks: [
          {
            id: "1",
            title: "Write the spec",
            description: "",
            status: "TODO",
            createdAt: "2026-11-08T00:00:00Z",
            updatedAt: "2026-11-08T00:00:00Z",
          },
        ],
      },
    });
    await user.click(screen.getByRole("button", { name: "Try again" }));

    expect(await screen.findByText("Write the spec")).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "Something went wrong" })).toBeNull();
  });
});
