import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { envTable, parseEnv } from "./env.ts";
import { makeTable, parseMake } from "./make.ts";

const root = resolve(process.cwd(), "..");
const out = resolve(process.cwd(), "src/generated");
mkdirSync(out, { recursive: true });

const configDir = join(root, "config");
const vars = readdirSync(configDir)
  .filter((f) => f.endsWith(".go") && !f.endsWith("_test.go"))
  .sort((a, b) => (a === "config.go" ? -1 : b === "config.go" ? 1 : a.localeCompare(b)))
  .flatMap((f) => parseEnv(readFileSync(join(configDir, f), "utf8")));
writeFileSync(join(out, "env.md"), envTable(vars) + "\n");

const targets = parseMake(readFileSync(join(root, "Makefile"), "utf8"));
writeFileSync(join(out, "make.md"), makeTable(targets) + "\n");

console.log(`docs-gen: ${vars.length} environment variables, ${targets.length} make targets`);
