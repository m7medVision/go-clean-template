import type { CodegenConfig } from "@graphql-codegen/cli";

// Types for the GraphQL operations in src/, from the gqlgen schema. Run through `make web-graphql`.
const config: CodegenConfig = {
  schema: "../internal/controller/graphql/schema/*.graphqls",
  documents: ["src/**/*.{ts,tsx}", "!src/graphql/**"],
  ignoreNoDocuments: true,
  generates: {
    "src/graphql/": {
      preset: "client",
      presetConfig: { fragmentMasking: false },
      config: {
        // Plain strings, so no GraphQL runtime ships to the browser.
        documentMode: "string",
        enumsAsTypes: true,
        useTypeImports: true,
        scalars: { Time: "string" },
      },
    },
  },
};

export default config;
