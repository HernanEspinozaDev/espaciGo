import type { AuthFacade } from "@/features/auth/application/auth-facade";
import type { AccessToken } from "@/features/auth/application/auth.port";
import type { HostListingsFacade } from "./host-listings-facade";
import type {
  ContentUpdateInput,
  ListingDraftInput,
  PhotoUploadInput,
  PricingInput,
  ScheduleInput,
} from "../domain/types";

// Compatibility at the UI boundary only. Core ports remain cookie-based and
// tokenless; a scope marker prevents a stale UI callback using a newer cookie.
export function createHostListingsFeatureBridge(
  core: HostListingsFacade,
  auth: Pick<AuthFacade, "getAccessToken">,
) {
  function requireScope(scope: AccessToken): void {
    if (!scope || scope !== auth.getAccessToken())
      throw new DOMException("La sesión cambió.", "AbortError");
  }
  function scoped<A extends unknown[], R>(
    operation: (...args: A) => Promise<R>,
  ) {
    return async (scope: AccessToken, ...args: A): Promise<R> => {
      requireScope(scope);
      return operation(...args);
    };
  }
  function revision(listingId: string, expected?: string): string {
    const selected = core.getSnapshot().selectedListing;
    const value =
      expected ?? (selected?.id === listingId ? selected.etag : undefined);
    if (!value)
      throw new Error("Actualiza el espacio antes de guardar cambios.");
    return value;
  }
  return {
    loadCategories: scoped(core.loadCategories.bind(core)),
    loadCategorySchema: scoped(core.loadCategorySchema.bind(core)),
    loadListings: scoped(core.loadListings.bind(core)),
    selectListing: scoped(core.selectListing.bind(core)),
    createDraft: scoped(core.createDraft.bind(core)),
    updateDraft: scoped(
      (
        listingId: string,
        input: ListingDraftInput,
        expected?: string,
        signal?: AbortSignal,
      ) =>
        core.updateDraft(
          listingId,
          input,
          revision(listingId, expected),
          signal,
        ),
    ),
    deleteListing: scoped(core.deleteListing.bind(core)),
    updateContent: scoped(
      (
        listingId: string,
        input: ContentUpdateInput,
        expected?: string,
        signal?: AbortSignal,
      ) =>
        core.updateContent(
          listingId,
          input,
          revision(listingId, expected),
          signal,
        ),
    ),
    publishListing: scoped(
      (listingId: string, expected?: string, signal?: AbortSignal) =>
        core.publishListing(listingId, revision(listingId, expected), signal),
    ),
    archiveListing: scoped(
      (listingId: string, expected?: string, signal?: AbortSignal) =>
        core.archiveListing(listingId, revision(listingId, expected), signal),
    ),
    loadPricing: scoped(core.loadPricing.bind(core)),
    updatePricing: scoped(
      (
        listingId: string,
        input: PricingInput,
        expected?: string,
        signal?: AbortSignal,
      ) =>
        core.updatePricing(
          listingId,
          input,
          revision(listingId, expected),
          signal,
        ),
    ),
    loadPricingHistory: scoped(core.loadPricingHistory.bind(core)),
    configureTimeZone: scoped(
      (
        listingId: string,
        timeZone: string,
        expected?: string,
        signal?: AbortSignal,
      ) =>
        core.configureTimeZone(
          listingId,
          timeZone,
          revision(listingId, expected),
          signal,
        ),
    ),
    loadWeeklySchedule: scoped(core.loadWeeklySchedule.bind(core)),
    updateWeeklySchedule: scoped(
      (
        listingId: string,
        input: ScheduleInput,
        expected?: string,
        signal?: AbortSignal,
      ) =>
        core.updateWeeklySchedule(
          listingId,
          input,
          revision(listingId, expected),
          signal,
        ),
    ),
    listManualBlocks: scoped(core.listManualBlocks.bind(core)),
    createManualBlock: scoped(core.createManualBlock.bind(core)),
    deleteManualBlock: scoped(core.deleteManualBlock.bind(core)),
    loadPhotos: scoped(core.loadPhotos.bind(core)),
    uploadPhoto: scoped(core.uploadPhoto.bind(core)),
    replacePhoto: scoped(
      (
        listingId: string,
        photoId: string,
        input: PhotoUploadInput,
        key: string,
        expected?: string,
        signal?: AbortSignal,
      ) =>
        core.replacePhoto(
          listingId,
          photoId,
          input,
          key,
          revision(listingId, expected),
          signal,
        ),
    ),
    setCoverPhoto: scoped(core.setCoverPhoto.bind(core)),
    removePhoto: scoped(
      async (
        listingId: string,
        photoId: string,
        expected?: string,
        signal?: AbortSignal,
      ) => {
        await core.removePhoto(
          listingId,
          photoId,
          revision(listingId, expected),
          signal,
        );
      },
    ),
    reorderPhotos: scoped(core.reorderPhotos.bind(core)),
    getPhotoUploadStatus: scoped(core.getPhotoUploadStatus.bind(core)),
    loadPhotoVariant: scoped(core.loadPhotoVariant.bind(core)),
  };
}
