import "server-only";

import { getApiBackendBaseUrl } from "@/lib/api/server-config";
import {
  getCloudRunIdentityHeaders,
  getTargetAudience,
} from "@/lib/api/cloud-run-identity";
import { serializeCatalogQuery } from "../domain/query-state";
import type {
  CatalogFailure,
  CatalogQuery,
  CatalogResult,
  PublicCategoryFilters,
  PublicLocations,
  PublicSpaceCategories,
  PublicSpaceDetail,
  PublicSpacePage,
  TermsResponse,
} from "../domain/types";
import type { PublicCatalogPort } from "../application/catalog-port";

function safeCode(value: unknown): string | undefined {
  if (typeof value === "string" && /^[A-Za-z0-9_.-]{1,96}$/.test(value)) {
    return value;
  }
  return undefined;
}

async function requestJson<T>(path: string): Promise<CatalogResult<T>> {
  try {
    const base = getApiBackendBaseUrl();
    const url = new URL(
      `${base.pathname.replace(/\/$/, "")}${path}`,
      base.origin,
    );
    const idHeaders = await getCloudRunIdentityHeaders(
      getTargetAudience(base.origin),
    );
    const response = await fetch(url, {
      method: "GET",
      headers: {
        Accept: "application/json",
        ...idHeaders,
      },
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.timeout(15_000),
    });
    if (!response.ok) {
      let code: string | undefined;
      try {
        const body: unknown = await response.json();
        if (body && typeof body === "object" && "error" in body) {
          const error = body.error;
          if (error && typeof error === "object" && "code" in error)
            code = safeCode(error.code);
        }
      } catch {
        // Keep only the status when the upstream body is not safe JSON.
      }
      const failure: CatalogFailure = {
        kind:
          response.status === 404
            ? "not-found"
            : response.status === 422
              ? "invalid"
              : "unavailable",
        status: response.status,
        ...(code ? { code } : {}),
      };
      return { ok: false, error: failure };
    }
    const contentType = response.headers.get("content-type")?.split(";", 1)[0];
    if (contentType?.trim().toLowerCase() !== "application/json") {
      return { ok: false, error: { kind: "unavailable", status: 502 } };
    }
    return { ok: true, value: (await response.json()) as T };
  } catch {
    return { ok: false, error: { kind: "unavailable", status: 503 } };
  }
}

function pathWithQuery(path: string, query: URLSearchParams): string {
  const suffix = query.toString();
  return suffix ? `${path}?${suffix}` : path;
}

export const goPublicCatalogAdapter: PublicCatalogPort = {
  listSpaces(query: CatalogQuery) {
    return requestJson<PublicSpacePage>(
      pathWithQuery("/public/spaces", serializeCatalogQuery(query)),
    );
  },
  listCategories() {
    return requestJson<PublicSpaceCategories>("/public/space-categories");
  },
  getCategoryFilters(code: string) {
    return requestJson<PublicCategoryFilters>(
      `/public/space-categories/${encodeURIComponent(code)}/filters`,
    );
  },
  listLocations(query = {}) {
    const params = new URLSearchParams();
    if (query.q) params.set("q", query.q.normalize("NFC"));
    if (query.region_code) params.set("region_code", query.region_code);
    if (query.limit !== undefined) params.set("limit", String(query.limit));
    return requestJson<PublicLocations>(
      pathWithQuery("/public/locations", params),
    );
  },
  getSpace(spaceId: string) {
    return requestJson<{ data: PublicSpaceDetail }>(
      `/public/spaces/${encodeURIComponent(spaceId)}`,
    ).then((result) =>
      result.ok ? { ok: true as const, value: result.value.data } : result,
    );
  },
  listTerms() {
    return requestJson<TermsResponse>("/auth/terms");
  },
};
