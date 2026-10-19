import type { RowData } from "@tanstack/react-table";
import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react";

import { Button } from "#/components/ui/button";
import type { DataTable } from "./use-data-table";

// DataTablePagination shows where the current page sits in the server's total, with
// previous / next controls.
export function DataTablePagination<TData extends RowData>({ table }: { table: DataTable<TData> }) {
  const { pageIndex, pageSize } = table.state.pagination;
  const total = table.getRowCount();
  const pageCount = Math.max(1, table.getPageCount());
  const first = total === 0 ? 0 : pageIndex * pageSize + 1;
  const last = Math.min((pageIndex + 1) * pageSize, total);

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-muted-foreground">
      <p>{total === 0 ? "No results" : `Showing ${first}–${last} of ${total}`}</p>
      <div className="flex items-center gap-2">
        <span>
          Page {pageIndex + 1} of {pageCount}
        </span>
        <Button
          variant="outline"
          size="icon"
          aria-label="Previous page"
          disabled={!table.getCanPreviousPage()}
          onClick={() => table.previousPage()}
        >
          <ChevronLeftIcon />
        </Button>
        <Button
          variant="outline"
          size="icon"
          aria-label="Next page"
          disabled={!table.getCanNextPage()}
          onClick={() => table.nextPage()}
        >
          <ChevronRightIcon />
        </Button>
      </div>
    </div>
  );
}
