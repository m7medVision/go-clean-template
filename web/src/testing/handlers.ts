import { HttpResponse, http } from "msw";

import type { paths } from "#/api/schema.gen";
import { server } from "./server";

type Method = "get" | "post" | "put" | "patch" | "delete";

type JsonBody<Operation> = Operation extends {
  responses: infer Responses;
}
  ? {
      [Status in keyof Responses]: Responses[Status] extends {
        content: { "application/json": infer Body };
      }
        ? Body
        : never;
    }[keyof Responses]
  : never;

const route = (path: string) => path.replaceAll(/\{(\w+)\}/g, ":$1");

// onRest answers one REST operation of the OpenAPI document. The path, the method and the body's
// type come from the generated schema, so an API change breaks the handler at compile time.
export function onRest<Path extends keyof paths, Verb extends Method & keyof paths[Path]>(
  method: Verb,
  path: Path,
  body: JsonBody<paths[Path][Verb]> | ((request: Request) => JsonBody<paths[Path][Verb]>),
  status = 200,
) {
  server.use(
    http[method](route(path), ({ request }) =>
      HttpResponse.json(
        (typeof body === "function" ? body(request) : body) as Record<string, unknown>,
        { status },
      ),
    ),
  );
}

// onProblem answers a REST operation with an RFC 9457 problem.
export function onProblem(
  method: Method,
  path: keyof paths,
  status: number,
  detail: string,
  title = "Error",
) {
  server.use(
    http[method](route(path), () =>
      HttpResponse.json({ status, title, detail }, { status, headers: problemHeaders }),
    ),
  );
}

const problemHeaders = { "Content-Type": "application/problem+json" };

const operationName = /\b(?:query|mutation)\s+(\w+)/;

// onOperation answers a GraphQL operation by its name; any other operation falls through to the
// next handler. The result type is the operation's generated type.
export function onOperation<Result, Variables = Record<string, never>>(
  name: string,
  result: Result | ((variables: Variables) => Result),
) {
  server.use(
    http.post("/graphql", async ({ request }) => {
      const body = (await request.clone().json()) as { query: string; variables?: Variables };
      if (operationName.exec(body.query)?.[1] !== name) return undefined;

      const data =
        typeof result === "function"
          ? (result as (v: Variables) => Result)(body.variables as Variables)
          : result;

      return HttpResponse.json({ data });
    }),
  );
}
