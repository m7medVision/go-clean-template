import type { RowData } from "@tanstack/react-table";
import { SearchIcon } from "lucide-react";

import { InputGroup, InputGroupAddon, InputGroupInput } from "#/components/ui/input-group";
import type { DataTable } from "./use-data-table";

// DataTableSearch filters the rows on the current page across every searchable column.
export function DataTableSearch<TData extends RowData>({
  table,
  placeholder,
}: {
  table: DataTable<TData>;
  placeholder: string;
}) {
  return (
    <InputGroup className="h-9 w-full sm:w-64">
      <InputGroupAddon>
        <SearchIcon />
      </InputGroupAddon>
      <InputGroupInput
        type="search"
        aria-label={`${placeholder} on this page`}
        placeholder={placeholder}
        value={String(table.state.globalFilter ?? "")}
        onChange={(event) => table.setGlobalFilter(event.target.value)}
      />
    </InputGroup>
  );
}
