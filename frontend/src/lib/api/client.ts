import "client-only";

import createClient from "openapi-fetch";
import type { paths } from "./generated/openapi";

export const apiClient = createClient<paths>({
  baseUrl: "/api/backend",
  credentials: "omit",
  cache: "no-store",
});
