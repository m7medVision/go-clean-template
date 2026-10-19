import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { PlusIcon } from "lucide-react";
import { useMemo, useState } from "react";

import { problemMessage } from "#/api/problem";
import { DataTable } from "#/components/data-table/data-table";
import { DataTablePagination } from "#/components/data-table/pagination";
import { DataTableSearch } from "#/components/data-table/search";
import { useDataTable } from "#/components/data-table/use-data-table";
import { DataTableViewOptions } from "#/components/data-table/view-options";
import { Button } from "#/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "#/components/ui/select";
import { Tabs, TabsList, TabsTrigger } from "#/components/ui/tabs";
import { tasksKey, tasksQuery, transitionTask } from "#/tasks/api";
import { type TaskRowActions, taskColumns } from "#/tasks/columns";
import { type Task, type TaskStatus, isTaskStatus, statusLabel, statuses } from "#/tasks/status";
import { DeleteTaskDialog, TaskFormDialog } from "#/tasks/task-dialogs";

const _pageSize = 10;
const _allStatuses = "all";

type TasksSearch = { status?: TaskStatus; page?: number };

const listQuery = ({ status, page = 1 }: TasksSearch) =>
  tasksQuery({ status, limit: _pageSize, offset: (page - 1) * _pageSize });

export const Route = createFileRoute("/_authed/tasks")({
  staticData: { title: "Tasks" },
  validateSearch: (search: Record<string, unknown>): TasksSearch => {
    const page = Number(search.page);

    return {
      status: isTaskStatus(search.status) ? search.status : undefined,
      page: Number.isInteger(page) && page > 1 ? page : undefined,
    };
  },
  loaderDeps: ({ search }) => search,
  loader: ({ context, deps }) => context.queryClient.ensureQueryData(listQuery(deps)),
  component: Tasks,
});

function Tasks() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const queryClient = useQueryClient();
  const { data } = useSuspenseQuery(listQuery(search));

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Task>();
  const [deleting, setDeleting] = useState<Task>();

  const transition = useMutation({
    ...transitionTask,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: tasksKey }),
  });
  const move = transition.mutate;

  const openForm = (task?: Task) => {
    setEditing(task);
    setFormOpen(true);
  };

  const columns = useMemo(() => {
    const actions: TaskRowActions = {
      onEdit: (task) => {
        setEditing(task);
        setFormOpen(true);
      },
      onMove: (task, status) => move({ id: task.id, status }),
      onDelete: setDeleting,
    };

    return taskColumns(actions);
  }, [move]);

  const table = useDataTable({
    data: data.tasks.tasks,
    columns,
    getRowId: (task) => task.id,
    pagination: {
      pageIndex: (search.page ?? 1) - 1,
      pageSize: _pageSize,
      rowCount: data.tasks.total,
      onPageChange: (pageIndex) =>
        void navigate({
          search: (prev) => ({ ...prev, page: pageIndex > 0 ? pageIndex + 1 : undefined }),
        }),
    },
  });

  const filtering = Boolean(table.state.globalFilter);

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">Tasks</h1>
          <p className="mt-1 text-muted-foreground">Browse, filter, and manage your tasks.</p>
        </div>
        <Button size="lg" onClick={() => openForm()}>
          <PlusIcon />
          Add task
        </Button>
      </div>

      <div className="space-y-4">
        <Tabs value="all">
          <TabsList variant="line" className="h-auto w-full justify-start border-b p-0">
            <TabsTrigger value="all" className="flex-none px-1 pb-3">
              All tasks
            </TabsTrigger>
          </TabsList>
        </Tabs>

        <div className="flex flex-wrap items-center gap-3">
          <DataTableSearch table={table} placeholder="Search tasks" />
          <Select
            value={search.status ?? _allStatuses}
            onValueChange={(value) =>
              void navigate({
                search: { status: isTaskStatus(value) ? value : undefined, page: undefined },
              })
            }
          >
            <SelectTrigger className="h-9 w-40" aria-label="Filter by status">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={_allStatuses}>All statuses</SelectItem>
              {statuses.map((status) => (
                <SelectItem key={status} value={status}>
                  {statusLabel[status]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <DataTableViewOptions table={table} />
        </div>

        {transition.isError && (
          <p role="alert" className="text-sm text-destructive">
            {problemMessage(transition.error)}
          </p>
        )}

        <DataTable
          table={table}
          empty={
            filtering
              ? "No tasks on this page match your search."
              : search.status
                ? `No tasks are ${statusLabel[search.status].toLowerCase()}.`
                : "No tasks yet. Add one to get started."
          }
        />
        <DataTablePagination table={table} />
      </div>

      <TaskFormDialog task={editing} open={formOpen} onOpenChange={setFormOpen} />
      <DeleteTaskDialog task={deleting} onOpenChange={(open) => !open && setDeleting(undefined)} />
    </div>
  );
}
