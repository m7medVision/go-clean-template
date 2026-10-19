import { useIsMutating } from "@tanstack/react-query";
import {
  CircleCheckIcon,
  CircleDashedIcon,
  CircleDotIcon,
  MoreHorizontalIcon,
  PencilIcon,
  Trash2Icon,
} from "lucide-react";

import { DataTableColumnHeader } from "#/components/data-table/column-header";
import { dataTableColumns } from "#/components/data-table/use-data-table";
import { Badge } from "#/components/ui/badge";
import { Button } from "#/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "#/components/ui/dropdown-menu";
import { cn } from "#/lib/utils";
import { transitionTask } from "./api";
import {
  type Task,
  type TaskStatus,
  nextStatuses,
  shortID,
  statusLabel,
  statusProgress,
} from "./status";

export type TaskRowActions = {
  onEdit: (task: Task) => void;
  onMove: (task: Task, status: TaskStatus) => void;
  onDelete: (task: Task) => void;
};

const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });
const formatDate = (iso: string) => dateFormat.format(new Date(iso));

const helper = dataTableColumns<Task>();
const header = DataTableColumnHeader;

const byProgress = (a: { original: Task }, b: { original: Task }) =>
  statusProgress[a.original.status] - statusProgress[b.original.status];

// taskColumns builds the column set; memoise it on the actions so the table keeps its row models.
export function taskColumns(actions: TaskRowActions) {
  return helper.columns([
    // Title and description both feed the search box; the cell shows them stacked.
    helper.accessor((task) => `${task.title}\n${task.description}`, {
      id: "task",
      header,
      meta: { label: "Task", className: "min-w-64" },
      enableHiding: false,
      sortFn: (a, b) => a.original.title.localeCompare(b.original.title),
      cell: ({ row }) => <TaskCell task={row.original} />,
    }),
    helper.accessor((task) => shortID(task.id), { id: "id", header, meta: { label: "ID" } }),
    helper.accessor("status", {
      header,
      meta: { label: "Status" },
      sortFn: byProgress,
      cell: ({ row }) => (
        <Badge variant="secondary" className="font-normal">
          {statusLabel[row.original.status]}
        </Badge>
      ),
    }),
    helper.accessor((task) => statusProgress[task.status], {
      id: "progress",
      header,
      meta: { label: "Progress", className: "min-w-48" },
      enableGlobalFilter: false,
      cell: ({ row }) => <Progress status={row.original.status} />,
    }),
    helper.accessor("createdAt", {
      header,
      meta: { label: "Created" },
      enableGlobalFilter: false,
      cell: ({ row }) => formatDate(row.original.createdAt),
    }),
    helper.accessor("updatedAt", {
      header,
      meta: { label: "Updated", className: "text-muted-foreground" },
      enableGlobalFilter: false,
      cell: ({ row }) => formatDate(row.original.updatedAt),
    }),
    helper.display({
      id: "actions",
      header: () => <span className="sr-only">Actions</span>,
      meta: { className: "w-12" },
      enableHiding: false,
      cell: ({ row }) => <RowActions task={row.original} actions={actions} />,
    }),
  ]);
}

const tileStyle: Record<TaskStatus, { icon: typeof CircleDotIcon; className: string }> = {
  TODO: { icon: CircleDashedIcon, className: "bg-muted text-muted-foreground" },
  IN_PROGRESS: {
    icon: CircleDotIcon,
    className: "bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400",
  },
  DONE: {
    icon: CircleCheckIcon,
    className: "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400",
  },
};

// TaskCell stands in for the product thumbnail and name: a status-coloured tile, then the text.
function TaskCell({ task }: { task: Task }) {
  const { icon: Icon, className } = tileStyle[task.status];

  return (
    <div className="flex items-center gap-3">
      <span
        aria-hidden
        className={cn("flex size-10 shrink-0 items-center justify-center rounded-md", className)}
      >
        <Icon className="size-5" />
      </span>
      <div className="min-w-0">
        <div className="truncate font-medium">{task.title}</div>
        {task.description && (
          <div className="max-w-80 truncate text-xs text-muted-foreground">{task.description}</div>
        )}
      </div>
    </div>
  );
}

const progressTone: Record<TaskStatus, { text: string; bar: string }> = {
  TODO: { text: "text-muted-foreground", bar: "bg-muted-foreground/40" },
  IN_PROGRESS: { text: "text-amber-600 dark:text-amber-400", bar: "bg-amber-500" },
  DONE: { text: "text-emerald-600 dark:text-emerald-400", bar: "bg-emerald-500" },
};

// Progress plays the part of the stock column: a figure, a coloured label, and a bar.
function Progress({ status }: { status: TaskStatus }) {
  const value = statusProgress[status];
  const tone = progressTone[status];

  return (
    <div className="max-w-60 space-y-1.5">
      <div>
        {value}% · <span className={tone.text}>{statusLabel[status]}</span>
      </div>
      <div
        role="progressbar"
        aria-label={`${statusLabel[status]}, ${value}%`}
        aria-valuenow={value}
        aria-valuemin={0}
        aria-valuemax={100}
        className="h-1.5 w-full overflow-hidden rounded-full bg-muted"
      >
        <div className={cn("h-full rounded-full", tone.bar)} style={{ width: `${value}%` }} />
      </div>
    </div>
  );
}

function RowActions({ task, actions }: { task: Task; actions: TaskRowActions }) {
  const moving = useIsMutating({ mutationKey: transitionTask.mutationKey }) > 0;
  const moves = nextStatuses(task.status);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={`Actions for ${task.title}`}>
          <MoreHorizontalIcon />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52">
        <DropdownMenuItem onSelect={() => actions.onEdit(task)}>
          <PencilIcon />
          Edit
        </DropdownMenuItem>
        {moves.length > 0 && <DropdownMenuSeparator />}
        {moves.map((status) => {
          const Icon = tileStyle[status].icon;

          return (
            <DropdownMenuItem
              key={status}
              disabled={moving}
              onSelect={() => actions.onMove(task, status)}
            >
              <Icon />
              Move to {statusLabel[status]}
            </DropdownMenuItem>
          );
        })}
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={() => actions.onDelete(task)}>
          <Trash2Icon />
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
