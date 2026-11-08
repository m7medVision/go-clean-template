// @vitest-environment happy-dom
import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vite-plus/test";

import type { ExchangeHistoryQuery } from "#/graphql/graphql";
import { onOperation, onProblem, onRest } from "#/testing/handlers";
import { renderRoute } from "#/testing/render";
import { signInAs } from "#/testing/session";

type Conversion = ExchangeHistoryQuery["exchangeHistory"]["conversions"][number];

const conversion: Conversion = {
  from: "OMR",
  to: "CNY",
  amount: "100",
  rate: "18.4250",
  result: "1842.50",
  rateDate: "2026-11-08",
};

describe("the exchange screen", () => {
  beforeEach(() => signInAs());

  it("lists the conversion history", async () => {
    onOperation<ExchangeHistoryQuery>("ExchangeHistory", {
      exchangeHistory: { total: 1, conversions: [conversion] },
    });

    renderRoute("/exchange");

    expect(await screen.findByRole("heading", { name: "Exchange" })).toBeTruthy();
    expect(await screen.findByText(/1,?842\.5/)).toBeTruthy();
  });

  it("converts and adds it to the history", async () => {
    const history: Conversion[] = [];
    onOperation<ExchangeHistoryQuery>("ExchangeHistory", () => ({
      exchangeHistory: { total: history.length, conversions: [...history] },
    }));
    const convert = onRest("post", "/v1/exchange/convert", () => {
      history.push(conversion);

      return {
        from: conversion.from,
        to: conversion.to,
        amount: conversion.amount,
        rate: conversion.rate,
        result: conversion.result,
        rate_date: conversion.rateDate,
      };
    });
    const { user } = renderRoute("/exchange");
    expect(await screen.findByText(/No conversions yet/)).toBeTruthy();

    await user.click(screen.getByRole("button", { name: "Convert" }));

    await screen.findAllByText(/1,?842\.5/);
    expect(screen.queryByText(/No conversions yet/)).toBeNull();
    expect(convert.sent).toEqual([{ amount: "100", from: "OMR", to: "CNY" }]);
  });

  it("shows why a conversion was refused", async () => {
    onOperation<ExchangeHistoryQuery>("ExchangeHistory", {
      exchangeHistory: { total: 0, conversions: [] },
    });
    onProblem("post", "/v1/exchange/convert", 502, "the rate provider is unavailable");
    const { user } = renderRoute("/exchange");
    await screen.findByText(/No conversions yet/);

    await user.click(screen.getByRole("button", { name: "Convert" }));

    expect((await screen.findByRole("alert")).textContent).toContain("rate provider");
  });
});
