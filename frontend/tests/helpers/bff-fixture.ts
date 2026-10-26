import type { Request } from "@playwright/test";

/** UI-contract fixture only. Real cookie/CSRF tests must hit the BFF itself. */
export interface BffFixtureResult {
  status: number;
  body?: unknown;
  contentType?: string;
  headers?: Record<string, string>;
}

type Input = Omit<BffFixtureResult, "status"> & { status?: number };

export function createBffFixtureState() {
  let authenticated = false;
  let expiresAt: string | null = null;
  return {
    adapt(
      request: Request,
      result: Input | null | undefined,
    ): BffFixtureResult | null {
      const pathname = new URL(request.url()).pathname;
      if (pathname === "/api/backend/auth/session" && !authenticated) {
        return {
          status: 401,
          body: {
            error: {
              code: "unauthenticated",
              message: "No active fixture session",
            },
          },
        };
      }
      if (!result) return null;
      const output: BffFixtureResult = {
        ...result,
        status: result.status ?? 200,
      };
      if (pathname === "/api/backend/auth/login" && output.status === 200) {
        const body = output.body as Record<string, unknown>;
        if (!body || typeof body.expires_at !== "string")
          throw new Error("Fixture login must supply its absolute expiry");
        // DTO whitelist: never deliver a Go access token to the browser mock.
        output.body = {
          account_id: body.account_id,
          roles: body.roles,
          capabilities: body.capabilities,
          restricted_mode: body.restricted_mode,
          expires_at: body.expires_at,
        };
        expiresAt = body.expires_at;
        authenticated = true;
      }
      if (pathname === "/api/backend/auth/session" && output.status === 200) {
        const body = output.body as Record<string, unknown>;
        output.body = {
          account_id: body.account_id,
          roles: body.roles,
          capabilities: body.capabilities,
          restricted_mode: body.restricted_mode,
          expires_at: expiresAt,
        };
      }
      if (pathname === "/api/backend/auth/logout") {
        authenticated = false;
        expiresAt = null;
        output.headers = {
          ...output.headers,
          "x-espacigo-session-cleared": "1",
        };
        if (output.status === 204 || output.status === 401) {
          output.status = 204;
          output.body = undefined;
        } else {
          output.status = 502;
          output.body = {
            error: {
              code: "remote_logout_unconfirmed",
              message: "Remote logout not confirmed in controlled fixture",
            },
          };
        }
      }
      return output;
    },
  };
}
