import { describe, expect, it } from "vite-plus/test";

import { currencyName, currencyOptions, groupDigits } from "./currencies";

describe("groupDigits", () => {
  it.each([
    ["1742.39", "1,742.39"],
    ["100", "100"],
    ["1234567.891", "1,234,567.891"],
    ["0.5", "0.5"],
  ])("groups %s as %s", (input, output) => {
    expect(groupDigits(input)).toBe(output);
  });
});

describe("currencies", () => {
  it("names a currency by its code", () => {
    expect(currencyName("OMR")).toBe("Omani Rial");
    expect(currencyName("CNY")).toBe("Chinese Yuan");
  });

  it("offers the Omani Rial and the Chinese Yuan", () => {
    const codes = currencyOptions.map((option) => option.value);

    expect(codes).toContain("OMR");
    expect(codes).toContain("CNY");
  });
});
