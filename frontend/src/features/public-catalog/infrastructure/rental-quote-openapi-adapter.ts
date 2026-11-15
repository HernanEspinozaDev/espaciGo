import "client-only";

import createClient from "openapi-fetch";
import type { paths } from "@/lib/api/generated/openapi";
import type { RentalQuotePort } from "../application/rental-quote.port";
import type {
  RentalQuoteRequest,
  RentalQuoteResult,
} from "../domain/rental-quote";

function isRentalQuote(
  value: unknown,
): value is import("../domain/rental-quote").RentalQuote {
  const record = (item: unknown): item is Record<string, unknown> =>
    typeof item === "object" && item !== null && !Array.isArray(item);
  const uuid =
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
  const dateTime = (item: unknown): item is string =>
    typeof item === "string" &&
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(
      item,
    ) &&
    Number.isFinite(Date.parse(item));
  const atLeast = (item: string, minimum: string) =>
    item.length > minimum.length ||
    (item.length === minimum.length && item >= minimum);
  const int64 = (item: unknown, minimum = "0"): item is number | string => {
    if (typeof item === "number")
      return (
        Number.isSafeInteger(item) &&
        item >= 0 &&
        atLeast(String(item), minimum)
      );
    if (typeof item !== "string" || !/^(0|[1-9]\d*)$/.test(item)) return false;
    const maxInt64 = "9223372036854775807";
    if (
      item.length > maxInt64.length ||
      (item.length === maxInt64.length && item > maxInt64)
    )
      return false;
    return atLeast(item, minimum);
  };
  if (!record(value)) return false;
  if (
    typeof value.id !== "string" ||
    !uuid.test(value.id) ||
    typeof value.space_id !== "string" ||
    !uuid.test(value.space_id) ||
    typeof value.space_title !== "string" ||
    value.space_title.trim().length === 0 ||
    (value.rate_unit !== "hora" &&
      value.rate_unit !== "dia" &&
      value.rate_unit !== "mes") ||
    !int64(value.unit_price_clp, "5001") ||
    value.currency !== "CLP" ||
    !int64(value.units, "1") ||
    !int64(value.subtotal_clp, "5001") ||
    !dateTime(value.start_at) ||
    !dateTime(value.end_at) ||
    Date.parse(value.end_at) <= Date.parse(value.start_at) ||
    typeof value.time_zone !== "string" ||
    value.time_zone.trim().length === 0 ||
    !dateTime(value.created_at) ||
    !dateTime(value.expires_at) ||
    Date.parse(value.expires_at) <= Date.parse(value.created_at)
  )
    return false;
  try {
    new Intl.DateTimeFormat("es-CL", { timeZone: value.time_zone });
  } catch {
    return false;
  }
  const unitPrice = BigInt(value.unit_price_clp);
  const units = BigInt(value.units);
  const subtotal = BigInt(value.subtotal_clp);
  return unitPrice * units === subtotal;
}

function failure(status: number): RentalQuoteResult {
  if (status === 400 || status === 422)
    return { ok: false, error: { kind: "invalid", status } };
  if (status === 401)
    return { ok: false, error: { kind: "unauthenticated", status } };
  if (status === 403)
    return { ok: false, error: { kind: "forbidden", status } };
  if (status === 404)
    return { ok: false, error: { kind: "not_found", status } };
  if (status === 409) return { ok: false, error: { kind: "conflict", status } };
  if (status === 503)
    return { ok: false, error: { kind: "unavailable", status } };
  return { ok: false, error: { kind: "other", status } };
}

export function createRentalQuoteAdapter(
  fetcher: typeof fetch = fetch,
  baseUrl = "/api/backend",
): RentalQuotePort {
  const client = createClient<paths>({
    baseUrl,
    credentials: "same-origin",
    cache: "no-store",
    fetch: fetcher,
  });

  return {
    async createQuote(request: RentalQuoteRequest, signal?: AbortSignal) {
      const { data, response } = await client.POST("/rental-quotes", {
        body: {
          space_id: request.space_id,
          start_at: request.start_at,
          end_at: request.end_at,
        },
        signal,
      });
      if (!response.ok) return failure(response.status);
      if (
        response.status !== 201 ||
        !data ||
        !isRentalQuote(data.data) ||
        data.data.space_id !== request.space_id ||
        data.data.start_at !== request.start_at ||
        data.data.end_at !== request.end_at
      )
        return failure(502);
      return { ok: true, value: data.data };
    },
  };
}
