// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";

import { onOperation } from "#/testing/handlers";
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
});
