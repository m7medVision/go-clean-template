import { ArrowRightIcon } from "lucide-react";

import { DataTableColumnHeader } from "#/components/data-table/column-header";
import { dataTableColumns } from "#/components/data-table/use-data-table";
import type { ExchangeHistoryQuery } from "#/graphql/graphql";
import { currencyName, groupDigits } from "./currencies";

export type Conversion = ExchangeHistoryQuery["exchangeHistory"]["conversions"][number];

// A row is a conversion plus its position in the whole history, which keys it: conversions have
// no id of their own.
export type ConversionRow = Conversion & { key: string };

const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeZone: "UTC" });
// formatRateDate shows a YYYY-MM-DD rate date as a calendar day, whatever the viewer's time zone.
export const formatRateDate = (day: string) => dateFormat.format(new Date(`${day}T00:00:00Z`));

const helper = dataTableColumns<ConversionRow>();
const header = DataTableColumnHeader;

// Sort decimal strings by value; Number is exact enough to order them.
const byDecimal =
  (pick: (row: ConversionRow) => string) =>
  (a: { original: ConversionRow }, b: { original: ConversionRow }) =>
    Number(pick(a.original)) - Number(pick(b.original));

export const conversionColumns = helper.columns([
  helper.accessor(
    (row) => `${row.from} ${row.to} ${currencyName(row.from)} ${currencyName(row.to)}`,
    {
      id: "pair",
      header,
      meta: { label: "Currencies", className: "min-w-56" },
      enableHiding: false,
      sortFn: (a, b) =>
        `${a.original.from}${a.original.to}`.localeCompare(`${b.original.from}${b.original.to}`),
      cell: ({ row }) => <PairCell conversion={row.original} />,
    },
  ),
  helper.accessor("amount", {
    header,
    meta: { label: "Amount", className: "text-right tabular-nums" },
    sortFn: byDecimal((row) => row.amount),
    cell: ({ row }) => `${groupDigits(row.original.amount)} ${row.original.from}`,
  }),
  helper.accessor("result", {
    header,
    meta: { label: "Converted", className: "text-right font-medium tabular-nums" },
    sortFn: byDecimal((row) => row.result),
    cell: ({ row }) => `${groupDigits(row.original.result)} ${row.original.to}`,
  }),
  helper.accessor("rate", {
    header,
    meta: { label: "Rate", className: "text-right text-muted-foreground tabular-nums" },
    enableGlobalFilter: false,
    sortFn: byDecimal((row) => row.rate),
    cell: ({ row }) => `1 ${row.original.from} = ${row.original.rate} ${row.original.to}`,
  }),
  helper.accessor("rateDate", {
    header,
    meta: { label: "Rate date", className: "text-muted-foreground" },
    enableGlobalFilter: false,
    cell: ({ row }) => formatRateDate(row.original.rateDate),
  }),
]);

function PairCell({ conversion }: { conversion: Conversion }) {
  return (
    <div className="flex items-center gap-2">
      <span className="font-medium">{conversion.from}</span>
      <ArrowRightIcon aria-label="to" className="size-3.5 text-muted-foreground" />
      <span className="font-medium">{conversion.to}</span>
      <span className="truncate text-xs text-muted-foreground">
        {currencyName(conversion.from)} to {currencyName(conversion.to)}
      </span>
    </div>
  );
}
