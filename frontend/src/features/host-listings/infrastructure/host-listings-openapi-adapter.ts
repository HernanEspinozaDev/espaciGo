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
  ListingDeletionReceipt,
  ListingDetail,
  ListingDraftInput,
  ListingState,
  ListingSummary,
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

function mapSpaceDraftToDetail(draft: OpenApiSpaceDraft): ListingDetail {
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

function mapWeeklyHours(data: OpenApiLocalWeeklyHours): ScheduleData {
  return {
    space_id: data.space_id,
    enabled: data.enabled,
    time_zone: data.time_zone,
    days: data.days.map((d) => ({
      weekday: d.weekday,
      periods: d.periods.map((p) => ({ open: p.open, close: p.close })),
    })),
  };
}

function mapPhotoToGalleryPhoto(raw: unknown): GalleryPhoto {
  const p = raw as Record<string, unknown>;
  const isSynthetic =
    p.fixture_code === "synthetic-png-v1" || !Array.isArray(p.variants);
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
    created_at:
      typeof p.created_at === "string"
        ? p.created_at
        : new Date().toISOString(),
    is_synthetic: isSynthetic,
    is_cover: Boolean(p.cover),
    visible: p.visible !== false,
    position: typeof p.position === "number" ? p.position : 0,
    alt_text: typeof p.alt_text === "string" ? p.alt_text : undefined,
  };
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
    return mapSpaceDraftToDetail(result.data);
  }

  async createDraft(
    input: ListingDraftInput,
    idempotencyKey?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    const key = idempotencyKey ?? generateIdempotencyKey();
    const result = await apiClient.POST("/spaces", {
      headers: {
        "Idempotency-Key": key,
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
        region_code: input.region_code,
        comuna_code: input.comuna_code,
        attribute_schema_version: input.attribute_schema_version,
        attributes: (input.attributes ?? {}) as Record<string, never>,
      },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapSpaceDraftToDetail(result.data);
  }

  async updateDraft(
    listingId: string,
    input: ListingDraftInput,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    const result = await apiClient.PUT("/spaces/{spaceId}", {
      params: {
        path: { spaceId: listingId },
      },
      headers: ifMatch ? { "If-Match": ifMatch } : undefined,
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
        region_code: input.region_code,
        comuna_code: input.comuna_code,
        attribute_schema_version: input.attribute_schema_version,
        attributes: (input.attributes ?? {}) as Record<string, never>,
      },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapSpaceDraftToDetail(result.data);
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
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    const result = await apiClient.PUT(
      "/spaces/{spaceId}/publication-content",
      {
        params: {
          path: { spaceId: listingId },
        },
        headers: ifMatch ? { "If-Match": ifMatch } : undefined,
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
    return mapSpaceDraftToDetail(result.data);
  }

  async publishListing(
    listingId: string,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    const result = await apiClient.PUT("/spaces/{spaceId}/publication", {
      params: {
        path: { spaceId: listingId },
      },
      headers: ifMatch ? { "If-Match": ifMatch } : undefined,
      body: { state: "activa" },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapSpaceDraftToDetail(result.data);
  }

  async archiveListing(
    listingId: string,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    const result = await apiClient.PUT("/spaces/{spaceId}/publication", {
      params: {
        path: { spaceId: listingId },
      },
      headers: ifMatch ? { "If-Match": ifMatch } : undefined,
      body: { state: "oculta" },
      bodySerializer: serializeExactJson,
      signal,
    });
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data) throw new ApiProtocolError();
    return mapSpaceDraftToDetail(result.data);
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
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<PricingHistoryItem> {
    const result = await apiClient.PUT("/spaces/{spaceId}/tariff", {
      params: {
        path: { spaceId: listingId },
      },
      headers: ifMatch ? { "If-Match": ifMatch } : undefined,
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
    return mapRate(result.data);
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
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<{ listingId: string; timeZone: string }> {
    const result = await apiClient.PUT("/spaces/{spaceId}/availability", {
      params: {
        path: { spaceId: listingId },
      },
      headers: ifMatch ? { "If-Match": ifMatch } : undefined,
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
    };
  }

  async getWeeklySchedule(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<ScheduleData | null> {
    const result = await apiClient.GET(
      "/local/booking-trial/catalog/{spaceId}/weekly-hours",
      {
        params: {
          path: { spaceId: listingId },
        },
        signal,
      },
    );
    if (result.response.status === 404) return null;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return mapWeeklyHours(result.data.data);
  }

  async updateWeeklySchedule(
    listingId: string,
    input: ScheduleInput,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ScheduleData> {
    const days = input.weekly_slots
      ? convertSlotsToWeeklyDays(input.weekly_slots)
      : [];
    const result = await apiClient.PUT(
      "/local/booking-trial/catalog/{spaceId}/weekly-hours",
      {
        params: {
          path: { spaceId: listingId },
        },
        headers: ifMatch ? { "If-Match": ifMatch } : undefined,
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
      },
    );
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
    if (!result.data?.data) throw new ApiProtocolError();
    return mapWeeklyHours(result.data.data);
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
    if (input?.is_cover) {
      await this.setCoverPhoto(listingId, photo.id, signal);
    }
    return photo;
  }

  async setCoverPhoto(
    listingId: string,
    photoId: string,
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.PUT("/spaces/{spaceId}/cover", {
      params: {
        path: { spaceId: listingId },
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
    signal?: AbortSignal,
  ): Promise<void> {
    const result = await apiClient.DELETE(
      "/spaces/{spaceId}/gallery/{photoId}",
      {
        params: {
          path: { spaceId: listingId, photoId },
        },
        signal,
      },
    );
    if (result.response.status === 404) return;
    if (!result.response.ok) {
      throw normalizeApiError(result.response, result.error);
    }
  }

  async reorderPhotos(
    listingId: string,
    photoIds: string[],
    signal?: AbortSignal,
  ): Promise<void> {
    for (let index = 0; index < photoIds.length; index += 1) {
      const photoId = photoIds[index];
      const result = await apiClient.PUT(
        "/spaces/{spaceId}/gallery/{photoId}",
        {
          params: {
            path: { spaceId: listingId, photoId },
          },
          body: { position: index + 1 },
          bodySerializer: serializeExactJson,
          signal,
        },
      );
      if (!result.response.ok) {
        throw normalizeApiError(result.response, result.error);
      }
    }
  }
}
