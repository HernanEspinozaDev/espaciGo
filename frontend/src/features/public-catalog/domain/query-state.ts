import { clpAmount } from "@/lib/api/clp";
import { assertIanaTimeZone, parseRfc3339 } from "@/lib/api/time";
import type {
  CatalogQuery,
  CatalogSort,
  PublicCategoryFilter,
  RawCatalogParams,
} from "./types";

const MAX_INT64 = BigInt("9223372036854775807");
const SORTS = [
  "default",
  "title_asc",
  "title_desc",
  "unit_price_asc",
  "unit_price_desc",
  "relevance",
  "estimated_total_asc",
  "estimated_total_desc",
] as const satisfies readonly CatalogSort[];
const RATE_UNITS = ["hora", "dia", "mes"] as const;
const DECIMAL = /^(0|[1-9][0-9]*)(?:\.[0-9]+)?$/;
const RFC3339_WITH_OFFSET =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/;

function single(params: RawCatalogParams, key: string): string | undefined {
  const value = params[key];
  return typeof value === "string" ? value : undefined;
}

function safeCode(value: string | undefined, maxLength: number) {
  return value && value.length <= maxLength && value === value.trim()
    ? value
    : undefined;
}

function validClp(value: string | undefined): string | undefined {
  if (!value || !/^\d{1,19}$/.test(value)) return undefined;
  try {
    const exact = BigInt(clpAmount(value));
    return exact <= MAX_INT64 ? value : undefined;
  } catch {
    return undefined;
  }
}

function validArea(value: string | undefined): string | undefined {
  if (!value || !/^[1-9][0-9]*(?:\.[0-9]{1,2})?$/.test(value)) return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? value : undefined;
}

function validDecimal(value: string, filter: PublicCategoryFilter) {
  if (!DECIMAL.test(value)) return false;
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) return false;
  if (filter.type === "integer" && !Number.isInteger(numeric)) return false;
  if (filter.minimum !== undefined && numeric < filter.minimum) return false;
  if (filter.maximum !== undefined && numeric > filter.maximum) return false;
  return true;
}

function validAttributeToken(
  token: string,
  filters: readonly PublicCategoryFilter[],
): boolean {
  if (token.length > 180) return false;
  const separator = token.indexOf(":");
  if (separator <= 0) return false;
  const code = token.slice(0, separator);
  const filter = filters.find((item) => item.code === code);
  if (!filter) return false;
  const expression = token.slice(separator + 1);

  if (filter.type === "boolean")
    return expression === "true" || expression === "false";
  if (filter.type === "enum") {
    const match = /^eq:([a-z][a-z0-9_]{0,79})$/.exec(expression);
    return Boolean(match && filter.options?.includes(match[1] ?? ""));
  }
  if (filter.type === "enum_list") {
    const match = /^in:([a-z][a-z0-9_]{0,79}(?:,[a-z][a-z0-9_]{0,79})*)$/.exec(
      expression,
    );
    if (!match) return false;
    const values = match[1]?.split(",") ?? [];
    return (
      new Set(values).size === values.length &&
      values.every((value) => filter.options?.includes(value))
    );
  }
  const match = /^(gte|lte):(.+)$/.exec(expression);
  return Boolean(match && match[2] && validDecimal(match[2], filter));
}

