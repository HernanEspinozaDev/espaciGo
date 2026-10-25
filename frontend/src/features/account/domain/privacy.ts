import type {
  components,
  Readable,
  Writable,
} from "@/lib/api/generated/openapi";

export type RightsRequestKind = Writable<
  components["schemas"]["RightsRequest"]["kind"]
>; // "acceso" | "supresion"
export type RightsRequestState = Writable<
  components["schemas"]["RightsRequest"]["state"]
>; // "recibida" | "identidad_pendiente" | "en_revision" | "resuelta" | "denegada_fundada"

export type RightsRequest = Readable<components["schemas"]["RightsRequest"]>;
export type RightsRequestList = Readable<
  components["schemas"]["RightsRequestList"]
>;
export type OwnIdentityExport = Readable<
  components["schemas"]["OwnIdentityExport"]
>;

export function isSuppressionPending(
  requests: readonly RightsRequest[] | null | undefined,
): boolean {
  if (!requests || !Array.isArray(requests)) return false;
  return requests.some(
    (r) =>
      r.kind === "supresion" &&
      (r.state === "recibida" ||
        r.state === "en_revision" ||
        r.state === "identidad_pendiente"),
  );
}

export function hasResolvedSuppression(
  requests: readonly RightsRequest[] | null | undefined,
): boolean {
  if (!requests || !Array.isArray(requests)) return false;
  return requests.some((r) => r.kind === "supresion" && r.state === "resuelta");
}
