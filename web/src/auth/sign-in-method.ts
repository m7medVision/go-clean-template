import type { LucideIcon } from "lucide-react";

// SignInMethod is a way of signing in that /login offers as a button under the password form.
// Starting one resets the others, so only its own error shows.
export type SignInMethod = {
  key: string;
  // Whether this browser and server can use it; the button shows only when they can.
  available: boolean;
  icon: LucideIcon;
  label: string;
  pendingLabel: string;
  pending: boolean;
  // The server's rejection, for FormError.
  error: unknown;
  start: () => void;
  reset: () => void;
};
