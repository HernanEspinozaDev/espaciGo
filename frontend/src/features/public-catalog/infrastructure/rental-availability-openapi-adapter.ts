import "client-only";

import createClient from "openapi-fetch";
import type { paths } from "@/lib/api/generated/openapi";
import type {
  PublicRentalAvailability,
  PublicRentalAvailabilityResult,
} from "../domain/rental-availability";
import type { RentalAvailabilityPort } from "../application/rental-availability.port";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isAvailability(value: unknown): value is PublicRentalAvailability {
  if (
    !isRecord(value) ||
    typeof value.space_id !== "string" ||
    typeof value.time_zone !== "string" ||
    (value.rate_unit !== "hora" &&
      value.rate_unit !== "dia" &&
      value.rate_unit !== "mes") ||
    typeof value.checked_at !== "string" ||
    !Array.isArray(value.items)
  )
    return false;

  return value.items.every(
    (item) =>
      isRecord(item) &&
      typeof item.start_at === "string" &&
      typeof item.end_at === "string",
  );
}

function errorCode(error: unknown): string | undefined {
  if (!isRecord(error) || !isRecord(error.error)) return undefined;
  return typeof error.error.code === "string" ? error.error.code : undefined;
}

function failure(
  status: number,
  code?: string,
): PublicRentalAvailabilityResult {
  if (status === 404) {
    return {
      ok: false,
      error: { kind: "not-found", status, ...(code ? { code } : {}) },
    };
  }
  if (status === 400 || status === 422) {
    return {
      ok: false,
      error: { kind: "invalid", status, ...(code ? { code } : {}) },
    };
  }
  return {
    ok: false,
    error: { kind: "unavailable", status, ...(code ? { code } : {}) },
  };
}

export function createRentalAvailabilityAdapter(
  fetcher: typeof fetch = fetch,
  baseUrl = "/api/backend",
): RentalAvailabilityPort {
  const client = createClient<paths>({
    baseUrl,
    credentials: "same-origin",
    cache: "no-store",
    fetch: fetcher,
  });

  return {
    async getAvailability(spaceId, date, duration, signal) {
      const { data, error, response } = await client.GET(
        "/spaces/{spaceID}/rental-availability",
        {
          params: {
            path: { spaceID: spaceId },
            query: { date, duration },
          },
          signal,
        },
      );
      if (!response.ok) return failure(response.status, errorCode(error));
      if (!data || !isAvailability(data.data)) return failure(502);
      return { ok: true, value: data.data };
    },
  };
}
