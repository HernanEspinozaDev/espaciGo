"use client";

import { useCallback, useSyncExternalStore } from "react";
import {
  hostListingsFacade,
  hostListingsFeatureBridge,
} from "../host-listings-runtime";
import type { HostListingsSnapshot } from "../application/host-listings-facade";
import type { AccessToken } from "@/features/auth/application/auth.port";
import type {
  Category,
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
  PhotoUploadInput,
  PricingHistoryItem,
  PricingInput,
  ScheduleData,
  ScheduleInput,
} from "../domain/types";

export interface HostListingsView {
  snapshot: HostListingsSnapshot;
  loadCategories(
    token: AccessToken,
    signal?: AbortSignal,
  ): Promise<readonly Category[]>;
  loadCategorySchema(
    token: AccessToken,
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
  ): Promise<{ listingId: string; timeZone: string; etag: string }>;
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
  loadManualBlocks(
    token: AccessToken,
    listingId: string,
    from: string,
    to: string,
    signal?: AbortSignal,
  ): Promise<ManualAvailabilityBlock[]>;
  createManualBlock(
    token: AccessToken,
    listingId: string,
    input: ManualAvailabilityBlockInput,
    signal?: AbortSignal,
  ): Promise<ManualAvailabilityBlock>;
  deleteManualBlock(
    token: AccessToken,
    listingId: string,
    blockId: string,
    signal?: AbortSignal,
  ): Promise<void>;
  loadPhotos(
    token: AccessToken,
    listingId: string,
  ): Promise<readonly GalleryPhoto[]>;
  uploadPhoto(
    token: AccessToken,
    listingId: string,
    input?: PhotoUploadInput,
    idempotencyKey?: string,
    signal?: AbortSignal,
  ): Promise<GalleryPhoto>;
  replacePhoto(
    token: AccessToken,
    listingId: string,
    photoId: string,
    input: PhotoUploadInput,
    key: string,
    ifMatch?: string,
  ): Promise<GalleryPhoto>;
  loadPhotoVariant(
    token: AccessToken,
    listingId: string,
    photoId: string,
    variant: "thumbnail" | "display",
    signal?: AbortSignal,
  ): Promise<Blob>;
  getPhotoUploadStatus(
    token: AccessToken,
    listingId: string,
    key: string,
  ): Promise<GalleryUploadStatus>;
  setCoverPhoto(
    token: AccessToken,
    listingId: string,
    photoId: string,
    onlyIfUnassigned?: boolean,
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
    (token: AccessToken, signal?: AbortSignal) =>
      hostListingsFeatureBridge.loadCategories(token, signal),
    [],
  );

  const loadCategorySchema = useCallback(
    (
      token: AccessToken,
      categoryCode: string,
      version?: number,
      signal?: AbortSignal,
    ) =>
      hostListingsFeatureBridge.loadCategorySchema(
        token,
        categoryCode,
        version,
        signal,
      ),
    [],
  );

  const loadListings = useCallback(
    (token: AccessToken, filter?: { status?: ListingState }) =>
      hostListingsFeatureBridge.loadListings(token, filter),
    [],
  );

  const selectListing = useCallback(
    (token: AccessToken, listingId: string) =>
      hostListingsFeatureBridge.selectListing(token, listingId),
    [],
  );

  const createDraft = useCallback(
    (token: AccessToken, input: ListingDraftInput, idempotencyKey?: string) =>
      hostListingsFeatureBridge.createDraft(token, input, idempotencyKey),
    [],
  );

  const updateDraft = useCallback(
    (
      token: AccessToken,
      listingId: string,
      input: ListingDraftInput,
      ifMatch?: string,
    ) =>
      hostListingsFeatureBridge.updateDraft(token, listingId, input, ifMatch),
    [],
  );

  const deleteListing = useCallback(
    (token: AccessToken, listingId: string) =>
      hostListingsFeatureBridge.deleteListing(token, listingId),
    [],
  );

  const updateContent = useCallback(
    (
      token: AccessToken,
      listingId: string,
      input: ContentUpdateInput,
      ifMatch?: string,
    ) =>
      hostListingsFeatureBridge.updateContent(token, listingId, input, ifMatch),
    [],
  );

  const publishListing = useCallback(
    (token: AccessToken, listingId: string, ifMatch?: string) =>
      hostListingsFeatureBridge.publishListing(token, listingId, ifMatch),
    [],
  );

  const archiveListing = useCallback(
    (token: AccessToken, listingId: string, ifMatch?: string) =>
      hostListingsFeatureBridge.archiveListing(token, listingId, ifMatch),
    [],
  );

  const loadPricing = useCallback(
    (token: AccessToken, listingId: string) =>
      hostListingsFeatureBridge.loadPricing(token, listingId),
    [],
  );

