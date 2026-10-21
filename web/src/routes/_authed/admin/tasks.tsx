import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { DataTable } from "#/components/data-table/data-table";
import { DataTablePagination } from "#/components/data-table/pagination";
import { DataTableSearch } from "#/components/data-table/search";
import { useDataTable } from "#/components/data-table/use-data-table";
import { DataTableViewOptions } from "#/components/data-table/view-options";
import { adminTaskColumns } from "#/tasks/admin-columns";
import { AllTasksError } from "#/tasks/admin-error";
import { allTasksQuery } from "#/tasks/api";

const _pageSize = 10;

type AllTasksSearch = { page?: number };

const listQuery = ({ page = 1 }: AllTasksSearch) =>
  allTasksQuery({ limit: _pageSize, offset: (page - 1) * _pageSize });

export const Route = createFileRoute("/_authed/admin/tasks")({
  staticData: { title: "All tasks" },
  validateSearch: (search: Record<string, unknown>): AllTasksSearch => {
    const page = Number(search.page);

    return { page: Number.isInteger(page) && page > 1 ? page : undefined };
  },
  loaderDeps: ({ search }) => search,
  loader: ({ context, deps }) => context.queryClient.ensureQueryData(listQuery(deps)),
  component: AllTasks,
  errorComponent: AllTasksError,
});

function AllTasks() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const { data } = useSuspenseQuery(listQuery(search));

  const table = useDataTable({
    data: data.allTasks.tasks,
    columns: adminTaskColumns,
    getRowId: (task) => task.id,
    pagination: {
      pageIndex: (search.page ?? 1) - 1,
      pageSize: _pageSize,
      rowCount: data.allTasks.total,
      onPageChange: (pageIndex) =>
        void navigate({ search: { page: pageIndex > 0 ? pageIndex + 1 : undefined } }),
    },
  });

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-semibold tracking-tight">All tasks</h1>
        <p className="mt-1 text-muted-foreground">Every user's tasks. Visible to admins only.</p>
      </div>

      <div className="space-y-4">
        <div className="flex flex-wrap items-center gap-3">
          <DataTableSearch table={table} placeholder="Search tasks" />
          <DataTableViewOptions table={table} />
        </div>
        <DataTable
          table={table}
          empty={
            table.state.globalFilter
              ? "No tasks on this page match your search."
              : "No user has a task yet."
          }
        />
        <DataTablePagination table={table} />
      </div>
    </div>
  );
}
