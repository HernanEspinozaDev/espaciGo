import "client-only";

import createClient from "openapi-fetch";
import type { paths } from "./generated/openapi";

export const apiClient = createClient<paths>({
  baseUrl: "/api/backend",
  credentials: "same-origin",
  cache: "no-store",
  fetch: (request: Request) => {
    const headers = new Headers(request.headers);
    if (headers.get("authorization") === "Bearer bff-session")
      headers.delete("authorization");
    return fetch(new Request(request, { headers, credentials: "same-origin" }));
  },
});
