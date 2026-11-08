// @vitest-environment happy-dom
import { screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vite-plus/test";

import type {
  CreateTaskMutation,
  DeleteTaskMutation,
  TaskStatus,
  TasksQuery,
  TransitionTaskMutation,
} from "#/graphql/graphql";
import { onOperation } from "#/testing/handlers";
import { renderRoute } from "#/testing/render";
import { signInAs } from "#/testing/session";

type Row = TasksQuery["tasks"]["tasks"][number];

const row = (id: string, title: string, status: TaskStatus = "TODO"): Row => ({
  id,
  title,
  description: "",
  status,
  createdAt: "2026-11-08T00:00:00Z",
  updatedAt: "2026-11-08T00:00:00Z",
  attributes: { priority: null, estimate: null, labels: null },
});

// serve answers the task operations from an in-memory list, as the API would.
function serve(tasks: Row[]) {
  onOperation<TasksQuery>("Tasks", () => ({ tasks: { total: tasks.length, tasks: [...tasks] } }));
  onOperation<CreateTaskMutation, { input: { title: string } }>("CreateTask", ({ input }) => {
    tasks.push(row(`id-${tasks.length + 1}`, input.title));

    return { createTask: { id: `id-${tasks.length}` } };
  });
  onOperation<TransitionTaskMutation, { id: string; status: TaskStatus }>(
    "TransitionTask",
    ({ id, status }) => {
      const task = tasks.find((t) => t.id === id);
      if (task) task.status = status;

      return { transitionTask: { id } };
    },
  );
  onOperation<DeleteTaskMutation, { id: string }>("DeleteTask", ({ id }) => {
    tasks.splice(
      tasks.findIndex((t) => t.id === id),
      1,
    );

    return { deleteTask: true };
  });
}

describe("the tasks screen", () => {
  beforeEach(() => signInAs());

  it("lists the caller's tasks in the table", async () => {
    serve([row("1", "Write the spec"), row("2", "Review the plan")]);

    renderRoute("/tasks");

    expect(await screen.findByText("Write the spec")).toBeTruthy();
    expect(screen.getByText("Review the plan")).toBeTruthy();
  });

  it("adds a task and shows it", async () => {
    serve([row("1", "Write the spec")]);
    const { user } = renderRoute("/tasks");
    await screen.findByText("Write the spec");

    await user.click(screen.getByRole("button", { name: "Add task" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText(/^Title/), "Ship it");
    await user.click(within(dialog).getByRole("button", { name: "Add task" }));

    expect(await screen.findByText("Ship it")).toBeTruthy();
  });

  it("moves a task to its next status", async () => {
    serve([row("1", "Write the spec")]);
    const { user } = renderRoute("/tasks");
    await screen.findByText("Write the spec");

    await user.click(screen.getByRole("button", { name: "Actions for Write the spec" }));
    await user.click(await screen.findByRole("menuitem", { name: /Move to In progress/ }));

    await waitFor(() => expect(screen.getAllByText(/In progress/).length).toBeGreaterThan(0));
    await user.click(screen.getByRole("button", { name: "Actions for Write the spec" }));
    expect(await screen.findByRole("menuitem", { name: /Move to Done/ })).toBeTruthy();
  });

  it("deletes a task after confirmation", async () => {
    serve([row("1", "Write the spec"), row("2", "Review the plan")]);
    const { user } = renderRoute("/tasks");
    await screen.findByText("Write the spec");

    await user.click(screen.getByRole("button", { name: "Actions for Write the spec" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    await user.click(await screen.findByRole("button", { name: "Delete task" }));

    await waitFor(() => expect(screen.queryByText("Write the spec")).toBeNull());
    expect(screen.getByText("Review the plan")).toBeTruthy();
  });
});
