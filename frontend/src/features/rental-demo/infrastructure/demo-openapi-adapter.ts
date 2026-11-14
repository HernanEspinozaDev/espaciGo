import createClient from "openapi-fetch";
import type { paths } from "@/lib/api/generated/openapi";
import type {
  DemoActor,
  DemoBlockRequest,
  DemoConfirmation,
  DemoMonth,
  DemoOwnerSpace,
  DemoPublicSpace,
  DemoQuote,
  DemoSession,
  DemoSpaceConfiguration,
} from "../domain/types";
import type { DemoPort } from "../application/demo.port";

export function createRentalDemoTransport(fetcher: typeof fetch = fetch) {
  return {
    fetch: (input: RequestInfo | URL, init?: RequestInit) => {
      const request =
        input instanceof Request ? input : new Request(input, init);
      const url = new URL(request.url);
      if (url.pathname.startsWith("/api/local/rental-demo/"))
        url.pathname = url.pathname.replace(
          "/api/local/rental-demo/",
          "/api/rental-demo/",
        );
      const headers = new Headers(request.headers);
      headers.delete("authorization");
      return fetcher(
        new Request(url, {
          method: request.method,
          headers,
          body: request.body,
          signal: request.signal,
          credentials: "same-origin",
          redirect: "error",
          ...(request.body ? { duplex: "half" } : {}),
        } as RequestInit),
      );
    },
  };
}

export function createRentalDemoAdapter(
  fetcher: typeof fetch = fetch,
  baseUrl = "/api",
): DemoPort {
  const client = createClient<paths>({
    baseUrl,
    fetch: createRentalDemoTransport(fetcher).fetch,
  });
  function result<T>(data: T | undefined, error: unknown): T {
    if (error || data === undefined)
      throw new Error("No se pudo completar la operación de demostración.");
    return data;
  }
  return {
    async session(signal) {
      const { data, response } = await client.GET(
        "/local/rental-demo/session",
        {
          signal,
        },
      );
      if (response.status === 401) return null;
      return result(data, undefined);
    },
    async login(actor: DemoActor, signal) {
      const { data, error } = await client.POST("/local/rental-demo/session", {
        body: { actor },
        signal,
      });
      return result(data, error);
    },
    async logout(signal) {
      const { error } = await client.DELETE("/local/rental-demo/session", {
        signal,
      });
      if (error)
        throw new Error("No se pudo cerrar la sesión de demostración.");
    },
    async spaces(owner, signal) {
      if (owner) {
        const { data, error } = await client.GET(
          "/local/rental-demo/owner/spaces",
          { signal },
        );
        return result(data, error);
      }
      const { data, error } = await client.GET("/local/rental-demo/spaces", {
        signal,
      });
      return result(data, error);
    },
    async month(spaceId, month, duration, owner, signal) {
      if (owner) {
        const { data, error } = await client.GET(
          "/local/rental-demo/owner/spaces/{spaceID}/month",
          {
            params: { path: { spaceID: spaceId }, query: { month, duration } },
            signal,
          },
        );
        return result(data, error);
      }
      const { data, error } = await client.GET(
        "/local/rental-demo/spaces/{spaceID}/month",
        {
          params: { path: { spaceID: spaceId }, query: { month, duration } },
          signal,
        },
      );
      return result(data, error);
    },
    async updateSpace(spaceId, config: DemoSpaceConfiguration, signal) {
      const { data, error } = await client.PUT(
        "/local/rental-demo/owner/spaces/{spaceID}",
        { params: { path: { spaceID: spaceId } }, body: config, signal },
      );
      return result(data, error);
    },
    async createBlock(spaceId, block: DemoBlockRequest, signal) {
      const { data, error } = await client.POST(
        "/local/rental-demo/owner/spaces/{spaceID}/blocks",
        { params: { path: { spaceID: spaceId } }, body: block, signal },
      );
      return result(data, error).id;
    },
    async deleteBlock(spaceId, blockId, revision, signal) {
      const { error } = await client.DELETE(
        "/local/rental-demo/owner/spaces/{spaceID}/blocks/{blockID}",
        {
          params: {
            path: { spaceID: spaceId, blockID: blockId },
            header: { "If-Match": `"${revision}"` },
          },
          signal,
        },
      );
      if (error) throw new Error("No se pudo retirar el bloqueo.");
    },
    async quote(spaceId, startAt, endAt, signal) {
      const { data, error } = await client.POST("/local/rental-demo/quotes", {
        body: { space_id: spaceId, start_at: startAt, end_at: endAt },
        signal,
      });
      return result(data, error);
    },
    async confirm(quoteId, request, signal) {
      const { data, error } = await client.POST(
        "/local/rental-demo/quotes/{quoteID}/confirm",
        { params: { path: { quoteID: quoteId } }, body: request, signal },
      );
      return result(data, error);
    },
    async attempt(intentId, signal) {
      const { data, error } = await client.GET(
        "/local/rental-demo/attempts/{intentID}",
        { params: { path: { intentID: intentId } }, signal },
      );
      return result(data, error);
    },
  };
}

export const rentalDemoAdapter = createRentalDemoAdapter();