export function sanitizeAttributes(
  raw: string | readonly string[] | undefined,
  filters: readonly PublicCategoryFilter[],
): string[] {
  if (!raw || filters.length === 0) return [];
  const candidates = typeof raw === "string" ? [raw] : raw;
  const accepted = candidates
    .slice(0, 32)
    .filter((token) => validAttributeToken(token, filters));
  const counts = new Map<string, number>();
  for (const token of accepted) {
    const code = token.slice(0, token.indexOf(":"));
    const expression = token.slice(token.indexOf(":") + 1);
    const operator = expression.split(":", 1)[0] ?? expression;
    const filter = filters.find((item) => item.code === code);
    const key =
      filter?.type === "integer" || filter?.type === "number"
        ? `${code}:${operator}`
        : code;
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return accepted.filter((token) => {
    const code = token.slice(0, token.indexOf(":"));
    const expression = token.slice(token.indexOf(":") + 1);
    const operator = expression.split(":", 1)[0] ?? expression;
    const filter = filters.find((item) => item.code === code);
    const key =
      filter?.type === "integer" || filter?.type === "number"
        ? `${code}:${operator}`
        : code;
    return counts.get(key) === 1;
  });
}

function validInterval(params: RawCatalogParams) {
  const start = single(params, "start_at");
  const end = single(params, "end_at");
  if (!start || !end) return {};
  try {
    assertIanaTimeZone("America/Santiago");
    if (!RFC3339_WITH_OFFSET.test(start) || !RFC3339_WITH_OFFSET.test(end))
      return {};
    if (parseRfc3339(end).getTime() <= parseRfc3339(start).getTime()) return {};
    return { start_at: start, end_at: end };
  } catch {
    return {};
  }
}

export function parseCatalogQuery(
  params: RawCatalogParams,
  filters: readonly PublicCategoryFilter[] = [],
): CatalogQuery {
  const query: CatalogQuery = { attribute: [] };
  const q = single(params, "q");
  if (q && q.trim().length <= 120) query.q = q.trim().normalize("NFC");

  const category = safeCode(single(params, "category_code"), 80);
  if (category) query.category_code = category;
  const region = single(params, "region_code");
  if (region && /^\d{2}$/.test(region)) query.region_code = region;
  const comuna = single(params, "comuna_code");
  if (comuna && /^\d{5}$/.test(comuna)) query.comuna_code = comuna;

  const area = validArea(single(params, "min_area_m2"));
  if (area) query.min_area_m2 = area;

  const unit = single(params, "rate_unit");
  if (RATE_UNITS.includes(unit as (typeof RATE_UNITS)[number]))
    query.rate_unit = unit as CatalogQuery["rate_unit"];

  for (const key of [
    "min_unit_price_clp",
    "max_unit_price_clp",
    "min_total_clp",
    "max_total_clp",
  ] as const) {
    const amount = validClp(single(params, key));
    if (amount) query[key] = amount;
  }

  Object.assign(query, validInterval(params));
  const sort = single(params, "sort");
  if (sort && (SORTS as readonly string[]).includes(sort)) {
    const candidate = sort as CatalogSort;
    const unitSort =
      candidate === "unit_price_asc" || candidate === "unit_price_desc";
    const totalSort =
      candidate === "estimated_total_asc" ||
      candidate === "estimated_total_desc";
    if (
      (!unitSort || query.rate_unit) &&
      (candidate !== "relevance" || query.q) &&
      (!totalSort || (query.start_at && query.end_at))
    ) {
      query.sort = candidate;
    }
  }

  const pageSize = single(params, "page_size");
  if (pageSize && /^\d{1,2}$/.test(pageSize)) {
    const parsed = Number(pageSize);
    if (parsed >= 1 && parsed <= 48) query.page_size = parsed;
  }

  const cursor = single(params, "cursor");
  if (cursor && cursor.length <= 4096) query.cursor = cursor;

  if (query.category_code) {
    const attributes = params.attribute;
    query.attribute = sanitizeAttributes(attributes, filters);
  }
  return query;
}

export function serializeCatalogQuery(query: CatalogQuery): URLSearchParams {
  const params = new URLSearchParams();
  const scalarKeys = [
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
  ] as const;
  for (const key of scalarKeys) {
    const value = query[key];
    if (value !== undefined && value !== "") params.set(key, String(value));
  }
  for (const attribute of query.attribute)
    params.append("attribute", attribute);
  return params;
}

export function withoutCursor(query: CatalogQuery): CatalogQuery {
  const next = { ...query };
  delete next.cursor;
  return next;
}

export function hasCatalogFilters(query: CatalogQuery): boolean {
  return Boolean(
    query.q ||
    query.category_code ||
    query.region_code ||
    query.comuna_code ||
    query.min_area_m2 ||
    query.attribute.length ||
    query.rate_unit ||
    query.min_unit_price_clp ||
    query.max_unit_price_clp ||
    query.start_at ||
    query.end_at ||
    query.min_total_clp ||
    query.max_total_clp ||
    query.cursor,
  );
}
