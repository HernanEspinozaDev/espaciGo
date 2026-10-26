"use client";

import "client-only";

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
import type { AuthSnapshot } from "@/features/auth/application/auth-facade";
import type { HostListingsPort } from "./host-listings.port";

export type HostListingsAuthSource = {
  getSnapshot: () => AuthSnapshot;
  subscribe: (listener: () => void) => () => void;
  checkSession: () => Promise<unknown>;
};

export type HostListingsPrivatePurgeListener =
  | (() => void)
  | {
      onPrivatePurge?: () => void;
    };

function isAuthError(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const candidate = error as {
    status?: unknown;
    statusCode?: unknown;
    code?: unknown;
  };
  return (
    candidate.status === 401 ||
    candidate.status === 403 ||
    candidate.statusCode === 401 ||
    candidate.statusCode === 403 ||
    candidate.code === "unauthorized" ||
    candidate.code === "forbidden" ||
    candidate.code === "UNAUTHORIZED" ||
    candidate.code === "FORBIDDEN"
  );
}

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
  #authSource?: HostListingsAuthSource;
  #onPrivatePurge?: () => void;
  #authUnsubscribe: (() => void) | null = null;
  #lastAuthSnapshot: AuthSnapshot | null = null;
  #abortController: AbortController | null = null;
  #selectAbortController: AbortController | null = null;
  #draftAbortController: AbortController | null = null;
  #pricingAbortController: AbortController | null = null;
  #sessionControllers = new Set<AbortController>();
  #snapshot: HostListingsSnapshot = initialSnapshot;
  #listeners = new Set<() => void>();
  #epoch = 0;

  constructor(
    port: HostListingsPort,
    authSource?: HostListingsAuthSource,
    onPrivatePurge?: HostListingsPrivatePurgeListener,
  ) {
    this.#port = port;
    this.#authSource = authSource;
    this.#onPrivatePurge =
      typeof onPrivatePurge === "function"
        ? onPrivatePurge
        : onPrivatePurge?.onPrivatePurge?.bind(onPrivatePurge);

    if (this.#authSource) {
      this.#lastAuthSnapshot = this.#authSource.getSnapshot();
      this.#authUnsubscribe = this.#authSource.subscribe(() => {
        this.#syncWithAuthSource();
      });
    }
  }

  #sameSession(prev: AuthSnapshot | null, current: AuthSnapshot): boolean {
    if (!prev) return false;
    if (prev.status !== "authenticated" || current.status !== "authenticated") {
      return false;
    }
    if (prev.accountId !== current.accountId) {
      return false;
    }
    if (prev.restrictedMode !== current.restrictedMode) {
      return false;
    }
    if (prev.expiresAt !== current.expiresAt) {
      return false;
    }
    const prevRoles = prev.roles ?? [];
    const currRoles = current.roles ?? [];
    if (
      prevRoles.length !== currRoles.length ||
      prevRoles.some((r: string, i: number) => r !== currRoles[i])
    ) {
      return false;
    }
    const prevCaps = prev.capabilities ?? [];
    const currCaps = current.capabilities ?? [];
    if (
      prevCaps.length !== currCaps.length ||
      prevCaps.some((c: string, i: number) => c !== currCaps[i])
    ) {
      return false;
    }
    return true;
  }

  #syncWithAuthSource(): void {
    if (!this.#authSource) return;
    const current = this.#authSource.getSnapshot();
    const prev = this.#lastAuthSnapshot;

    if (prev?.status === "authenticated") {
      if (
        current.status !== "authenticated" ||
        !this.#sameSession(prev, current)
      ) {
        this.clear();
      }
    } else if (current.status === "authenticated") {
      this.clear();
    }
    this.#lastAuthSnapshot = current;
  }

  #handleAuthError(): void {
    this.clear();
    try {
      void this.#authSource?.checkSession().catch(() => undefined);
    } catch {
      // best-effort session check
    }
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

  #beginSessionOperation(callerSignal?: AbortSignal): {
    epoch: number;
    controller: AbortController;
    signal: AbortSignal;
  } {
    const epoch = this.#epoch;
    const controller = new AbortController();
    this.#sessionControllers.add(controller);
    const signal = callerSignal
      ? AbortSignal.any([callerSignal, controller.signal])
      : controller.signal;
    return { epoch, controller, signal };
  }

  #isSessionValid(scope: { epoch: number; signal: AbortSignal }): boolean {
    return this.#epoch === scope.epoch && !scope.signal.aborted;
  }

  #endSessionOperation(scope: { controller: AbortController }): void {
    this.#sessionControllers.delete(scope.controller);
  }

  cancel(): void {
    if (this.#abortController) {
      this.#abortController.abort();
      this.#abortController = null;
    }
    if (this.#selectAbortController) {
      this.#selectAbortController.abort();
      this.#selectAbortController = null;
    }
    if (this.#draftAbortController) {
      this.#draftAbortController.abort();
      this.#draftAbortController = null;
    }
    if (this.#pricingAbortController) {
      this.#pricingAbortController.abort();
      this.#pricingAbortController = null;
    }
    this.#sessionControllers.forEach((c) => c.abort());
    this.#sessionControllers.clear();
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
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return [];
    }

    const scope = this.#beginSessionOperation(signal);
    try {
      const categories = await this.#port.listCategories(scope.signal);
      if (!this.#isSessionValid(scope)) {
        return [];
      }
      this.#setSnapshot({
        categories: Object.freeze(categories),
      });
      return categories;
    } catch (err) {
      if (!this.#isSessionValid(scope)) {
        if (err instanceof Error && err.name === "AbortError") {
          throw err;
        }
        return [];
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al cargar categorías";
      this.#setSnapshot({ error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async loadCategorySchema(
    categoryCode: string,
    version?: number,
    signal?: AbortSignal,
  ): Promise<CategoryProfileSchema | null> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return null;
    }

    const cacheKey =
      version !== undefined ? `${categoryCode}@${version}` : categoryCode;
    if (this.#snapshot.categorySchemas[cacheKey]) {
      return this.#snapshot.categorySchemas[cacheKey];
    }

    const scope = this.#beginSessionOperation(signal);
    try {
      const schema = await this.#port.getCategoryProfile(
        categoryCode,
        version,
        scope.signal,
      );
      if (!this.#isSessionValid(scope)) {
        return null;
      }
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
      if (!this.#isSessionValid(scope)) {
        if (err instanceof Error && err.name === "AbortError") {
          throw err;
        }
        return null;
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error
          ? err.message
          : `Error al cargar perfil para ${categoryCode}`;
      this.#setSnapshot({ error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async loadListings(filter?: {
    status?: ListingState;
  }): Promise<readonly ListingSummary[]> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return [];
    }

    const epoch = this.#epoch;
    if (this.#abortController) {
      this.#abortController.abort();
      this.#abortController = null;
    }
    const controller = new AbortController();
    this.#abortController = controller;
    const signal = controller.signal;

    this.#setSnapshot({ loading: true, error: null });
    try {
      const listings = await this.#port.listListings(filter, signal);
      if (this.#epoch !== epoch || signal.aborted) {
        return [];
      }
      this.#setSnapshot({
        listings: Object.freeze(listings),
        loading: false,
        error: null,
      });
      return listings;
    } catch (err) {
      if (this.#epoch !== epoch || signal.aborted) {
        if (err instanceof Error && err.name === "AbortError") {
          throw err;
        }
        return [];
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al cargar publicaciones";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      if (this.#abortController === controller) {
        this.#abortController = null;
      }
    }
  }

  async selectListing(listingId: string): Promise<ListingDetail | null> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return null;
    }

    const epoch = this.#epoch;
    if (this.#selectAbortController) {
      this.#selectAbortController.abort();
      this.#selectAbortController = null;
    }
    const controller = new AbortController();
    this.#selectAbortController = controller;
    const signal = controller.signal;

    this.#setSnapshot({ loading: true, error: null });
    try {
      const [detail, tariff, schedule, photos] = await Promise.all([
        this.#port.getListing(listingId, signal),
        this.#port.getPricing(listingId, signal).catch((err) => {
          if (isAuthError(err)) throw err;
          return null;
        }),
        this.#port.getWeeklySchedule(listingId, signal).catch((err) => {
          if (isAuthError(err)) throw err;
          return null;
        }),
        this.#port.listPhotos(listingId, signal).catch((err) => {
          if (isAuthError(err)) throw err;
          return [];
        }),
      ]);

      if (this.#epoch !== epoch || signal.aborted) {
        return null;
      }

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
      if (this.#epoch !== epoch || signal.aborted) {
        if (err instanceof Error && err.name === "AbortError") {
          throw err;
        }
        return null;
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error
          ? err.message
          : `Error al cargar publicación ${listingId}`;
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      if (this.#selectAbortController === controller) {
        this.#selectAbortController = null;
      }
    }
  }

  async createDraft(
    input: ListingDraftInput,
    idempotencyKey?: string,
  ): Promise<ListingDetail> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const epoch = this.#epoch;
    if (this.#draftAbortController) {
      this.#draftAbortController.abort();
      this.#draftAbortController = null;
    }
    const controller = new AbortController();
    this.#draftAbortController = controller;
    const signal = controller.signal;

    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#port.createDraft(
        input,
        idempotencyKey,
        signal,
      );
      if (this.#epoch !== epoch || signal.aborted) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
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
      if (this.#epoch !== epoch || signal.aborted) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al crear borrador";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      if (this.#draftAbortController === controller) {
        this.#draftAbortController = null;
      }
    }
  }

  async updateDraft(
    listingId: string,
    input: ListingDraftInput,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const scope = this.#beginSessionOperation(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#port.updateDraft(
        listingId,
        input,
        ifMatch,
        scope.signal,
      );
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
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
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al actualizar borrador";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async deleteListing(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<ListingDeletionReceipt> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const scope = this.#beginSessionOperation(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      const receipt = await this.#port.deleteListing(listingId, scope.signal);
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
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
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al retirar publicación";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async updateContent(
    listingId: string,
    input: ContentUpdateInput,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const scope = this.#beginSessionOperation(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#port.updateContent(
        listingId,
        input,
        ifMatch,
        scope.signal,
      );
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
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
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al actualizar contenido";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async publishListing(
    listingId: string,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const scope = this.#beginSessionOperation(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#port.publishListing(
        listingId,
        ifMatch,
        scope.signal,
      );
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
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
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al publicar espacio";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async archiveListing(
    listingId: string,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const scope = this.#beginSessionOperation(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      const detail = await this.#port.archiveListing(
        listingId,
        ifMatch,
        scope.signal,
      );
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
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
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al ocultar espacio";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async loadPricing(listingId: string): Promise<PricingHistoryItem | null> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return null;
    }

    const epoch = this.#epoch;
    if (this.#pricingAbortController) {
      this.#pricingAbortController.abort();
      this.#pricingAbortController = null;
    }
    const controller = new AbortController();
    this.#pricingAbortController = controller;
    const signal = controller.signal;

    try {
      const tariff = await this.#port.getPricing(listingId, signal);
      if (this.#epoch !== epoch || signal.aborted) {
        return null;
      }
      this.#setSnapshot({ activeTariff: tariff });
      return tariff;
    } catch (err) {
      if (this.#epoch !== epoch || signal.aborted) {
        if (err instanceof Error && err.name === "AbortError") {
          throw err;
        }
        return null;
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al cargar tarifa";
      this.#setSnapshot({ error: message });
      throw err;
    } finally {
      if (this.#pricingAbortController === controller) {
        this.#pricingAbortController = null;
      }
    }
  }

  async updatePricing(
    listingId: string,
    input: PricingInput,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<PricingHistoryItem> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const scope = this.#beginSessionOperation(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      const updated = await this.#port.updatePricing(
        listingId,
        input,
        ifMatch,
        scope.signal,
      );
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
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
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al actualizar tarifa";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async loadPricingHistory(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<readonly PricingHistoryItem[]> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return [];
    }

    const scope = this.#beginSessionOperation(signal);
    try {
      const history = await this.#port.listPricingHistory(
        listingId,
        scope.signal,
      );
      if (!this.#isSessionValid(scope)) {
        return [];
      }
      this.#setSnapshot({ tariffHistory: Object.freeze(history) });
      return history;
    } catch (err) {
      if (!this.#isSessionValid(scope)) {
        if (err instanceof Error && err.name === "AbortError") {
          throw err;
        }
        return [];
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error
          ? err.message
          : "Error al cargar historial de tarifas";
      this.#setSnapshot({ error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async configureTimeZone(
    listingId: string,
    timeZone: string,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const scope = this.#beginSessionOperation(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.configureTimeZone(
        listingId,
        timeZone,
        ifMatch,
        scope.signal,
      );
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
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
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al configurar zona horaria";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async loadWeeklySchedule(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<ScheduleData | null> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return null;
    }

    const scope = this.#beginSessionOperation(signal);
    try {
      const schedule = await this.#port.getWeeklySchedule(
        listingId,
        scope.signal,
      );
      if (!this.#isSessionValid(scope)) {
        return null;
      }
      this.#setSnapshot({ weeklySchedule: schedule });
      return schedule;
    } catch (err) {
      if (!this.#isSessionValid(scope)) {
        if (err instanceof Error && err.name === "AbortError") {
          throw err;
        }
        return null;
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al cargar horario semanal";
      this.#setSnapshot({ error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async updateWeeklySchedule(
    listingId: string,
    input: ScheduleInput,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ScheduleData> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const scope = this.#beginSessionOperation(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      const schedule = await this.#port.updateWeeklySchedule(
        listingId,
        input,
        ifMatch,
        scope.signal,
      );
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      this.#setSnapshot({
        weeklySchedule: schedule,
        loading: false,
        error: null,
      });
      return schedule;
    } catch (err) {
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al actualizar horario";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async loadPhotos(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<readonly GalleryPhoto[]> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      return [];
    }

    const scope = this.#beginSessionOperation(signal);
    try {
      const photos = await this.#port.listPhotos(listingId, scope.signal);
      if (!this.#isSessionValid(scope)) {
        return [];
      }
      this.#setSnapshot({ photos: Object.freeze(photos) });
      return photos;
    } catch (err) {
      if (!this.#isSessionValid(scope)) {
        if (err instanceof Error && err.name === "AbortError") {
          throw err;
        }
        return [];
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al cargar fotos";
      this.#setSnapshot({ error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async uploadPhoto(
    listingId: string,
    input?: PhotoUploadInput,
    idempotencyKey?: string,
    signal?: AbortSignal,
  ): Promise<GalleryPhoto> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const scope = this.#beginSessionOperation(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      const photo = await this.#port.uploadPhoto(
        listingId,
        input,
        idempotencyKey,
        scope.signal,
      );
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      const updatedPhotos = [...this.#snapshot.photos, photo];
      this.#setSnapshot({
        photos: Object.freeze(updatedPhotos),
        loading: false,
        error: null,
      });
      return photo;
    } catch (err) {
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al subir foto";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async setCoverPhoto(
    listingId: string,
    photoId: string,
    signal?: AbortSignal,
  ): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const scope = this.#beginSessionOperation(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.setCoverPhoto(listingId, photoId, scope.signal);
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
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
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al definir foto de portada";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async removePhoto(
    listingId: string,
    photoId: string,
    signal?: AbortSignal,
  ): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const scope = this.#beginSessionOperation(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.removePhoto(listingId, photoId, scope.signal);
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
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
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al eliminar foto";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  async reorderPhotos(
    listingId: string,
    photoIds: string[],
    signal?: AbortSignal,
  ): Promise<void> {
    if (
      this.#authSource &&
      this.#authSource.getSnapshot().status !== "authenticated"
    ) {
      throw new Error("unauthenticated");
    }

    const scope = this.#beginSessionOperation(signal);
    this.#setSnapshot({ loading: true, error: null });
    try {
      await this.#port.reorderPhotos(listingId, photoIds, scope.signal);
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
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
      if (!this.#isSessionValid(scope)) {
        throw new DOMException("Operación cancelada", "AbortError");
      }
      if (isAuthError(err)) {
        this.#handleAuthError();
      }
      const message =
        err instanceof Error ? err.message : "Error al reordenar fotos";
      this.#setSnapshot({ loading: false, error: message });
      throw err;
    } finally {
      this.#endSessionOperation(scope);
    }
  }

  clear(): void {
    this.#epoch += 1;
    this.cancel();
    this.#setSnapshot(initialSnapshot);
    try {
      this.#onPrivatePurge?.();
    } catch {
      // non-blocking purge listener
    }
  }
}
