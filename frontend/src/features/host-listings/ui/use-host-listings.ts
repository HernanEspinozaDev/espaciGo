"use client";

import { useCallback, useSyncExternalStore } from "react";
import { hostListingsFacade } from "../host-listings-runtime";
import type { HostListingsSnapshot } from "../application/host-listings-facade";
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

export interface HostListingsView {
  snapshot: HostListingsSnapshot;
  loadCategories(signal?: AbortSignal): Promise<readonly Category[]>;
  loadCategorySchema(
    categoryCode: string,
    version?: number,
    signal?: AbortSignal,
  ): Promise<CategoryProfileSchema | null>;
  loadListings(
    token: AccessToken,
    filter?: { status?: ListingState },
  ): Promise<readonly ListingSummary[]>;
  selectListing(
    token: AccessToken,
    listingId: string,
  ): Promise<ListingDetail | null>;
  createDraft(
    token: AccessToken,
    input: ListingDraftInput,
    idempotencyKey?: string,
  ): Promise<ListingDetail>;
  updateDraft(
    token: AccessToken,
    listingId: string,
    input: ListingDraftInput,
    ifMatch?: string,
  ): Promise<ListingDetail>;
  deleteListing(
    token: AccessToken,
    listingId: string,
  ): Promise<ListingDeletionReceipt>;
  updateContent(
    token: AccessToken,
    listingId: string,
    input: ContentUpdateInput,
    ifMatch?: string,
  ): Promise<ListingDetail>;
  publishListing(
    token: AccessToken,
    listingId: string,
    ifMatch?: string,
  ): Promise<ListingDetail>;
  archiveListing(
    token: AccessToken,
    listingId: string,
    ifMatch?: string,
  ): Promise<ListingDetail>;
  loadPricing(
    token: AccessToken,
    listingId: string,
  ): Promise<PricingHistoryItem | null>;
  updatePricing(
    token: AccessToken,
    listingId: string,
    input: PricingInput,
    ifMatch?: string,
  ): Promise<PricingHistoryItem>;
  loadPricingHistory(
    token: AccessToken,
    listingId: string,
  ): Promise<readonly PricingHistoryItem[]>;
  configureTimeZone(
    token: AccessToken,
    listingId: string,
    timeZone: string,
    ifMatch?: string,
  ): Promise<void>;
  loadWeeklySchedule(
    token: AccessToken,
    listingId: string,
  ): Promise<ScheduleData | null>;
  updateWeeklySchedule(
    token: AccessToken,
    listingId: string,
    input: ScheduleInput,
    ifMatch?: string,
  ): Promise<ScheduleData>;
  loadPhotos(
    token: AccessToken,
    listingId: string,
  ): Promise<readonly GalleryPhoto[]>;
  uploadPhoto(
    token: AccessToken,
    listingId: string,
    input?: PhotoUploadInput,
    idempotencyKey?: string,
  ): Promise<GalleryPhoto>;
  setCoverPhoto(
    token: AccessToken,
    listingId: string,
    photoId: string,
  ): Promise<void>;
  removePhoto(
    token: AccessToken,
    listingId: string,
    photoId: string,
  ): Promise<void>;
  reorderPhotos(
    token: AccessToken,
    listingId: string,
    photoIds: string[],
  ): Promise<void>;
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
    (token: AccessToken, filter?: { status?: ListingState }) =>
      hostListingsFacade.loadListings(token, filter),
    [],
  );

  const selectListing = useCallback(
    (token: AccessToken, listingId: string) =>
      hostListingsFacade.selectListing(token, listingId),
    [],
  );

  const createDraft = useCallback(
    (token: AccessToken, input: ListingDraftInput, idempotencyKey?: string) =>
      hostListingsFacade.createDraft(token, input, idempotencyKey),
    [],
  );

  const updateDraft = useCallback(
    (
      token: AccessToken,
      listingId: string,
      input: ListingDraftInput,
      ifMatch?: string,
    ) => hostListingsFacade.updateDraft(token, listingId, input, ifMatch),
    [],
  );

  const deleteListing = useCallback(
    (token: AccessToken, listingId: string) =>
      hostListingsFacade.deleteListing(token, listingId),
    [],
  );

  const updateContent = useCallback(
    (
      token: AccessToken,
      listingId: string,
      input: ContentUpdateInput,
      ifMatch?: string,
    ) => hostListingsFacade.updateContent(token, listingId, input, ifMatch),
    [],
  );

  const publishListing = useCallback(
    (token: AccessToken, listingId: string, ifMatch?: string) =>
      hostListingsFacade.publishListing(token, listingId, ifMatch),
    [],
  );

  const archiveListing = useCallback(
    (token: AccessToken, listingId: string, ifMatch?: string) =>
      hostListingsFacade.archiveListing(token, listingId, ifMatch),
    [],
  );

  const loadPricing = useCallback(
    (token: AccessToken, listingId: string) =>
      hostListingsFacade.loadPricing(token, listingId),
    [],
  );

  const updatePricing = useCallback(
    (
      token: AccessToken,
      listingId: string,
      input: PricingInput,
      ifMatch?: string,
    ) => hostListingsFacade.updatePricing(token, listingId, input, ifMatch),
    [],
  );

  const loadPricingHistory = useCallback(
    (token: AccessToken, listingId: string) =>
      hostListingsFacade.loadPricingHistory(token, listingId),
    [],
  );

  const configureTimeZone = useCallback(
    (
      token: AccessToken,
      listingId: string,
      timeZone: string,
      ifMatch?: string,
    ) =>
      hostListingsFacade.configureTimeZone(token, listingId, timeZone, ifMatch),
    [],
  );

  const loadWeeklySchedule = useCallback(
    (token: AccessToken, listingId: string) =>
      hostListingsFacade.loadWeeklySchedule(token, listingId),
    [],
  );

  const updateWeeklySchedule = useCallback(
    (
      token: AccessToken,
      listingId: string,
      input: ScheduleInput,
      ifMatch?: string,
    ) =>
      hostListingsFacade.updateWeeklySchedule(token, listingId, input, ifMatch),
    [],
  );

  const loadPhotos = useCallback(
    (token: AccessToken, listingId: string) =>
      hostListingsFacade.loadPhotos(token, listingId),
    [],
  );

  const uploadPhoto = useCallback(
    (
      token: AccessToken,
      listingId: string,
      input?: PhotoUploadInput,
      idempotencyKey?: string,
    ) =>
      hostListingsFacade.uploadPhoto(token, listingId, input, idempotencyKey),
    [],
  );

  const setCoverPhoto = useCallback(
    (token: AccessToken, listingId: string, photoId: string) =>
      hostListingsFacade.setCoverPhoto(token, listingId, photoId),
    [],
  );

  const removePhoto = useCallback(
    (token: AccessToken, listingId: string, photoId: string) =>
      hostListingsFacade.removePhoto(token, listingId, photoId),
    [],
  );

  const reorderPhotos = useCallback(
    (token: AccessToken, listingId: string, photoIds: string[]) =>
      hostListingsFacade.reorderPhotos(token, listingId, photoIds),
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
