import "client-only";

import { apiClient } from "@/lib/api/client";
import { ApiProtocolError, normalizeApiError } from "@/lib/api/errors";
import { serializeExactJson } from "@/lib/api/exact-json";
import type { HostListingsPort } from "../application/host-listings.port";
import type {
  Category,
  CategoryAttributeDefinition,
  CategoryProfileSchema,
  ContentUpdateInput,
  GalleryPhoto,
  GalleryUploadStatus,
  ListingDeletionReceipt,
  ListingDetail,
  ListingDraftInput,
  ListingState,
  ListingSummary,
  ManualAvailabilityBlock,
  ManualAvailabilityBlockInput,
  OpenApiManualBlock,
  OpenApiLocalWeeklyHours,
  OpenApiRate,
  OpenApiSpaceAttributeProfile,
  OpenApiSpaceCategory,
  OpenApiSpaceDraft,
  PhotoUploadInput,
  PricingHistoryItem,
  PricingInput,
  ScheduleData,
  ScheduleInput,
} from "../domain/types";
import {
  convertSlotsToWeeklyDays,
  generateIdempotencyKey,
  isValidUUID,
} from "../domain/validation";

function mapSpaceCategory(item: OpenApiSpaceCategory): Category {
  return {
    code: item.code,
    name: item.name,
  };
}

function mapAttributeProfile(
  profile: OpenApiSpaceAttributeProfile,
): CategoryProfileSchema {
  return {
    category_code: profile.category_code,
    schema_version: profile.schema_version,
    attributes: profile.attributes.map((attr) => ({
      code: attr.code,
      label: attr.label,
      description: attr.description,
      type: attr.type,
      required: attr.required,
      order: attr.order,
      unit: attr.unit,
      options: attr.options,
      minimum: attr.minimum,
      maximum: attr.maximum,
      step: attr.step,
      filter_candidate: attr.filter_candidate,
    })) as readonly CategoryAttributeDefinition[],
  };
}

function requireStrongEtag(value: string | null): string {
  if (!value || !/^"[\x21\x23-\x7e\x80-\xff]+"$/.test(value)) {
    throw new ApiProtocolError();
  }
  return value;
}

function mapSpaceDraftToDetail(
  draft: OpenApiSpaceDraft,
  etag?: string,
): ListingDetail {
  return {
    id: draft.id,
    category_code: draft.category_code,
    category_name: draft.category_name,
    title: draft.title,
    description: draft.description,
    area_m2: draft.area_m2,
    capacity: draft.capacity,
    usage_rules: draft.usage_rules,
    rate_unit: draft.rate_unit,
    base_price_clp: draft.base_price_clp,
    address: draft.address,
    state: draft.state,
    time_zone: draft.time_zone ?? null,
    region_code: draft.region_code,
    comuna_code: draft.comuna_code,
    attribute_schema_version: draft.attribute_schema_version,
    attributes: Object.freeze(
      (draft.attributes as Record<string, unknown>) ?? {},
    ),
    cover_photo_id: draft.cover_photo_id ?? null,
    ...(etag ? { etag } : {}),
  };
}

function mapSpaceDraftToSummary(draft: OpenApiSpaceDraft): ListingSummary {
  return {
    id: draft.id,
    category_code: draft.category_code,
    category_name: draft.category_name,
    title: draft.title,
    state: draft.state,
    base_price_clp: draft.base_price_clp,
    rate_unit: draft.rate_unit,
    area_m2: draft.area_m2,
    capacity: draft.capacity,
    cover_photo_id: draft.cover_photo_id ?? null,
    time_zone: draft.time_zone ?? null,
    region_code: draft.region_code,
    comuna_code: draft.comuna_code,
  };
}

function mapRate(rate: OpenApiRate): PricingHistoryItem {
  return {
    space_id: rate.space_id,
    version: rate.version,
    rate_unit: rate.rate_unit,
    base_price: rate.base_price,
    currency: rate.currency,
    created_at: rate.created_at,
  };
}

