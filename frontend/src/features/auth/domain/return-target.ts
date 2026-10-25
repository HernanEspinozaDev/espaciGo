import {
  parseCatalogQuery,
  serializeCatalogQuery,
} from "@/features/public-catalog/domain/query-state";
import type { RawCatalogParams } from "@/features/public-catalog/domain/types";

const categoryCode = /^[a-z][a-z0-9_-]{0,79}$/;
const canonicalUuid =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const catalogQueryKeys = new Set([
  "q",
  "category_code",
  "region_code",
  "comuna_code",
  "min_area_m2",
  "rate_unit",
  "min_unit_price_clp",
  "max_unit_price_clp",
  "start_at",
  "end_at",
  "min_total_clp",
  "max_total_clp",
  "sort",
  "page_size",
  "cursor",
]);
const fixedPaths = new Set([
  "/",
  "/explorar",
  "/categorias",
  "/como-funciona",
  "/anfitriones",
  "/arriendos",
  "/dashboard",
  "/dashboard/general",
  "/dashboard/security",
  "/dashboard/activity",
  "/mi-cuenta",
  "/mi-cuenta/privacidad",
  "/mi-cuenta/seguridad",
  "/mi-cuenta/verificaciones",
  "/anfitrion",
  "/anfitrion/espacios",
  "/anfitrion/espacios/nuevo",
  "/anfitrion/reservas",
  "/reservas",
  "/reservas/nueva",
  "/administracion",
  "/administracion/verificaciones",
  "/administracion/espacios",
  "/administracion/disputas",
  "/administracion/cuentas",
  "/administracion/auditoria",
  "/administracion/global",
]);

export type ReturnTarget = string & { readonly __returnTarget: unique symbol };
const home = "/" as ReturnTarget;

function hasUnsafeCharacters(value: string): boolean {
  return /[\\#\u0000-\u001f\u007f]/.test(value);
}

function parseExploreQuery(search: string): string | null {
  const params = new URLSearchParams(search);
  const raw: RawCatalogParams = {};
  const counts = new Map<string, number>();
  for (const [key, value] of params) {
    if (!catalogQueryKeys.has(key)) return null;
    const count = (counts.get(key) ?? 0) + 1;
    if (count > 1) return null;
    counts.set(key, count);
    raw[key] = value;
  }
  const query = parseCatalogQuery(raw);
  const normalized = serializeCatalogQuery(query);
  if (normalized.has("attribute")) return null;
  for (const [key, value] of params) {
    const canonical = normalized.get(key);
    if (canonical === null) return null;
    if (key === "q") {
      if (canonical !== value.trim()) return null;
    } else if (canonical !== value) {
      return null;
    }
  }
  return normalized.size ? `/explorar?${normalized.toString()}` : "/explorar";
}

export function resolveReturnTarget(
  value: string | null | undefined,
): ReturnTarget {
  if (!value) return home;
  const raw = value.trim();
  if (
    raw !== value ||
    raw.length > 4096 ||
    hasUnsafeCharacters(raw) ||
    raw.startsWith("//") ||
    /^[a-z][a-z\d+.-]*:/i.test(raw) ||
    raw.includes("%2f") ||
    raw.includes("%5c")
  ) {
    return home;
  }

  const question = raw.indexOf("?");
  const pathname = question < 0 ? raw : raw.slice(0, question);
  const search = question < 0 ? "" : raw.slice(question + 1);
  if (!pathname.startsWith("/") || pathname.startsWith("//")) return home;
  if (
    pathname.split("/").some((segment) => segment === "." || segment === "..")
  )
    return home;
  if (pathname === "/explorar") {
    const result = parseExploreQuery(search);
    return (result ?? "/") as ReturnTarget;
  }
  if (search) return home;
  if (fixedPaths.has(pathname)) return pathname as ReturnTarget;

  const category = /^\/categorias\/([^/]+)$/.exec(pathname);
  if (category && categoryCode.test(category[1] ?? ""))
    return pathname as ReturnTarget;
  const space = /^\/espacios\/([^/]+)$/.exec(pathname);
  if (space && canonicalUuid.test(space[1] ?? ""))
    return pathname as ReturnTarget;
  const booking = /^\/reservas\/([^/]+)$/.exec(pathname);
  if (
    booking &&
    (canonicalUuid.test(booking[1] ?? "") ||
      /^res-[a-z0-9_-]+$/i.test(booking[1] ?? ""))
  )
    return pathname as ReturnTarget;
  const hostBooking = /^\/anfitrion\/reservas\/([^/]+)$/.exec(pathname);
  if (
    hostBooking &&
    (canonicalUuid.test(hostBooking[1] ?? "") ||
      /^res-[a-z0-9_-]+$/i.test(hostBooking[1] ?? ""))
  )
    return pathname as ReturnTarget;
  const hostEdit = /^\/anfitrion\/espacios\/([^/]+)\/editar$/.exec(pathname);
  if (hostEdit) return pathname as ReturnTarget;
  return home;
}

export function resolveReturnParam(values: readonly string[]): ReturnTarget {
  return values.length === 1 ? resolveReturnTarget(values[0]) : home;
}
