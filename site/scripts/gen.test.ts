import { describe, expect, it } from "vite-plus/test";
import { envTable, parseEnv } from "./env.ts";
import { makeTable, parseMake } from "./make.ts";

const config = `
type (
	App struct {
		Name    string \`env:"APP_NAME,required"\`
		// BaseURL is the public origin.
		BaseURL string \`env:"APP_BASE_URL" envDefault:"http://localhost"\`
		Skip    string
	}
)
type Worker struct {
	Retries int \`env:"WORKER_MAX_RETRIES" envDefault:"5"\`
}
`;

describe("parseEnv", () => {
  it("reads every tagged field with its section, default and comment", () => {
    expect(parseEnv(config)).toEqual([
      {
        name: "APP_NAME",
        section: "App",
        type: "string",
        required: true,
        default: undefined,
        description: "",
      },
      {
        name: "APP_BASE_URL",
        section: "App",
        type: "string",
        required: false,
        default: "http://localhost",
        description: "BaseURL is the public origin.",
      },
      {
        name: "WORKER_MAX_RETRIES",
        section: "Worker",
        type: "int",
        required: false,
        default: "5",
        description: "",
      },
    ]);
  });

  it("renders a markdown table", () => {
    expect(envTable(parseEnv(config))).toContain("| `APP_NAME` | App | string | yes |  |  |");
  });
});

describe("parseMake", () => {
  const mk = "VAR = x\nhelp: ## Display help\nrun: deps ### run the app\n\t@echo hi\nquiet:\n";

  it("reads documented targets only", () => {
    expect(parseMake(mk)).toEqual([
      { name: "help", description: "Display help" },
      { name: "run", description: "run the app" },
    ]);
  });

  it("renders a markdown table", () => {
    expect(makeTable(parseMake(mk))).toContain("| `make run` | run the app |");
  });
});
