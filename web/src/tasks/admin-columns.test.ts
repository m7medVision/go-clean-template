import { describe, expect, it } from "vite-plus/test";

import { type AdminTask, adminTaskColumns } from "./admin-columns";

const task: AdminTask = {
  id: "498b3ffc-684a-4efd-b45a-467a91feac52",
  userId: "4e72bed6-8ca6-4956-bc36-a634f3526783",
  title: "Carol's task",
  status: "TODO",
  createdAt: "2026-10-22T00:00:00Z",
};

const column = (id: string) => {
  const found = adminTaskColumns.find(
    (c) => c.id === id || ("accessorKey" in c && c.accessorKey === id),
  );
  if (!found || !("accessorFn" in found) || !found.accessorFn)
    throw new Error(`no accessor column ${id}`);

  return found.accessorFn;
};

describe("adminTaskColumns", () => {
  // Regression: the owner showed the user ID dressed as a task ID ("TASK-4E72BED6").
  it("shows the owner as the user ID's first eight characters, not as a task ID", () => {
    expect(column("owner")(task, 0)).toBe("4e72bed6");
  });

  it("labels every column for the view options menu", () => {
    for (const c of adminTaskColumns) expect(c.meta?.label).toBeTruthy();
  });
});
