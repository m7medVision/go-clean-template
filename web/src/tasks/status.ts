import type { TaskStatus, TasksQuery } from "#/graphql/graphql";

export type { TaskStatus };
// A row of the task list, as the Tasks query selects it.
export type Task = TasksQuery["tasks"]["tasks"][number];

export const statuses: readonly TaskStatus[] = ["TODO", "IN_PROGRESS", "DONE"];

export const statusLabel: Record<TaskStatus, string> = {
  TODO: "To do",
  IN_PROGRESS: "In progress",
  DONE: "Done",
};

// How far along a task is, for the progress column. Done is the only finished state.
export const statusProgress: Record<TaskStatus, number> = {
  TODO: 0,
  IN_PROGRESS: 50,
  DONE: 100,
};

// Mirrors entity.Task.Transition so the UI only offers moves the API accepts.
// The server stays the authority: a stale list still gets BAD_USER_INPUT and shows its message.
const transitions: Record<TaskStatus, readonly TaskStatus[]> = {
  TODO: ["IN_PROGRESS"],
  IN_PROGRESS: ["DONE", "TODO"],
  DONE: [],
};

export function nextStatuses(status: TaskStatus): readonly TaskStatus[] {
  return transitions[status];
}

export function isTaskStatus(value: unknown): value is TaskStatus {
  return typeof value === "string" && (statuses as readonly string[]).includes(value);
}

// shortID renders a UUID the way the products list renders a SKU.
export function shortID(id: string): string {
  return `TASK-${id.slice(0, 8).toUpperCase()}`;
}
