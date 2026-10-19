import { z } from "zod";

// Mirror the Huma constraints on request.Login and request.Register; the server stays the
// authority and its own errors still reach the form through FormError.

const email = z.email("Enter a valid email address.");

export const loginSchema = z.object({
  email,
  password: z.string().min(1, "Enter your password."),
});

const _minUsername = 3;
const _maxUsername = 255;
export const minPassword = 6;

export const registerSchema = z.object({
  username: z
    .string()
    .trim()
    .min(_minUsername, `Use at least ${_minUsername} characters.`)
    .max(_maxUsername, `Use at most ${_maxUsername} characters.`),
  email,
  password: z.string().min(minPassword, `Use at least ${minPassword} characters.`),
});
