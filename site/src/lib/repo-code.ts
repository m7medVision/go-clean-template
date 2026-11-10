import { readFileSync } from "node:fs";
import { extname, resolve } from "node:path";

export const repoRoot = resolve(process.cwd(), "..");

const languages: Record<string, string> = {
  ".go": "go",
  ".proto": "proto",
  ".graphqls": "graphql",
  ".sql": "sql",
  ".ts": "ts",
  ".tsx": "tsx",
  ".yml": "yaml",
  ".yaml": "yaml",
  ".json": "json",
  ".sh": "sh",
  ".tmpl": "go",
  ".mk": "makefile",
};

export interface Excerpt {
  code: string;
  lang: string;
  startLine: number;
}

export interface ExcerptOptions {
  symbol?: string;
  lines?: string;
}

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

function declaration(symbol: string): RegExp {
  const name = escape(symbol);

  return new RegExp(
    `^(func (\\([^)]*\\) )?${name}\\b|type ${name}\\b|(const|var) ${name}\\b|` +
      `(message|service|enum|input|interface|scalar|type) ${name}\\b|extend type ${name}\\b)`,
  );
}

function blockEnd(lines: string[], start: number): number {
  let depth = 0;
  let opened = false;

  for (let i = start; i < lines.length; i++) {
    for (const ch of lines[i]) {
      if (ch === "{" || ch === "(") {
        depth++;
        opened = true;
      } else if (ch === "}" || ch === ")") {
        depth--;
      }
    }
    if (opened && depth === 0) {
      return i;
    }
  }

  return start;
}

function withComments(lines: string[], start: number): number {
  let i = start;
  while (i > 0 && /^\s*(\/\/|#)/.test(lines[i - 1])) {
    i--;
  }

  return i;
}

export function excerpt(path: string, opts: ExcerptOptions = {}): Excerpt {
  const file = resolve(repoRoot, path);
  if (!file.startsWith(repoRoot)) {
    throw new Error(`RepoCode: ${path} is outside the repository`);
  }
  let text: string;
  try {
    text = readFileSync(file, "utf8");
  } catch {
    throw new Error(`RepoCode: ${path} does not exist`);
  }
  const lines = text.replace(/\n$/, "").split("\n");
  const lang = languages[extname(path)] ?? "text";

  if (opts.symbol) {
    const re = declaration(opts.symbol);
    const at = lines.findIndex((l) => re.test(l));
    if (at < 0) {
      throw new Error(`RepoCode: ${opts.symbol} not found in ${path}`);
    }
    const from = withComments(lines, at);
    const to = blockEnd(lines, at);

    return { code: lines.slice(from, to + 1).join("\n"), lang, startLine: from + 1 };
  }

  if (opts.lines) {
    const m = /^(\d+)-(\d+)$/.exec(opts.lines);
    if (!m) {
      throw new Error(`RepoCode: lines must be "from-to", got "${opts.lines}"`);
    }
    const from = Number(m[1]);
    const to = Number(m[2]);
    if (from < 1 || to < from || to > lines.length) {
      throw new Error(
        `RepoCode: lines ${opts.lines} out of range for ${path} (${lines.length} lines)`,
      );
    }

    return { code: lines.slice(from - 1, to).join("\n"), lang, startLine: from };
  }

  return { code: lines.join("\n"), lang, startLine: 1 };
}
