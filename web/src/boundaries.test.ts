import { spawnSync } from "node:child_process";
import { mkdirSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { afterAll, beforeAll, describe, expect, it } from "vite-plus/test";

// The import layers are lint rules in vite.config.ts. This lints real files that break (or keep)
// them, so a rule that stops working fails here and not silently.

const root = join(import.meta.dirname, "..");
const probes = join(root, "src", "__boundary_probes__");

const cases = [
  ["a domain importing another domain", "tasks", "#/exchange/schema", true],
  ["a domain importing a route", "passkey", "#/routes/login", true],
  ["shared components importing a domain", "components", "#/sso/ceremony", true],
  ["shared lib importing a route", "lib", "#/routes/login", true],
  ["shared auth importing a domain", "auth", "#/tasks/status", true],
  ["a domain importing its own folder", "tasks", "#/tasks/status", false],
  ["a domain importing shared code", "tasks", "#/api/problem", false],
] as const;

const file = (folder: string, index: number) => `src/${folder}/__probe_${index}.ts`;
let output = "";

describe("the import layers", () => {
  beforeAll(() => {
    mkdirSync(probes, { recursive: true });
    cases.forEach(([, folder, target], index) => {
      writeFileSync(join(root, file(folder, index)), `import "${target}";\n`);
    });
    const result = spawnSync(
      join(root, "node_modules/.bin/vp"),
      ["lint", ...cases.map(([, folder], index) => file(folder, index))],
      { cwd: root, encoding: "utf8" },
    );
    output = `${result.stdout}${result.stderr}`;
  }, 120_000);

  afterAll(() => {
    rmSync(probes, { recursive: true, force: true });
    cases.forEach(([, folder], index) => rmSync(join(root, file(folder, index)), { force: true }));
  });

  it.each(cases.map((c, index) => [...c, index] as const))(
    "%s",
    (_name, folder, _target, forbidden, index) => {
      const flagged = output
        .split("\n")
        .some(
          (line) => line.includes(file(folder, index)) && line.includes("no-restricted-imports"),
        );

      expect(flagged).toBe(forbidden);
    },
  );
});