function mapWeeklyHours(
  data: OpenApiLocalWeeklyHours,
  etag: string,
): ScheduleData {
  return {
    etag,
    space_id: data.space_id,
    enabled: data.enabled,
    time_zone: data.time_zone,
    days: data.days.map((d) => ({
      weekday: d.weekday,
      periods: d.periods.map((p) => ({ open: p.open, close: p.close })),
    })),
  };
}

function mapManualBlock(block: OpenApiManualBlock): ManualAvailabilityBlock {
  return {
    id: block.id,
    space_id: block.space_id,
    start_at: block.start_at,
    end_at: block.end_at,
    time_zone: block.time_zone,
    reason: block.reason,
    created_at: block.created_at,
  };
}

function isGoTimeDateTime(value: unknown): value is string {
  if (typeof value !== "string") return false;
  // Gallery created_at is serialized from Go time.Time, which cannot represent leap seconds.
  const match =
    /^(\d{4})-(\d{2})-(\d{2})[Tt](\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(Z|z|[+-](\d{2}):(\d{2}))$/.exec(
      value,
    );
  if (!match) return false;

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const hour = Number(match[4]);
  const minute = Number(match[5]);
  const second = Number(match[6]);
  const offsetHour = match[8] ? Number(match[8]) : 0;
  const offsetMinute = match[9] ? Number(match[9]) : 0;
  const leapYear = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
  const maxDay =
    month === 2
      ? leapYear
        ? 29
        : 28
      : [4, 6, 9, 11].includes(month)
        ? 30
        : 31;

  return (
    month >= 1 &&
    month <= 12 &&
    day >= 1 &&
    day <= maxDay &&
    hour <= 23 &&
    minute <= 59 &&
    second <= 59 &&
    offsetHour <= 23 &&
    offsetMinute <= 59 &&
    Number.isFinite(Date.parse(value))
  );
}

function mapPhotoToGalleryPhoto(raw: unknown): GalleryPhoto {
  if (typeof raw !== "object" || raw === null || Array.isArray(raw))
    throw new ApiProtocolError();
  const p = raw as Record<string, unknown>;
  const isSynthetic = p.fixture_code === "synthetic-png-v1";
  const state =
    p.state === "pending" || p.state === "ready" ? p.state : undefined;
  const createdAt = p.created_at;
  const validCreatedAt = isGoTimeDateTime(createdAt) ? createdAt : undefined;
  if (
    (state !== "pending" && validCreatedAt === undefined) ||
    (createdAt !== undefined && validCreatedAt === undefined)
  )
    throw new ApiProtocolError();
  if (
    !isSynthetic &&
    (state === undefined ||
      typeof p.id !== "string" ||
      !isValidUUID(p.id) ||
      p.id.trim() !== p.id ||
      typeof p.space_id !== "string" ||
      !isValidUUID(p.space_id) ||
      p.space_id.trim() !== p.space_id)
  )
    throw new ApiProtocolError();
  if (
    !isSynthetic &&
    state === "ready" &&
    (typeof p.alt_text !== "string" ||
      p.alt_text.length > 250 ||
      typeof p.visible !== "boolean" ||
      typeof p.cover !== "boolean" ||
      typeof p.position !== "number" ||
      !Number.isInteger(p.position) ||
      p.position < 1 ||
      p.position > 30)
  )
    throw new ApiProtocolError();
  const variants =
    !isSynthetic && state === "ready"
      ? mapPhotoVariants(p.variants, p.space_id, p.id)
      : [];
  return {
    id: String(p.id ?? ""),
    space_id: p.space_id ? String(p.space_id) : undefined,
    mime_type:
      typeof p.mime_type === "string"
        ? p.mime_type
        : isSynthetic
          ? "image/png"
          : "image/webp",
    sha256: typeof p.sha256 === "string" ? p.sha256 : "",
    size_bytes: typeof p.size_bytes === "number" ? p.size_bytes : 0,
    ...(validCreatedAt !== undefined ? { created_at: validCreatedAt } : {}),
    is_synthetic: isSynthetic,
    is_cover: Boolean(p.cover),
    visible: p.visible !== false,
    position: typeof p.position === "number" ? p.position : 0,
    alt_text: typeof p.alt_text === "string" ? p.alt_text : undefined,
    variants,
    ...(state ? { state } : {}),
  };
}

function mapPhotoVariants(
  value: unknown,
  spaceId: unknown,
  photoId: unknown,
): NonNullable<GalleryPhoto["variants"]> {
  if (
    !Array.isArray(value) ||
    value.length < 1 ||
    value.length > 2 ||
    typeof spaceId !== "string" ||
    typeof photoId !== "string" ||
    !isValidUUID(spaceId) ||
    spaceId.trim() !== spaceId ||
    !isValidUUID(photoId) ||
    photoId.trim() !== photoId
  )
    throw new ApiProtocolError();
  return value.map((item) => {
    if (!item || typeof item !== "object" || Array.isArray(item))
      throw new ApiProtocolError();
    const variant = item as Record<string, unknown>;
    const name = variant.name;
    if (name !== "thumbnail" && name !== "display")
      throw new ApiProtocolError();
    const expected = `/api/v1/spaces/${spaceId}/gallery/${photoId}/variants/${name}`;
    if (variant.mime_type !== "image/webp" || variant.url !== expected)
      throw new ApiProtocolError();
    const sizeBytes = variant.size_bytes;
    const sha256 = variant.sha256;
    const width = variant.width;
    const height = variant.height;
    if (
      typeof sizeBytes !== "number" ||
      !Number.isInteger(sizeBytes) ||
      sizeBytes < 1 ||
      sizeBytes > 10_485_760 ||
      typeof sha256 !== "string" ||
      !/^[0-9a-f]{64}$/.test(sha256) ||
      typeof width !== "number" ||
      !Number.isInteger(width) ||
      width < 1 ||
      width > 1600 ||
      typeof height !== "number" ||
      !Number.isInteger(height) ||
      height < 1 ||
      height > 1600
    )
      throw new ApiProtocolError();
    return {
      name,
      url: `/api/backend/spaces/${spaceId}/gallery/${photoId}/variants/${name}`,
      mime_type: "image/webp",
      size_bytes: sizeBytes,
      sha256,
      width,
      height,
    };
  });
}

export class OpenApiHostListingsAdapter implements HostListingsPort {
  async listCategories(signal?: AbortSignal): Promise<Category[]> {
    const result = await apiClient.GET("/spaces/categories", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.items) throw new ApiProtocolError();
    return result.data.items.map(mapSpaceCategory);
  }

  async getCategoryProfile(
    categoryCode: string,
    version?: number,
    signal?: AbortSignal,
  ): Promise<CategoryProfileSchema | null> {
    if (version !== undefined) {
      const result = await apiClient.GET(
        "/spaces/categories/{categoryCode}/attributes/{version}",
        {
          params: {
            path: { categoryCode, version },
          },
          signal,
        },
      );
      if (result.response.status === 404) return null;
      if (!result.response.ok) {
        throw normalizeApiError(result.response, result.error);
      }
      if (!result.data) throw new ApiProtocolError();
      return mapAttributeProfile(result.data);
    }

    const result = await apiClient.GET(
      "/spaces/categories/{categoryCode}/attributes",
      {
        params: {
          path: { categoryCode },
        },
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapAttributeProfile(result.data);
  }

  async listListings(
    filter?: { status?: ListingState },
    signal?: AbortSignal,
  ): Promise<ListingSummary[]> {
    const result = await apiClient.GET("/spaces", {
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.items) throw new ApiProtocolError();
    let items = result.data.items.map(mapSpaceDraftToSummary);
    if (filter?.status) {
      items = items.filter((item) => item.state === filter.status);
    }
    return items;
  }

  async getListing(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail | null> {
    const result = await apiClient.GET("/spaces/{spaceId}", {
      params: {
        path: { spaceId: listingId },
      },
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapSpaceDraftToDetail(
      result.data,
      requireStrongEtag(result.response.headers.get("ETag")),
    );
  }

  async createDraft(
    input: ListingDraftInput,
    idempotencyKey?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    void idempotencyKey;
    const result = await apiClient.POST("/spaces", {
      body: {
        category_code: input.category_code,
        title: input.title,
        description: input.description,
        area_m2: input.area_m2,
        capacity: input.capacity,
        usage_rules: input.usage_rules,
        rate_unit: input.rate_unit,
        base_price_clp: input.base_price_clp,
        address: input.address,
        ...(input.region_code === undefined
          ? {}
          : { region_code: input.region_code }),
        ...(input.comuna_code === undefined
          ? {}
          : { comuna_code: input.comuna_code }),
        ...(input.attribute_schema_version === undefined
          ? {}
          : { attribute_schema_version: input.attribute_schema_version }),
        attributes: (input.attributes ?? {}) as Record<string, never>,
      },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapSpaceDraftToDetail(
      result.data,
      requireStrongEtag(result.response.headers.get("ETag")),
    );
  }

  async updateDraft(
    listingId: string,
    input: ListingDraftInput,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    const result = await apiClient.PUT("/spaces/{spaceId}", {
      params: {
        path: { spaceId: listingId },
        header: { "If-Match": requireStrongEtag(ifMatch) },
      },
      body: {
        category_code: input.category_code,
        title: input.title,
        description: input.description,
        area_m2: input.area_m2,
        capacity: input.capacity,
        usage_rules: input.usage_rules,
        rate_unit: input.rate_unit,
        base_price_clp: input.base_price_clp,
        address: input.address,
        ...(input.region_code === undefined
          ? {}
          : { region_code: input.region_code }),
        ...(input.comuna_code === undefined
          ? {}
          : { comuna_code: input.comuna_code }),
        ...(input.attribute_schema_version === undefined
          ? {}
          : { attribute_schema_version: input.attribute_schema_version }),
        attributes: (input.attributes ?? {}) as Record<string, never>,
      },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapSpaceDraftToDetail(
      result.data,
      requireStrongEtag(result.response.headers.get("ETag")),
    );
  }

  async deleteListing(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<ListingDeletionReceipt> {
    const result = await apiClient.DELETE("/spaces/{spaceId}", {
      params: {
        path: { spaceId: listingId },
      },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return {
      space_id: result.data.space_id,
      state: result.data.state,
      occurred_at: result.data.occurred_at,
      removed: Object.freeze(result.data.removed),
      preserved: Object.freeze(result.data.preserved),
      files_pending_cleanup: result.data.files_pending_cleanup,
      reused: result.data.reused,
    };
  }

  async updateContent(
    listingId: string,
    input: ContentUpdateInput,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    const result = await apiClient.PUT(
      "/spaces/{spaceId}/publication-content",
      {
        params: {
          path: { spaceId: listingId },
          header: { "If-Match": requireStrongEtag(ifMatch ?? null) },
        },
        body: {
          title: input.title,
          description: input.description,
          capacity: input.capacity,
          usage_rules: input.usage_rules,
          base_price_clp: input.base_price_clp,
          region_code: input.region_code,
          comuna_code: input.comuna_code,
        },
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapSpaceDraftToDetail(
      result.data,
      requireStrongEtag(result.response.headers.get("ETag")),
    );
  }

  async publishListing(
    listingId: string,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    const result = await apiClient.PUT("/spaces/{spaceId}/publication", {
      params: {
        path: { spaceId: listingId },
        header: { "If-Match": requireStrongEtag(ifMatch ?? null) },
      },
      body: { state: "activa" },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapSpaceDraftToDetail(
      result.data,
      requireStrongEtag(result.response.headers.get("ETag")),
    );
  }

  async archiveListing(
    listingId: string,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    const result = await apiClient.PUT("/spaces/{spaceId}/publication", {
      params: {
        path: { spaceId: listingId },
        header: { "If-Match": requireStrongEtag(ifMatch ?? null) },
      },
      body: { state: "oculta" },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapSpaceDraftToDetail(
      result.data,
      requireStrongEtag(result.response.headers.get("ETag")),
    );
  }

  async getPricing(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<PricingHistoryItem | null> {
    const result = await apiClient.GET("/spaces/{spaceId}/tariff", {
      params: {
        path: { spaceId: listingId },
      },
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapRate(result.data);
  }

  async updatePricing(
    listingId: string,
    input: PricingInput,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<PricingHistoryItem> {
    const result = await apiClient.PUT("/spaces/{spaceId}/tariff", {
      params: {
        path: { spaceId: listingId },
        header: { "If-Match": requireStrongEtag(ifMatch) },
      },
      body: {
        rate_unit: input.rate_unit,
        base_price: input.base_price,
      },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return {
      ...mapRate(result.data),
      etag: requireStrongEtag(result.response.headers.get("ETag")),
    };
  }

  async listPricingHistory(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<PricingHistoryItem[]> {
    const result = await apiClient.GET("/spaces/{spaceId}/tariffs", {
      params: {
        path: { spaceId: listingId },
      },
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.items) throw new ApiProtocolError();
    return result.data.items.map(mapRate);
  }

  async configureTimeZone(
    listingId: string,
    timeZone: string,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<{ listingId: string; timeZone: string; etag: string }> {
    const result = await apiClient.PUT("/spaces/{spaceId}/availability", {
      params: {
        path: { spaceId: listingId },
        header: { "If-Match": requireStrongEtag(ifMatch) },
      },
      body: { time_zone: timeZone },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return {
      listingId: result.data.space_id,
      timeZone: result.data.time_zone,
      etag: requireStrongEtag(result.response.headers.get("ETag")),
    };
  }

  async getWeeklySchedule(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<ScheduleData | null> {
    const result = await apiClient.GET("/spaces/{spaceId}/weekly-hours", {
      params: {
        path: { spaceId: listingId },
      },
      signal,
    });
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return mapWeeklyHours(
      result.data.data,
      requireStrongEtag(result.response.headers.get("ETag")),
    );
  }

  async updateWeeklySchedule(
    listingId: string,
    input: ScheduleInput,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<ScheduleData> {
    const days = input.weekly_slots
      ? convertSlotsToWeeklyDays(input.weekly_slots)
      : [];
    const result = await apiClient.PUT("/spaces/{spaceId}/weekly-hours", {
      params: {
        path: { spaceId: listingId },
        header: { "If-Match": requireStrongEtag(ifMatch) },
      },
      body: {
        enabled: input.enabled ?? true,
        days: days.map((d) => ({
          weekday: d.weekday,
          periods: d.periods.map((p) => ({
            open: p.open,
            close: p.close,
          })),
        })),
      },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return mapWeeklyHours(
      result.data.data,
      requireStrongEtag(result.response.headers.get("ETag")),
    );
  }

  async listPhotos(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<GalleryPhoto[]> {
    const result = await apiClient.GET("/spaces/{spaceId}/gallery", {
      params: {
        path: { spaceId: listingId },
      },
      signal,
    });
    if (result.response.status === 404) return [];
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    const items = result.data?.items ?? [];
    return items.map(mapPhotoToGalleryPhoto);
  }

  async listManualBlocks(
    listingId: string,
    from: string,
    to: string,
    signal?: AbortSignal,
  ): Promise<ManualAvailabilityBlock[]> {
    const result = await apiClient.GET(
      "/spaces/{spaceId}/availability/blocks",
      {
        params: {
          path: { spaceId: listingId },
          query: { from, to },
        },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.items) throw new ApiProtocolError();
    return result.data.items.map(mapManualBlock);
  }

  async createManualBlock(
    listingId: string,
    input: ManualAvailabilityBlockInput,
    signal?: AbortSignal,
  ): Promise<ManualAvailabilityBlock> {
    const result = await apiClient.POST(
      "/spaces/{spaceId}/availability/blocks",
      {
        params: { path: { spaceId: listingId } },
        body: input,
        bodySerializer: serializeExactJson,
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapManualBlock(result.data);
  }

  async deleteManualBlock(
    listingId: string,
    blockId: string,
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.DELETE(
      "/spaces/{spaceId}/availability/blocks/{blockId}",
      {
        params: { path: { spaceId: listingId, blockId } },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async getPhotoUploadStatus(
    listingId: string,
    idempotencyKey: string,
    signal?: AbortSignal,
  ): Promise<GalleryUploadStatus> {
    const result = await apiClient.GET(
      "/spaces/{spaceId}/gallery/upload-status",
      {
        params: {
          header: { "Idempotency-Key": idempotencyKey },
          path: { spaceId: listingId },
        },
        signal,
      },
    );
    if (result.response.status === 404) return { state: "not_found" };
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.photo_id || !result.data.state) {
      throw new ApiProtocolError();
    }
    return { photo_id: result.data.photo_id, state: result.data.state };
  }

  async uploadPhoto(
    listingId: string,
    input?: PhotoUploadInput,
    idempotencyKey?: string,
    signal?: AbortSignal,
  ): Promise<GalleryPhoto> {
    const key = idempotencyKey ?? generateIdempotencyKey();
    let bodyParam: { image: string } | undefined;
    if (input?.image) {
      const formData = new FormData();
      formData.append("image", input.image);
      bodyParam = formData as unknown as { image: string };
    }

    const result = await apiClient.POST("/spaces/{spaceId}/gallery", {
      params: {
        header: {
          "Idempotency-Key": key,
        },
        path: {
          spaceId: listingId,
        },
      },
      body: bodyParam,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.photo) throw new ApiProtocolError();
    const photo = mapPhotoToGalleryPhoto(result.data.photo);
    if (input?.is_cover && photo.state !== "pending") {
      await this.setCoverPhoto(listingId, photo.id, signal);
    }
    return photo;
  }

  async replacePhoto(
    listingId: string,
    photoId: string,
    input: PhotoUploadInput,
    idempotencyKey: string,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<GalleryPhoto> {
    if (!input.image) throw new ApiProtocolError();
    const formData = new FormData();
    formData.append("image", input.image);
    const result = await apiClient.PUT(
      "/spaces/{spaceId}/gallery/{photoId}/content",
      {
        params: {
          header: {
            "Idempotency-Key": idempotencyKey || generateIdempotencyKey(),
            "If-Match": requireStrongEtag(ifMatch),
          },
          path: { spaceId: listingId, photoId },
        },
        body: formData as unknown as { image: string },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.photo) throw new ApiProtocolError();
    return mapPhotoToGalleryPhoto(result.data.photo);
  }

  async setCoverPhoto(
    listingId: string,
    photoId: string,
    signal?: AbortSignal,
    ifMatch?: string,
  ): Promise<void> {
    const result = await apiClient.PUT("/spaces/{spaceId}/cover", {
      params: {
        path: { spaceId: listingId },
        header: { "If-Match": requireStrongEtag(ifMatch ?? null) },
      },
      body: { photo_id: photoId },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async removePhoto(
    listingId: string,
    photoId: string,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<string> {
    const result = await apiClient.DELETE(
      "/spaces/{spaceId}/gallery/{photoId}",
      {
        params: {
          path: { spaceId: listingId, photoId },
          header: { "If-Match": requireStrongEtag(ifMatch) },
        },
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return requireStrongEtag(result.response.headers.get("ETag"));
  }

  async movePhoto(
    listingId: string,
    photoId: string,
    targetPosition: number,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<string> {
    const result = await apiClient.PUT("/spaces/{spaceId}/gallery/{photoId}", {
      params: {
        path: { spaceId: listingId, photoId },
        header: { "If-Match": requireStrongEtag(ifMatch) },
      },
      body: { position: targetPosition },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    return requireStrongEtag(result.response.headers.get("ETag"));
  }

  async getPhotoVariant(
    listingId: string,
    photoId: string,
    variant: "thumbnail" | "display",
    signal?: AbortSignal,
  ): Promise<Blob> {
    const result = await apiClient.GET(
      "/spaces/{spaceId}/gallery/{photoId}/variants/{variant}",
      {
        params: {
          path: { spaceId: listingId, photoId, variant },
        },
        parseAs: "blob",
        signal,
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!(result.data instanceof Blob)) throw new ApiProtocolError();
    return result.data;
  }
}
