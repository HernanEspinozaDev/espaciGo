import type { components } from "@/lib/api/generated/openapi";

export type CreateRentalQuoteRequest =
  components["schemas"]["CreateRentalQuoteRequest"];
export type RentalQuoteRequest = CreateRentalQuoteRequest;
export type RentalQuote = components["schemas"]["RentalQuote"];

export type RentalQuoteError =
  | { kind: "invalid"; status: 400 | 422 }
  | { kind: "unauthenticated"; status: 401 }
  | { kind: "forbidden"; status: 403 }
  | { kind: "not_found"; status: 404 }
  | { kind: "conflict"; status: 409 }
  | { kind: "unavailable"; status: 503 }
  | { kind: "other"; status: number };

export type RentalQuoteResult =
  { ok: true; value: RentalQuote } | { ok: false; error: RentalQuoteError };
