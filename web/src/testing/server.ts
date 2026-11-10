import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";

import type { components } from "#/api/schema.gen";

// Every flag answers its backend default unless a test answers /v1/flags itself (onRest).
const defaultFlags: components["schemas"]["FlagValues"] = { taskAttributes: true };

export const server = setupServer(http.get("/v1/flags", () => HttpResponse.json(defaultFlags)));
