"use client";

import { useCallback, useSyncExternalStore } from "react";
import { hostListingsFacade } from "../host-listings-runtime";
import type { HostListingsSnapshot } from "../application/host-listings-facade";
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

export interface HostListingsView {
  snapshot: HostListingsSnapshot;
  loadCategories(signal?: AbortSignal): Promise<readonly Category[]>;
  loadCategorySchema(
    categoryCode: string,
    version?: number,
    signal?: AbortSignal,
  ): Promise<CategoryProfileSchema | null>;
  loadListings(filter?: {
    status?: ListingState;
  }): Promise<readonly ListingSummary[]>;
  selectListing(listingId: string): Promise<ListingDetail | null>;
  createDraft(
    input: ListingDraftInput,
    idempotencyKey?: string,
  ): Promise<ListingDetail>;
  updateDraft(
    listingId: string,
    input: ListingDraftInput,
    ifMatch?: string,
  ): Promise<ListingDetail>;
  deleteListing(listingId: string): Promise<ListingDeletionReceipt>;
  updateContent(
    listingId: string,
    input: ContentUpdateInput,
    ifMatch?: string,
  ): Promise<ListingDetail>;
  publishListing(listingId: string, ifMatch?: string): Promise<ListingDetail>;
  archiveListing(listingId: string, ifMatch?: string): Promise<ListingDetail>;
  loadPricing(listingId: string): Promise<PricingHistoryItem | null>;
  updatePricing(
    listingId: string,
    input: PricingInput,
    ifMatch?: string,
  ): Promise<PricingHistoryItem>;
  loadPricingHistory(listingId: string): Promise<readonly PricingHistoryItem[]>;
  configureTimeZone(
    listingId: string,
    timeZone: string,
    ifMatch?: string,
  ): Promise<void>;
  loadWeeklySchedule(listingId: string): Promise<ScheduleData | null>;
  updateWeeklySchedule(
    listingId: string,
    input: ScheduleInput,
    ifMatch?: string,
  ): Promise<ScheduleData>;
  loadPhotos(listingId: string): Promise<readonly GalleryPhoto[]>;
  uploadPhoto(
    listingId: string,
    input?: PhotoUploadInput,
    idempotencyKey?: string,
  ): Promise<GalleryPhoto>;
  setCoverPhoto(listingId: string, photoId: string): Promise<void>;
  removePhoto(listingId: string, photoId: string): Promise<void>;
  reorderPhotos(listingId: string, photoIds: string[]): Promise<void>;
  setFilter(filter: Partial<HostListingsSnapshot["filter"]>): void;
  clear(): void;
}

export function useHostListings(): HostListingsView {
  const snapshot = useSyncExternalStore(
    hostListingsFacade.subscribe,
    hostListingsFacade.getSnapshot,
    hostListingsFacade.getServerSnapshot,
  );

  const loadCategories = useCallback(
    (signal?: AbortSignal) => hostListingsFacade.loadCategories(signal),
    [],
  );

  const loadCategorySchema = useCallback(
    (categoryCode: string, version?: number, signal?: AbortSignal) =>
      hostListingsFacade.loadCategorySchema(categoryCode, version, signal),
    [],
  );

  const loadListings = useCallback(
    (filter?: { status?: ListingState }) =>
      hostListingsFacade.loadListings(filter),
    [],
  );

  const selectListing = useCallback(
    (listingId: string) => hostListingsFacade.selectListing(listingId),
    [],
  );

  const createDraft = useCallback(
    (input: ListingDraftInput, idempotencyKey?: string) =>
      hostListingsFacade.createDraft(input, idempotencyKey),
    [],
  );

  const updateDraft = useCallback(
    (listingId: string, input: ListingDraftInput, ifMatch?: string) =>
      hostListingsFacade.updateDraft(listingId, input, ifMatch),
    [],
  );

  const deleteListing = useCallback(
    (listingId: string) => hostListingsFacade.deleteListing(listingId),
    [],
  );

  const updateContent = useCallback(
    (listingId: string, input: ContentUpdateInput, ifMatch?: string) =>
      hostListingsFacade.updateContent(listingId, input, ifMatch),
    [],
  );

  const publishListing = useCallback(
    (listingId: string, ifMatch?: string) =>
      hostListingsFacade.publishListing(listingId, ifMatch),
    [],
  );

  const archiveListing = useCallback(
    (listingId: string, ifMatch?: string) =>
      hostListingsFacade.archiveListing(listingId, ifMatch),
    [],
  );

  const loadPricing = useCallback(
    (listingId: string) => hostListingsFacade.loadPricing(listingId),
    [],
  );

  const updatePricing = useCallback(
    (listingId: string, input: PricingInput, ifMatch?: string) =>
      hostListingsFacade.updatePricing(listingId, input, ifMatch),
    [],
  );

  const loadPricingHistory = useCallback(
    (listingId: string) => hostListingsFacade.loadPricingHistory(listingId),
    [],
  );

  const configureTimeZone = useCallback(
    (listingId: string, timeZone: string, ifMatch?: string) =>
      hostListingsFacade.configureTimeZone(listingId, timeZone, ifMatch),
    [],
  );

  const loadWeeklySchedule = useCallback(
    (listingId: string) => hostListingsFacade.loadWeeklySchedule(listingId),
    [],
  );

  const updateWeeklySchedule = useCallback(
    (listingId: string, input: ScheduleInput, ifMatch?: string) =>
      hostListingsFacade.updateWeeklySchedule(listingId, input, ifMatch),
    [],
  );

  const loadPhotos = useCallback(
    (listingId: string) => hostListingsFacade.loadPhotos(listingId),
    [],
  );

  const uploadPhoto = useCallback(
    (listingId: string, input?: PhotoUploadInput, idempotencyKey?: string) =>
      hostListingsFacade.uploadPhoto(listingId, input, idempotencyKey),
    [],
  );

  const setCoverPhoto = useCallback(
    (listingId: string, photoId: string) =>
      hostListingsFacade.setCoverPhoto(listingId, photoId),
    [],
  );

  const removePhoto = useCallback(
    (listingId: string, photoId: string) =>
      hostListingsFacade.removePhoto(listingId, photoId),
    [],
  );

  const reorderPhotos = useCallback(
    (listingId: string, photoIds: string[]) =>
      hostListingsFacade.reorderPhotos(listingId, photoIds),
    [],
  );

  const setFilter = useCallback(
    (filter: Partial<HostListingsSnapshot["filter"]>) =>
      hostListingsFacade.setFilter(filter),
    [],
  );

  const clear = useCallback(() => hostListingsFacade.clear(), []);

  return {
    snapshot,
    loadCategories,
    loadCategorySchema,
    loadListings,
    selectListing,
    createDraft,
    updateDraft,
    deleteListing,
    updateContent,
    publishListing,
    archiveListing,
    loadPricing,
    updatePricing,
    loadPricingHistory,
    configureTimeZone,
    loadWeeklySchedule,
    updateWeeklySchedule,
    loadPhotos,
    uploadPhoto,
    setCoverPhoto,
    removePhoto,
    reorderPhotos,
    setFilter,
    clear,
  };
}
