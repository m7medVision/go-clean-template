import { defineConfig, lazyPlugins } from "vite-plus";
import { devtools } from "@tanstack/devtools-vite";

import { tanstackRouter } from "@tanstack/router-plugin/vite";

import viteReact from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The Go app. `make web-dev` exports HTTP_PORT from .env / .env.example.
const api = `http://127.0.0.1:${process.env.HTTP_PORT ?? "8080"}`;

// Paths the Go app owns. Keep in sync with restapi.NewRouter and the Caddyfile.
const backendPaths = [
  "/v1",
  "/graphql",
  "/docs",
  "/openapi.json",
  "/openapi.yaml",
  "/schemas",
  "/healthz",
];

const generated = ["src/routeTree.gen.ts", "src/api/schema.gen.ts", "src/graphql/**"];

const domains = ["tasks", "exchange", "passkey", "sso"];

type Restriction = ["error", { patterns: { regex: string; message: string }[] }];

const restrict = (folders: string[], forbidden: string[], message: string) => {
  const rule: Restriction = [
    "error",
    { patterns: [{ regex: `^#/(${forbidden.join("|")})(/|$)`, message }] },
  ];

  return {
    files: folders.map((folder) => `src/${folder}/**`),
    rules: { "no-restricted-imports": rule },
  };
};

const layers = [
  ...domains.map((domain) =>
    restrict(
      [domain],
      [...domains.filter((other) => other !== domain), "routes"],
      "a domain never imports another domain or a route",
    ),
  ),
  restrict(
    ["api", "auth", "components", "hooks", "lib"],
    [...domains, "routes"],
    "shared code never imports a domain or a route",
  ),
];

const config = defineConfig({
  fmt: {
    ignorePatterns: ["dist/**", ...generated],
  },
  lint: {
    ignorePatterns: ["dist/**", ...generated],
    jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
    rules: { "vite-plus/prefer-vite-plus-imports": "error" },
    options: { typeAware: true, typeCheck: true },
    overrides: layers,
  },
  resolve: { tsconfigPaths: true },
  test: {
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: ["./src/testing/setup.ts"],
    environmentOptions: { happyDOM: { url: "http://localhost:3000" } },
  },
  server: {
    // `make dev` and `make web-dev` export WEB_PORT from .env / .env.example.
    port: Number(process.env.WEB_PORT ?? "3000"),
    // Same origin as in production, so no CORS and no absolute API URLs.
    proxy: Object.fromEntries(backendPaths.map((path) => [path, api])),
  },
  plugins: lazyPlugins(() => [
    devtools(),
    tailwindcss(),
    tanstackRouter({ target: "react", autoCodeSplitting: true }),
    viteReact(),
  ]),
});

export default config;
