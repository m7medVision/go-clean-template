import { describe, expect, it } from "vite-plus/test";

import { convertSchema } from "./schema";

describe("convertSchema", () => {
  it("trims the amount and uppercases the codes", () => {
    expect(convertSchema.parse({ amount: " 100.10 ", from: "omr", to: "cny" })).toEqual({
      amount: "100.10",
      from: "OMR",
      to: "CNY",
    });
  });

  it.each([
    ["an empty amount", { amount: "", from: "OMR", to: "CNY" }, "Enter an amount"],
    ["a word", { amount: "ten", from: "OMR", to: "CNY" }, "Enter an amount"],
    ["a negative amount", { amount: "-5", from: "OMR", to: "CNY" }, "Enter an amount"],
    ["a zero amount", { amount: "0.00", from: "OMR", to: "CNY" }, "above zero"],
    ["too many decimals", { amount: "1.123456789", from: "OMR", to: "CNY" }, "Enter an amount"],
    ["a missing currency", { amount: "1", from: "", to: "CNY" }, "Pick a currency"],
    ["the same currency twice", { amount: "1", from: "OMR", to: "omr" }, "two different"],
  ])("rejects %s", (_, input, message) => {
    const result = convertSchema.safeParse(input);

    expect(result.success).toBe(false);
    expect(result.error?.issues[0]?.message).toContain(message);
  });
});
