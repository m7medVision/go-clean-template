export interface EnvVar {
  name: string;
  section: string;
  type: string;
  required: boolean;
  default?: string;
  description: string;
}

const structLine = /^\s*(?:type\s+)?(\w+)\s+struct\s*\{/;
const fieldLine = /^\s*\w+\s+([\w.[\]*]+)\s+`([^`]*)`/;
const commentLine = /^\s*\/\/\s?(.*)$/;

export function parseEnv(source: string): EnvVar[] {
  const vars: EnvVar[] = [];
  let section = "";
  let comment: string[] = [];

  for (const line of source.split("\n")) {
    const s = structLine.exec(line);
    if (s) {
      section = s[1];
      comment = [];
      continue;
    }
    const c = commentLine.exec(line);
    if (c) {
      comment.push(c[1]);
      continue;
    }
    const f = fieldLine.exec(line);
    const tag = f && /env:"([A-Z0-9_]+)((?:,\w+)*)"/.exec(f[2]);
    if (f && tag) {
      const def = /envDefault:"([^"]*)"/.exec(f[2]);
      vars.push({
        name: tag[1],
        section,
        type: f[1],
        required: tag[2].split(",").includes("required"),
        default: def?.[1],
        description: comment.join(" ").trim(),
      });
    }
    comment = [];
  }

  return vars;
}

const cell = (s: string) => s.replace(/\|/g, "\\|");

export function envTable(vars: EnvVar[]): string {
  const rows = vars.map(
    (v) =>
      `| \`${v.name}\` | ${v.section} | ${v.type} | ${v.required ? "yes" : ""} | ${
        v.default === undefined ? "" : v.default === "" ? '`""`' : `\`${cell(v.default)}\``
      } | ${cell(v.description)} |`,
  );

  return [
    "| Variable | Section | Type | Required | Default | Notes |",
    "| --- | --- | --- | --- | --- | --- |",
    ...rows,
  ].join("\n");
}
