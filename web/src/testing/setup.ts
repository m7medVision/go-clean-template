import { cleanup } from "@testing-library/react";
import { afterAll, afterEach, beforeAll, vi } from "vite-plus/test";

import { auth } from "#/auth/token";
import { server } from "./server";

vi.mock("@tanstack/react-devtools", () => ({ TanStackDevtools: () => null }));
vi.mock("@tanstack/react-router-devtools", () => ({ TanStackRouterDevtoolsPanel: () => null }));

beforeAll(() => server.listen({ onUnhandledFrame: "error" }));

afterEach(() => {
  cleanup();
  server.resetHandlers();
  auth.clear();
});

afterAll(() => server.close());
