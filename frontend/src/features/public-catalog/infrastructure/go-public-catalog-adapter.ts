import "server-only";

import { getApiBackendBaseUrl } from "@/lib/api/server-config";
import { getCloudRunIdentityHeadersForBackend } from "@/lib/api/cloud-run-identity";
import { serializeCatalogQuery } from "../domain/query-state";
import type {
  CatalogFailure,
  CatalogQuery,
  CatalogResult,
  PublicCategoryFilters,
  PublicLocations,
  PublicSpace,
  PublicSpaceCategories,
  PublicSpaceDetail,
  PublicSpacePage,
  TermsResponse,
} from "../domain/types";
import type { PublicCatalogPort } from "../application/catalog-port";
import { publicVerification } from "../domain/verification";
import type { components } from "@/lib/api/generated/openapi";

const publicSpaceFields = [
  "space_id",
  "category_code",
  "category_name",
  "title",
  "description",
  "area_m2",
  "rate_unit",
  "base_price_clp",
  "currency",
  "region_code",
  "region_name",
  "comuna_code",
  "comuna_name",
  "estimated_total_clp",
  "cover_image_id",
  "features",
  "images",
  "verification",
] as const satisfies readonly (keyof components["schemas"]["PublicSpace"])[];

const publicSpaceDetailFields = [
  "space_id",
  "category_code",
  "category_name",
  "title",
  "description",
  "area_m2",
  "rate_unit",
  "base_price_clp",
  "currency",
  "region_code",
  "region_name",
  "comuna_code",
  "comuna_name",
  "estimated_total_clp",
  "usage_rules",
  "features",
  "images",
  "weekly_schedule",
  "verification",
] as const satisfies readonly (keyof components["schemas"]["PublicSpaceDetail"])[];

function publicRecord(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined;
}

function malformedCatalogResponse<T>(): CatalogResult<T> {
  return { ok: false, error: { kind: "unavailable", status: 502 } };
}

function pickPublicFields(
  value: object,
  fields: readonly string[],
): Record<string, unknown> {
  const source = value as Record<string, unknown>;
  return Object.fromEntries(
    fields.flatMap((field) =>
      Object.hasOwn(source, field) ? [[field, source[field]]] : [],
    ),
  );
}

function publicFeatures(value: unknown): PublicSpace["features"] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item) => {
    const feature = publicRecord(item);
    if (
      !feature ||
      typeof feature.code !== "string" ||
      typeof feature.label !== "string" ||
      typeof feature.value !== "boolean"
    )
      return [];
    return [{ code: feature.code, label: feature.label, value: feature.value }];
  });
}

function publicImages(value: unknown): PublicSpace["images"] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item) => {
    const image = publicRecord(item);
    if (
      !image ||
      typeof image.image_id !== "string" ||
      typeof image.is_cover !== "boolean" ||
      (image.source !== "synthetic" && image.source !== "media")
    )
      return [];
    return [
      {
        image_id: image.image_id,
        is_cover: image.is_cover,
        source: image.source,
      },
    ];
  });
}

function isPublicTimeZone(value: unknown): value is string {
  if (
    typeof value !== "string" ||
    value.trim() === "" ||
    value.trim() !== value
  )
    return false;
  try {
    new Intl.DateTimeFormat("en-US", { timeZone: value }).format(0);
    return true;
  } catch {
    return false;
  }
}

