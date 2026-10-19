// The currencies the form offers. The rate provider knows about 160; this list keeps the picker
// short and leads with the Gulf. The server stays the authority: an unknown code is a 400.
const codes = [
  "OMR",
  "CNY",
  "AED",
  "SAR",
  "QAR",
  "KWD",
  "BHD",
  "USD",
  "EUR",
  "GBP",
  "INR",
  "PKR",
  "EGP",
  "TRY",
  "JPY",
  "KRW",
  "SGD",
  "HKD",
  "MYR",
  "CHF",
  "CAD",
  "AUD",
] as const;

const names = new Intl.DisplayNames(["en"], { type: "currency" });

export const currencyName = (code: string) => names.of(code) ?? code;

export const currencyOptions = codes.map((code) => ({
  value: code,
  label: `${code} · ${currencyName(code)}`,
}));

// groupDigits adds thousands separators to a decimal string without turning it into a float.
export function groupDigits(decimal: string): string {
  const [whole = "", fraction] = decimal.split(".");
  const grouped = whole.replace(/\B(?=(\d{3})+(?!\d))/g, ",");

  return fraction === undefined ? grouped : `${grouped}.${fraction}`;
}
