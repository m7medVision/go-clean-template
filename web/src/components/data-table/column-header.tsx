import type { CellData, Column, RowData } from "@tanstack/react-table";
import { ArrowDownIcon, ArrowUpIcon, ChevronsUpDownIcon } from "lucide-react";

import type { DataTableFeatures } from "./use-data-table";

// DataTableColumnHeader is a column's header: its label, plus a sort toggle when it can sort.
// Clicking cycles ascending, descending, unsorted.
export function DataTableColumnHeader<TData extends RowData, TValue extends CellData>({
  column,
}: {
  column: Column<DataTableFeatures, TData, TValue>;
}) {
  const label = column.columnDef.meta?.label ?? column.id;
  if (!column.getCanSort()) return <span className="text-xs font-medium">{label}</span>;

  const sorted = column.getIsSorted();
  const Icon =
    sorted === "asc" ? ArrowUpIcon : sorted === "desc" ? ArrowDownIcon : ChevronsUpDownIcon;

  return (
    <button
      type="button"
      onClick={column.getToggleSortingHandler()}
      className="inline-flex items-center gap-1 rounded-sm text-xs font-medium text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none"
    >
      {label}
      <Icon className="size-3" />
    </button>
  );
}