function publicWeeklySchedule(
  value: unknown,
): components["schemas"]["PublicWeeklySchedule"] | undefined {
  const schedule = publicRecord(value);
  const timeZone = schedule?.time_zone;
  if (
    !schedule ||
    typeof schedule.enabled !== "boolean" ||
    !isPublicTimeZone(timeZone) ||
    !Array.isArray(schedule.days) ||
    schedule.days.length !== 7
  )
    return undefined;

  const weekdays = new Set<number>();
  const toMinutes = (time: string) =>
    time === "24:00"
      ? 24 * 60
      : Number(time.slice(0, 2)) * 60 + Number(time.slice(3));
  const days = schedule.days.map((rawDay) => {
    const day = publicRecord(rawDay);
    if (
      !day ||
      typeof day.weekday !== "number" ||
      !Number.isInteger(day.weekday) ||
      day.weekday < 1 ||
      day.weekday > 7 ||
      weekdays.has(day.weekday) ||
      !Array.isArray(day.periods) ||
      day.periods.length > 2
    )
      return undefined;
    weekdays.add(day.weekday);
    const periods = day.periods.map((rawPeriod) => {
      const period = publicRecord(rawPeriod);
      if (
        !period ||
        typeof period.open !== "string" ||
        !/^(?:[01]\d|2[0-3]):[0-5]\d$/.test(period.open) ||
        typeof period.close !== "string" ||
        !/^(?:(?:[01]\d|2[0-3]):[0-5]\d|24:00)$/.test(period.close)
      )
        return undefined;
      const openMinutes = toMinutes(period.open);
      const closeMinutes = toMinutes(period.close);
      if (closeMinutes <= openMinutes) return undefined;
      return {
        open: period.open,
        close: period.close,
        openMinutes,
        closeMinutes,
      };
    });
    if (periods.some((period) => period === undefined)) return undefined;
    const validPeriods = periods as {
      open: string;
      close: string;
      openMinutes: number;
      closeMinutes: number;
    }[];
    const orderedPeriods = [...validPeriods].sort(
      (left, right) => left.openMinutes - right.openMinutes,
    );
    for (let index = 1; index < orderedPeriods.length; index++) {
      if (
        orderedPeriods[index]!.openMinutes <
        orderedPeriods[index - 1]!.closeMinutes
      )
        return undefined;
    }
    return {
      weekday: day.weekday,
      periods: validPeriods.map(({ open, close }) => ({ open, close })),
    };
  });
  if (days.some((day) => day === undefined) || weekdays.size !== 7)
    return undefined;
  return {
    enabled: schedule.enabled,
    time_zone: timeZone,
    days: days as components["schemas"]["PublicWeeklySchedule"]["days"],
  };
}

function adaptPublicSpace(
  space: components["schemas"]["PublicSpace"],
): PublicSpace {
  const details = pickPublicFields(space, publicSpaceFields);
  return {
    ...details,
    features: publicFeatures(space.features),
    images: publicImages(space.images),
    verification: publicVerification(space.verification),
  } as PublicSpace;
}

function adaptPublicSpaceDetail(
  space: components["schemas"]["PublicSpaceDetail"],
): PublicSpaceDetail {
  const details = pickPublicFields(space, publicSpaceDetailFields);
  const schedule = publicWeeklySchedule(space.weekly_schedule);
  if (schedule) details.weekly_schedule = schedule;
  else delete details.weekly_schedule;
  return {
    ...details,
    features: publicFeatures(space.features),
    images: publicImages(space.images),
    verification: publicVerification(space.verification),
  } as PublicSpaceDetail;
}

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
    const idHeaders = await getCloudRunIdentityHeadersForBackend(base);
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
    return requestJson<components["schemas"]["PublicSpacePage"]>(
      pathWithQuery("/public/spaces", serializeCatalogQuery(query)),
    ).then((result): CatalogResult<PublicSpacePage> => {
      if (!result.ok) return result;
      const page = publicRecord(result.value);
      if (
        !page ||
        !Array.isArray(page.items) ||
        page.items.some((item) => !publicRecord(item))
      )
        return malformedCatalogResponse();
      return {
        ok: true,
        value: {
          ...(result.value as unknown as components["schemas"]["PublicSpacePage"]),
          items: page.items.map((item) =>
            adaptPublicSpace(
              item as unknown as components["schemas"]["PublicSpace"],
            ),
          ),
        },
      };
    });
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
    return requestJson<{ data: components["schemas"]["PublicSpaceDetail"] }>(
      `/public/spaces/${encodeURIComponent(spaceId)}`,
    ).then((result): CatalogResult<PublicSpaceDetail> => {
      if (!result.ok) return result;
      const envelope = publicRecord(result.value);
      const data = publicRecord(envelope?.data);
      if (!data) return malformedCatalogResponse();
      return {
        ok: true,
        value: adaptPublicSpaceDetail(
          data as unknown as components["schemas"]["PublicSpaceDetail"],
        ),
      };
    });
  },
  listTerms() {
    return requestJson<TermsResponse>("/auth/terms");
  },
};
