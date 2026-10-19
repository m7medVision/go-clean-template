import {
  type CellData,
  type ColumnDef,
  type ColumnVisibilityState,
  type ReactTable,
  type RowData,
  type SortingState,
  type TableFeatures,
  columnFilteringFeature,
  columnVisibilityFeature,
  createColumnHelper,
  createFilteredRowModel,
  createSortedRowModel,
  filterFns,
  functionalUpdate,
  globalFilteringFeature,
  rowPaginationFeature,
  rowSortingFeature,
  sortFns,
  tableFeatures,
  useTable,
} from "@tanstack/react-table";
import { useState } from "react";

// Every data table uses this one feature set, so the shared parts (header, view options,
// pagination) can rely on sorting, visibility, filtering and pagination being there.
// Paging is server-side; sorting and the search box apply to the rows the server returned.
export const dataTableFeatures = tableFeatures({
  rowSortingFeature,
  columnVisibilityFeature,
  columnFilteringFeature,
  globalFilteringFeature,
  rowPaginationFeature,
  sortedRowModel: createSortedRowModel(),
  filteredRowModel: createFilteredRowModel(),
  sortFns,
  filterFns,
});

export type DataTableFeatures = typeof dataTableFeatures;
export type DataTable<TData extends RowData> = ReactTable<DataTableFeatures, TData>;

declare module "@tanstack/table-core" {
  // oxlint-disable-next-line no-unused-vars -- the parameters must match the declaration being merged
  interface ColumnMeta<
    TFeatures extends TableFeatures,
    TData extends RowData,
    TValue extends CellData,
  > {
    // label names the column in its header and in the "Manage table" menu.
    label?: string;
    // className is applied to the column's header and body cells.
    className?: string;
  }
}

export function dataTableColumns<TData extends RowData>() {
  return createColumnHelper<DataTableFeatures, TData>();
}

type ServerPagination = {
  pageIndex: number;
  pageSize: number;
  // rowCount is the total the server reported, across all pages.
  rowCount: number;
  onPageChange: (pageIndex: number) => void;
};

export function useDataTable<TData extends RowData>({
  data,
  columns,
  getRowId,
  pagination,
}: {
  data: ReadonlyArray<TData>;
  // oxlint-disable-next-line no-explicit-any -- columns hold different value types; the helper types each one
  columns: ReadonlyArray<ColumnDef<DataTableFeatures, TData, any>>;
  getRowId: (row: TData) => string;
  pagination: ServerPagination;
}): DataTable<TData> {
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnVisibility, setColumnVisibility] = useState<ColumnVisibilityState>({});
  const [globalFilter, setGlobalFilter] = useState("");
  const page = { pageIndex: pagination.pageIndex, pageSize: pagination.pageSize };

  return useTable({
    features: dataTableFeatures,
    data,
    columns,
    getRowId,
    state: { sorting, columnVisibility, globalFilter, pagination: page },
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    onGlobalFilterChange: setGlobalFilter,
    globalFilterFn: "includesString",
    manualPagination: true,
    rowCount: pagination.rowCount,
    onPaginationChange: (updater) =>
      pagination.onPageChange(functionalUpdate(updater, page).pageIndex),
  });
}