  const updatePricing = useCallback(
    (
      token: AccessToken,
      listingId: string,
      input: PricingInput,
      ifMatch?: string,
    ) =>
      hostListingsFeatureBridge.updatePricing(token, listingId, input, ifMatch),
    [],
  );

  const loadPricingHistory = useCallback(
    (token: AccessToken, listingId: string) =>
      hostListingsFeatureBridge.loadPricingHistory(token, listingId),
    [],
  );

  const configureTimeZone = useCallback(
    (
      token: AccessToken,
      listingId: string,
      timeZone: string,
      ifMatch?: string,
    ) =>
      hostListingsFeatureBridge.configureTimeZone(
        token,
        listingId,
        timeZone,
        ifMatch,
      ),
    [],
  );

  const loadWeeklySchedule = useCallback(
    (token: AccessToken, listingId: string) =>
      hostListingsFeatureBridge.loadWeeklySchedule(token, listingId),
    [],
  );

  const updateWeeklySchedule = useCallback(
    (
      token: AccessToken,
      listingId: string,
      input: ScheduleInput,
      ifMatch?: string,
    ) =>
      hostListingsFeatureBridge.updateWeeklySchedule(
        token,
        listingId,
        input,
        ifMatch,
      ),
    [],
  );

  const loadManualBlocks = useCallback(
    (
      token: AccessToken,
      listingId: string,
      from: string,
      to: string,
      signal?: AbortSignal,
    ) =>
      hostListingsFeatureBridge.listManualBlocks(
        token,
        listingId,
        from,
        to,
        signal,
      ),
    [],
  );

  const createManualBlock = useCallback(
    (
      token: AccessToken,
      listingId: string,
      input: ManualAvailabilityBlockInput,
      signal?: AbortSignal,
    ) =>
      hostListingsFeatureBridge.createManualBlock(
        token,
        listingId,
        input,
        signal,
      ),
    [],
  );

  const deleteManualBlock = useCallback(
    (
      token: AccessToken,
      listingId: string,
      blockId: string,
      signal?: AbortSignal,
    ) =>
      hostListingsFeatureBridge.deleteManualBlock(
        token,
        listingId,
        blockId,
        signal,
      ),
    [],
  );

  const loadPhotos = useCallback(
    (token: AccessToken, listingId: string) =>
      hostListingsFeatureBridge.loadPhotos(token, listingId),
    [],
  );

  const uploadPhoto = useCallback(
    (
      token: AccessToken,
      listingId: string,
      input?: PhotoUploadInput,
      idempotencyKey?: string,
      signal?: AbortSignal,
    ) =>
      hostListingsFeatureBridge.uploadPhoto(
        token,
        listingId,
        input,
        idempotencyKey,
        signal,
      ),
    [],
  );

  const setCoverPhoto = useCallback(
    (
      token: AccessToken,
      listingId: string,
      photoId: string,
      onlyIfUnassigned?: boolean,
    ) =>
      hostListingsFeatureBridge.setCoverPhoto(
        token,
        listingId,
        photoId,
        undefined,
        onlyIfUnassigned,
      ),
    [],
  );

  const replacePhoto = useCallback(
    (
      token: AccessToken,
      listingId: string,
      photoId: string,
      input: PhotoUploadInput,
      key: string,
      ifMatch?: string,
    ) =>
      hostListingsFeatureBridge.replacePhoto(
        token,
        listingId,
        photoId,
        input,
        key,
        ifMatch,
      ),
    [],
  );
  const loadPhotoVariant = useCallback(
    (
      token: AccessToken,
      listingId: string,
      photoId: string,
      variant: "thumbnail" | "display",
      signal?: AbortSignal,
    ) =>
      hostListingsFeatureBridge.loadPhotoVariant(
        token,
        listingId,
        photoId,
        variant,
        signal,
      ),
    [],
  );

  const getPhotoUploadStatus = useCallback(
    (token: AccessToken, listingId: string, key: string) =>
      hostListingsFeatureBridge.getPhotoUploadStatus(token, listingId, key),
    [],
  );

  const removePhoto = useCallback(
    (token: AccessToken, listingId: string, photoId: string) =>
      hostListingsFeatureBridge.removePhoto(token, listingId, photoId),
    [],
  );

  const reorderPhotos = useCallback(
    (token: AccessToken, listingId: string, photoIds: string[]) =>
      hostListingsFeatureBridge.reorderPhotos(token, listingId, photoIds),
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
    loadManualBlocks,
    createManualBlock,
    deleteManualBlock,
    loadPhotos,
    uploadPhoto,
    replacePhoto,
    loadPhotoVariant,
    getPhotoUploadStatus,
    setCoverPhoto,
    removePhoto,
    reorderPhotos,
    setFilter,
    clear,
  };
}
