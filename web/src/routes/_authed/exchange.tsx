import { useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { ArrowLeftRightIcon } from "lucide-react";
import { useMemo } from "react";

import { DataTable } from "#/components/data-table/data-table";
import { DataTablePagination } from "#/components/data-table/pagination";
import { DataTableSearch } from "#/components/data-table/search";
import { useDataTable } from "#/components/data-table/use-data-table";
import { DataTableViewOptions } from "#/components/data-table/view-options";
import { FormError } from "#/components/form/form-components";
import { useAppForm, zodValidation } from "#/components/form/use-app-form";
import { Button } from "#/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "#/components/ui/card";
import { exchangeHistoryKey, exchangeHistoryQuery, useConvert } from "#/exchange/api";
import { type ConversionRow, conversionColumns, formatRateDate } from "#/exchange/columns";
import { currencyName, currencyOptions, groupDigits } from "#/exchange/currencies";
import { convertSchema } from "#/exchange/schema";

const _pageSize = 10;

type ExchangeSearch = { page?: number };

const historyQuery = ({ page = 1 }: ExchangeSearch) =>
  exchangeHistoryQuery({ limit: _pageSize, offset: (page - 1) * _pageSize });

export const Route = createFileRoute("/_authed/exchange")({
  staticData: { title: "Exchange" },
  validateSearch: (search: Record<string, unknown>): ExchangeSearch => {
    const page = Number(search.page);

    return { page: Number.isInteger(page) && page > 1 ? page : undefined };
  },
  loaderDeps: ({ search }) => search,
  loader: ({ context, deps }) => context.queryClient.ensureQueryData(historyQuery(deps)),
  component: Exchange,
});

function Exchange() {
  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-semibold tracking-tight">Exchange</h1>
        <p className="mt-1 text-muted-foreground">
          Convert between currencies at today&rsquo;s rate.
        </p>
      </div>
      <ConvertCard />
      <History />
    </div>
  );
}

function ConvertCard() {
  const queryClient = useQueryClient();
  const convert = useConvert();

  const form = useAppForm({
    defaultValues: { amount: "100", from: "OMR", to: "CNY" },
    ...zodValidation(convertSchema),
    onSubmit: ({ value }) =>
      convert.mutateAsync(
        { body: convertSchema.parse(value) },
        { onSuccess: () => queryClient.invalidateQueries({ queryKey: exchangeHistoryKey }) },
      ),
  });

  const swap = () => {
    const { from, to } = form.state.values;
    form.setFieldValue("from", to);
    form.setFieldValue("to", from);
  };

  const result = convert.data;

  return (
    <Card className="mx-auto w-full max-w-3xl">
      <CardHeader>
        <CardTitle>Convert</CardTitle>
        <CardDescription>Each conversion is saved to your history below.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <form.AppForm>
          <form.Form>
            <form.AppField name="amount">
              {(field) => (
                <field.TextField label="Amount" required inputMode="decimal" autoComplete="off" />
              )}
            </form.AppField>
            <div className="grid items-start gap-4 sm:grid-cols-[1fr_auto_1fr]">
              <form.AppField name="from">
                {(field) => <field.SelectField label="From" required options={currencyOptions} />}
              </form.AppField>
              <Button
                type="button"
                variant="outline"
                size="icon"
                className="sm:mt-6"
                onClick={swap}
                aria-label="Swap currencies"
              >
                <ArrowLeftRightIcon />
              </Button>
              <form.AppField name="to">
                {(field) => <field.SelectField label="To" required options={currencyOptions} />}
              </form.AppField>
            </div>
            <FormError error={convert.error} />
            <form.SubmitButton size="lg" className="w-full sm:w-auto" pending="Converting…">
              Convert
            </form.SubmitButton>
          </form.Form>
        </form.AppForm>

        {result && (
          <div aria-live="polite" className="rounded-lg border bg-muted/40 p-4">
            <p className="text-sm text-muted-foreground">
              {groupDigits(result.amount)} {currencyName(result.from)} is
            </p>
            <p className="mt-1 text-3xl font-semibold tracking-tight tabular-nums">
              {groupDigits(result.result)} {result.to}
            </p>
            <p className="mt-2 text-sm text-muted-foreground">
              1 {result.from} = {result.rate} {result.to} · rate of{" "}
              {formatRateDate(result.rate_date)}
            </p>
          </div>
        )}

        <p className="text-xs text-muted-foreground">
          <a
            href="https://www.exchangerate-api.com"
            target="_blank"
            rel="noreferrer"
            className="underline underline-offset-2 hover:text-foreground"
          >
            Rates By Exchange Rate API
          </a>
          . Rates update once a day and are not live market rates.
        </p>
      </CardContent>
    </Card>
  );
}

function History() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  const { data } = useSuspenseQuery(historyQuery(search));

  const offset = ((search.page ?? 1) - 1) * _pageSize;
  const rows = useMemo<ConversionRow[]>(
    () =>
      data.exchangeHistory.conversions.map((conversion, index) => ({
        ...conversion,
        key: String(offset + index),
      })),
    [data, offset],
  );

  const table = useDataTable({
    data: rows,
    columns: conversionColumns,
    getRowId: (row) => row.key,
    pagination: {
      pageIndex: (search.page ?? 1) - 1,
      pageSize: _pageSize,
      rowCount: data.exchangeHistory.total,
      onPageChange: (pageIndex) =>
        void navigate({ search: { page: pageIndex > 0 ? pageIndex + 1 : undefined } }),
    },
  });

  return (
    <section className="space-y-4" aria-labelledby="history-heading">
      <h2 id="history-heading" className="text-xl font-semibold tracking-tight">
        History
      </h2>
      <div className="flex flex-wrap items-center gap-3">
        <DataTableSearch table={table} placeholder="Search conversions" />
        <DataTableViewOptions table={table} />
      </div>
      <DataTable
        table={table}
        empty={
          table.state.globalFilter
            ? "No conversions on this page match your search."
            : "No conversions yet. Convert an amount to start your history."
        }
      />
      <DataTablePagination table={table} />
    </section>
  );
}
