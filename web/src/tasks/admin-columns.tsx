import { DataTableColumnHeader } from "#/components/data-table/column-header";
import { dataTableColumns } from "#/components/data-table/use-data-table";
import { Badge } from "#/components/ui/badge";
import type { AllTasksQuery } from "#/graphql/graphql";
import { statusLabel, statusProgress } from "./status";

export type AdminTask = AllTasksQuery["allTasks"]["tasks"][number];

const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });

const helper = dataTableColumns<AdminTask>();
const header = DataTableColumnHeader;

// The "All tasks" table is read-only: an admin sees every user's tasks but changes only their own.
export const adminTaskColumns = helper.columns([
  helper.accessor("title", {
    header,
    meta: { label: "Task", className: "min-w-64 font-medium" },
    enableHiding: false,
  }),
  // The owner's user ID, shortened; the API does not join the users table for a name.
  helper.accessor((task) => task.userId.slice(0, 8), {
    id: "owner",
    header,
    meta: { label: "Owner" },
    cell: ({ getValue }) => <span className="font-mono text-xs">{getValue()}</span>,
  }),
  helper.accessor("status", {
    header,
    meta: { label: "Status" },
    sortFn: (a, b) => statusProgress[a.original.status] - statusProgress[b.original.status],
    cell: ({ row }) => (
      <Badge variant="secondary" className="font-normal">
        {statusLabel[row.original.status]}
      </Badge>
    ),
  }),
  helper.accessor("createdAt", {
    header,
    meta: { label: "Created" },
    enableGlobalFilter: false,
    cell: ({ row }) => dateFormat.format(new Date(row.original.createdAt)),
  }),
]);
