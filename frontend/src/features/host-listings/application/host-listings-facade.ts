"use client";

import "client-only";

import type { AccessToken } from "@/features/auth/application/auth.port";
import type {
  Category,
  CategoryProfileSchema,
  ContentUpdateInput,
  GalleryPhoto,
  ListingDeletionReceipt,
  ListingDetail,
  ListingDraftInput,
  ListingState,
  ListingSummary,
  PhotoUploadInput,
  PricingHistoryItem,
  PricingInput,
  ScheduleData,
  ScheduleInput,
} from "../domain/types";
import type { HostListingsPort } from "./host-listings.port";

export interface HostListingsSnapshot {
  readonly categories: readonly Category[];
  readonly categorySchemas: Readonly<Record<string, CategoryProfileSchema>>;
  readonly listings: readonly ListingSummary[];
  readonly selectedListing: ListingDetail | null;
  readonly activeTariff: PricingHistoryItem | null;
  readonly tariffHistory: readonly PricingHistoryItem[];
  readonly weeklySchedule: ScheduleData | null;
  readonly photos: readonly GalleryPhoto[];
  readonly filter: {
    readonly status: ListingState | "all";
    readonly categoryCode?: string;
  };
  readonly loading: boolean;
  readonly error: string | null;
}

const initialSnapshot: HostListingsSnapshot = Object.freeze({
  categories: Object.freeze([]),
  categorySchemas: Object.freeze({}),
  listings: Object.freeze([]),
  selectedListing: null,
  activeTariff: null,
  tariffHistory: Object.freeze([]),
  weeklySchedule: null,
  photos: Object.freeze([]),
  filter: Object.freeze({
    status: "all",
  }),
  loading: false,
  error: null,
});

export class HostListingsFacade {
  #port: HostListingsPort;
  #abortController: AbortController | null = null;
  #snapshot: HostListingsSnapshot = initialSnapshot;
  #listeners = new Set<() => void>();

  constructor(port: HostListingsPort) {
    this.#port = port;
  }

  getSnapshot = (): HostListingsSnapshot => {
    return this.#snapshot;
  };

  getServerSnapshot = (): HostListingsSnapshot => {
    return initialSnapshot;
  };

  subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => {
      this.#listeners.delete(listener);
    };
  };

  cancel(): void {
    if (this.#abortController) {
      this.#abortController.abort();
      this.#abortController = null;
    }
  }

  #setSnapshot(partial: Partial<HostListingsSnapshot>): void {
    this.#snapshot = Object.freeze({
      ...this.#snapshot,
      ...partial,
    });
    for (const listener of this.#listeners) {
      listener();
    }
  }

  setFilter(filter: Partial<HostListingsSnapshot["filter"]>): void {
    this.#setSnapshot({
      filter: Object.freeze({
        ...this.#snapshot.filter,
        ...filter,
      }),
    });
  }

  async loadCategories(signal?: AbortSignal): Promise<readonly Category[]> {
    try {
      const categories = await this.#port.listCategories(signal);
      this.#setSnapshot({
        categories: Object.freeze(categories),
      });
      return categories;
    } catch (err) {
      if (signal?.aborted) throw err;
      const message =
        err instanceof Error ? err.message : "Error al cargar categorías";
      this.#setSnapshot({ error: message });
      throw err;
    }
  }

  async loadCategorySchema(
    categoryCode: string,
    version?: number,
    signal?: AbortSignal,
  ): Promise<CategoryProfileSchema | null> {
    const cacheKey =
      version !== undefined ? `${categoryCode}@${version}` : categoryCode;
    if (this.#snapshot.categorySchemas[cacheKey]) {
      return this.#snapshot.categorySchemas[cacheKey];
    }

    try {
      const schema = await this.#port.getCategoryProfile(
        categoryCode,
        version,
        signal,
      );
      if (schema) {
        this.#setSnapshot({
          categorySchemas: Object.freeze({
            ...this.#snapshot.categorySchemas,
            [cacheKey]: schema,
            ...(version === undefined ? { [categoryCode]: schema } : {}),
          }),
        });
      }
      return schema;
    } catch (err) {
      if (signal?.aborted) throw err;
      const message =
        err instanceof Error
          ? err.message
          : `Error al cargar perfil para ${categoryCode}`;
      this.#setSnapshot({ error: message });
      throw err;
    }
  }

  async loadListings(
    token: AccessToken,
    filter?: { status?: ListingState },
  ): Promise<readonly ListingSummary[]> {
    this.cancel();
    this.#abortController = new AbortController();
    const signal = this.#abortController.signal;

    this.#setSnapshot({ loading: true, error: null });
    try {
      const listings = await this.#port.listListings(token, filter, signal);
      this.#setSnapshot({
        listings: Object.freeze(listings),
        loading: false,
        error: null,
      });
      return listings;
    } catch (err) {
      if (signal.aborted) throw err;
      const message =
        err instanceof Error ? err.message : "Error al cargar publicaciones";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#abortController = null;
    }
  }

  async selectListing(
    token: AccessToken,
    listingId: string,
  ): Promise<ListingDetail | null> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const [detail, tariff, schedule, photos] = await Promise.all([
        this.#port.getListing(token, listingId),
        this.#port.getPricing(token, listingId).catch(() => null),
        this.#port.getWeeklySchedule(token, listingId).catch(() => null),
        this.#port.listPhotos(token, listingId).catch(() => []),
      ]);

      this.#setSnapshot({
        selectedListing: detail,
        activeTariff: tariff,
        weeklySchedule: schedule,
        photos: Object.freeze(photos),
        loading: false,
        error: null,
      });
      return detail;
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : `Error al cargar publicación ${listingId}`;
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async createDraft(
    token: AccessToken,
    input: ListingDraftInput,
    idempotencyKey?: string,
  ): Promise<ListingDetail> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#port.createDraft(token, input, idempotencyKey);
      const updatedListings = [
        {
          id: detail.id,
          category_code: detail.category_code,
          category_name: detail.category_name,
          title: detail.title,
          state: detail.state,
          base_price_clp: detail.base_price_clp,
          rate_unit: detail.rate_unit,
          area_m2: detail.area_m2,
          capacity: detail.capacity,
          cover_photo_id: detail.cover_photo_id,
          time_zone: detail.time_zone,
          region_code: detail.region_code,
          comuna_code: detail.comuna_code,
        },
        ...this.#snapshot.listings,
      ];
      this.#setSnapshot({
        listings: Object.freeze(updatedListings),
        selectedListing: detail,
        loading: false,
        error: null,
      });
      return detail;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al crear borrador";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async updateDraft(
    token: AccessToken,
    listingId: string,
    input: ListingDraftInput,
    ifMatch?: string,
  ): Promise<ListingDetail> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#port.updateDraft(
        token,
        listingId,
        input,
        ifMatch,
      );
      const updatedListings = this.#snapshot.listings.map((item) =>
        item.id === listingId
          ? {
              ...item,
              title: detail.title,
              base_price_clp: detail.base_price_clp,
              rate_unit: detail.rate_unit,
              area_m2: detail.area_m2,
              capacity: detail.capacity,
              state: detail.state,
              time_zone: detail.time_zone,
              region_code: detail.region_code,
              comuna_code: detail.comuna_code,
            }
          : item,
      );
      this.#setSnapshot({
        listings: Object.freeze(updatedListings),
        selectedListing: detail,
        loading: false,
        error: null,
      });
      return detail;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al actualizar borrador";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async deleteListing(
    token: AccessToken,
    listingId: string,
  ): Promise<ListingDeletionReceipt> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const receipt = await this.#port.deleteListing(token, listingId);
      const remaining = this.#snapshot.listings.filter(
        (item) => item.id !== listingId,
      );
      this.#setSnapshot({
        listings: Object.freeze(remaining),
        selectedListing:
          this.#snapshot.selectedListing?.id === listingId
            ? null
            : this.#snapshot.selectedListing,
        loading: false,
        error: null,
      });
      return receipt;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al retirar publicación";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async updateContent(
    token: AccessToken,
    listingId: string,
    input: ContentUpdateInput,
    ifMatch?: string,
  ): Promise<ListingDetail> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#port.updateContent(
        token,
        listingId,
        input,
        ifMatch,
      );
      const updatedListings = this.#snapshot.listings.map((item) =>
        item.id === listingId
          ? {
              ...item,
              title: detail.title,
              base_price_clp: detail.base_price_clp,
              capacity: detail.capacity,
              region_code: detail.region_code,
              comuna_code: detail.comuna_code,
            }
          : item,
      );
      this.#setSnapshot({
        listings: Object.freeze(updatedListings),
        selectedListing: detail,
        loading: false,
        error: null,
      });
      return detail;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al actualizar contenido";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async publishListing(
    token: AccessToken,
    listingId: string,
    ifMatch?: string,
  ): Promise<ListingDetail> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#port.publishListing(token, listingId, ifMatch);
      const updatedListings = this.#snapshot.listings.map((item) =>
        item.id === listingId ? { ...item, state: "activa" as const } : item,
      );
      this.#setSnapshot({
        listings: Object.freeze(updatedListings),
        selectedListing: detail,
        loading: false,
        error: null,
      });
      return detail;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al publicar espacio";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async archiveListing(
    token: AccessToken,
    listingId: string,
    ifMatch?: string,
  ): Promise<ListingDetail> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#port.archiveListing(token, listingId, ifMatch);
      const updatedListings = this.#snapshot.listings.map((item) =>
        item.id === listingId ? { ...item, state: "oculta" as const } : item,
      );
      this.#setSnapshot({
        listings: Object.freeze(updatedListings),
        selectedListing: detail,
        loading: false,
        error: null,
      });
      return detail;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al ocultar espacio";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async loadPricing(
    token: AccessToken,
    listingId: string,
  ): Promise<PricingHistoryItem | null> {
    try {
      const tariff = await this.#port.getPricing(token, listingId);
      this.#setSnapshot({ activeTariff: tariff });
      return tariff;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al cargar tarifa";
      this.#setSnapshot({ error: message });
      throw err;
    }
  }

  async updatePricing(
    token: AccessToken,
    listingId: string,
    input: PricingInput,
    ifMatch?: string,
  ): Promise<PricingHistoryItem> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const updated = await this.#port.updatePricing(
        token,
        listingId,
        input,
        ifMatch,
      );
      this.#setSnapshot({
        activeTariff: updated,
        tariffHistory: Object.freeze([
          updated,
          ...this.#snapshot.tariffHistory,
        ]),
        loading: false,
        error: null,
      });
      return updated;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al actualizar tarifa";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async loadPricingHistory(
    token: AccessToken,
    listingId: string,
  ): Promise<readonly PricingHistoryItem[]> {
    try {
      const history = await this.#port.listPricingHistory(token, listingId);
      this.#setSnapshot({ tariffHistory: Object.freeze(history) });
      return history;
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Error al cargar historial de tarifas";
      this.#setSnapshot({ error: message });
      throw err;
    }
  }

  async configureTimeZone(
    token: AccessToken,
    listingId: string,
    timeZone: string,
    ifMatch?: string,
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.configureTimeZone(token, listingId, timeZone, ifMatch);
      if (this.#snapshot.selectedListing?.id === listingId) {
        this.#setSnapshot({
          selectedListing: {
            ...this.#snapshot.selectedListing,
            time_zone: timeZone,
          },
        });
      }
      this.#setSnapshot({ loading: false, error: null });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al configurar zona horaria";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async loadWeeklySchedule(
    token: AccessToken,
    listingId: string,
  ): Promise<ScheduleData | null> {
    try {
      const schedule = await this.#port.getWeeklySchedule(token, listingId);
      this.#setSnapshot({ weeklySchedule: schedule });
      return schedule;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al cargar horario semanal";
      this.#setSnapshot({ error: message });
      throw err;
    }
  }

  async updateWeeklySchedule(
    token: AccessToken,
    listingId: string,
    input: ScheduleInput,
    ifMatch?: string,
  ): Promise<ScheduleData> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const schedule = await this.#port.updateWeeklySchedule(
        token,
        listingId,
        input,
        ifMatch,
      );
      this.#setSnapshot({
        weeklySchedule: schedule,
        loading: false,
        error: null,
      });
      return schedule;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al actualizar horario";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async loadPhotos(
    token: AccessToken,
    listingId: string,
  ): Promise<readonly GalleryPhoto[]> {
    try {
      const photos = await this.#port.listPhotos(token, listingId);
      this.#setSnapshot({ photos: Object.freeze(photos) });
      return photos;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al cargar fotos";
      this.#setSnapshot({ error: message });
      throw err;
    }
  }

  async uploadPhoto(
    token: AccessToken,
    listingId: string,
    input?: PhotoUploadInput,
    idempotencyKey?: string,
  ): Promise<GalleryPhoto> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      const photo = await this.#port.uploadPhoto(
        token,
        listingId,
        input,
        idempotencyKey,
      );
      const updatedPhotos = [...this.#snapshot.photos, photo];
      this.#setSnapshot({
        photos: Object.freeze(updatedPhotos),
        loading: false,
        error: null,
      });
      return photo;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al subir foto";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async setCoverPhoto(
    token: AccessToken,
    listingId: string,
    photoId: string,
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.setCoverPhoto(token, listingId, photoId);
      const updated = this.#snapshot.photos.map((p) => ({
        ...p,
        is_cover: p.id === photoId,
      }));
      this.#setSnapshot({
        photos: Object.freeze(updated),
        selectedListing:
          this.#snapshot.selectedListing?.id === listingId
            ? { ...this.#snapshot.selectedListing, cover_photo_id: photoId }
            : this.#snapshot.selectedListing,
        loading: false,
        error: null,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al definir foto de portada";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async removePhoto(
    token: AccessToken,
    listingId: string,
    photoId: string,
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.removePhoto(token, listingId, photoId);
      const updated = this.#snapshot.photos.filter((p) => p.id !== photoId);
      this.#setSnapshot({
        photos: Object.freeze(updated),
        selectedListing:
          this.#snapshot.selectedListing?.cover_photo_id === photoId
            ? { ...this.#snapshot.selectedListing, cover_photo_id: null }
            : this.#snapshot.selectedListing,
        loading: false,
        error: null,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al eliminar foto";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  async reorderPhotos(
    token: AccessToken,
    listingId: string,
    photoIds: string[],
  ): Promise<void> {
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.reorderPhotos(token, listingId, photoIds);
      const idMap = new Map<string, number>();
      photoIds.forEach((id, idx) => idMap.set(id, idx + 1));
      const updated = [...this.#snapshot.photos]
        .map((p) => ({
          ...p,
          position: idMap.get(p.id) ?? p.position,
        }))
        .sort((a, b) => a.position - b.position);
      this.#setSnapshot({
        photos: Object.freeze(updated),
        loading: false,
        error: null,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al reordenar fotos";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    }
  }

  clear(): void {
    this.cancel();
    this.#setSnapshot(initialSnapshot);
  }
}
