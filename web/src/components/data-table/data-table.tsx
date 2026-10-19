import { type RowData, flexRender } from "@tanstack/react-table";
import type { ReactNode } from "react";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "#/components/ui/table";
import type { DataTable as DataTableInstance } from "./use-data-table";

// DataTable renders a table built with useDataTable. Toolbars and pagination sit around it.
export function DataTable<TData extends RowData>({
  table,
  empty,
}: {
  table: DataTableInstance<TData>;
  empty: ReactNode;
}) {
  const rows = table.getRowModel().rows;

  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((group) => (
          <TableRow key={group.id} className="bg-muted/40 hover:bg-muted/40">
            {group.headers.map((header) => {
              const sorted = header.column.getIsSorted();

              return (
                <TableHead
                  key={header.id}
                  className={header.column.columnDef.meta?.className}
                  aria-sort={
                    sorted === "asc" ? "ascending" : sorted === "desc" ? "descending" : undefined
                  }
                >
                  {header.isPlaceholder
                    ? null
                    : flexRender(header.column.columnDef.header, header.getContext())}
                </TableHead>
              );
            })}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {rows.length === 0 ? (
          <TableRow className="hover:bg-transparent">
            <TableCell
              colSpan={table.getVisibleLeafColumns().length}
              className="h-40 text-center text-muted-foreground"
            >
              {empty}
            </TableCell>
          </TableRow>
        ) : (
          rows.map((row) => (
            <TableRow key={row.id}>
              {row.getVisibleCells().map((cell) => (
                <TableCell key={cell.id} className={cell.column.columnDef.meta?.className}>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </TableCell>
              ))}
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
  );
}
