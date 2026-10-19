import { z } from "zod";

// Mirrors request.Convert's Huma tags and entity.NewConversion's rules; the server stays the authority.
const amountPattern = /^[0-9]{1,15}(\.[0-9]{1,8})?$/;
const codePattern = /^[A-Za-z]{3}$/;

const currency = z
  .string()
  .regex(codePattern, "Pick a currency.")
  .transform((code) => code.toUpperCase());

// convertSchema validates the convert form. The amount stays a string so no precision is lost
// before the server computes with exact decimals.
export const convertSchema = z
  .object({
    amount: z
      .string()
      .trim()
      .regex(amountPattern, "Enter an amount like 100 or 100.50.")
      .refine((amount) => /[1-9]/.test(amount), "Enter an amount above zero."),
    from: currency,
    to: currency,
  })
  .refine((value) => value.from !== value.to, {
    message: "Pick two different currencies.",
    path: ["to"],
  });
