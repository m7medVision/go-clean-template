export interface Target {
  name: string;
  description: string;
}

const targetLine = /^([a-zA-Z0-9_-]+):[^=]*?#{2,3}\s*(.+)$/;

export function parseMake(source: string): Target[] {
  return source
    .split("\n")
    .map((l) => targetLine.exec(l))
    .filter((m): m is RegExpExecArray => m !== null)
    .map((m) => ({ name: m[1], description: m[2].trim() }));
}

export function makeTable(targets: Target[]): string {
  return [
    "| Target | What it does |",
    "| --- | --- |",
    ...targets.map((t) => `| \`make ${t.name}\` | ${t.description.replace(/\|/g, "\\|")} |`),
  ].join("\n");
}
