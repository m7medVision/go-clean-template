import { defineConfig, lazyPlugins } from "vite-plus";
import { devtools } from "@tanstack/devtools-vite";

import { tanstackRouter } from "@tanstack/router-plugin/vite";

import viteReact from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { sentryVitePlugin } from "@sentry/vite-plugin";

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

const vendor = {
  regex: "^posthog-js(/|$)",
  message: "only src/lib/analytics.ts imports posthog-js",
};

const restrict = (
  folders: string[],
  forbidden: string[],
  message: string,
  extra: { regex: string; message: string }[] = [vendor],
) => {
  const rule: Restriction = [
    "error",
    { patterns: [{ regex: `^#/(${forbidden.join("|")})(/|$)`, message }, ...extra] },
  ];

  return {
    files: folders.map((folder) => `src/${folder}/**`),
    rules: { "no-restricted-imports": rule },
  };
};

const vendorOnly = (files: string[]) => {
  const rule: Restriction = ["error", { patterns: [vendor] }];

  return { files, rules: { "no-restricted-imports": rule } };
};

const layers = [
  vendorOnly(["src/*.ts", "src/*.tsx", "src/routes/**", "src/testing/**"]),
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
  {
    ...restrict(
      ["lib"],
      [...domains, "routes"],
      "shared code never imports a domain or a route",
      [],
    ),
    files: ["src/lib/analytics.ts"],
  },
];

// Source maps are built only when they can be uploaded to GlitchTip (a Sentry-compatible server)
// and are deleted from dist/ afterwards, so they are never served.
const uploadMaps = Boolean(process.env.SENTRY_AUTH_TOKEN);

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
  build: { sourcemap: uploadMaps ? "hidden" : false },
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
    ...(uploadMaps
      ? [
          sentryVitePlugin({
            url: process.env.SENTRY_URL,
            authToken: process.env.SENTRY_AUTH_TOKEN,
            org: process.env.SENTRY_ORG,
            project: process.env.SENTRY_PROJECT,
            release: { name: process.env.VITE_RELEASE },
            sourcemaps: { filesToDeleteAfterUpload: ["dist/**/*.map"] },
            telemetry: false,
          }),
        ]
      : []),
  ]),
});

export default config;
