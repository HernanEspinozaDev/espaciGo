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
  PhotoUploadInput,
  PricingInput,
  PricingHistoryItem,
  ScheduleData,
  ScheduleInput,
} from "../domain/types";

export interface HostListingsPort {
  listCategories(signal?: AbortSignal): Promise<Category[]>;
  getCategoryProfile(
    categoryCode: string,
    version?: number,
    signal?: AbortSignal,
  ): Promise<CategoryProfileSchema | null>;
  listListings(
    filter?: { status?: ListingState },
    signal?: AbortSignal,
  ): Promise<ListingSummary[]>;
  getListing(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail | null>;
  createDraft(
    input: ListingDraftInput,
    idempotencyKey?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail>;
  updateDraft(
    listingId: string,
    input: ListingDraftInput,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail>;
  deleteListing(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<ListingDeletionReceipt>;
  updateContent(
    listingId: string,
    input: ContentUpdateInput,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail>;
  publishListing(
    listingId: string,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail>;
  archiveListing(
    listingId: string,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail>;
  getPricing(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<PricingHistoryItem | null>;
  updatePricing(
    listingId: string,
    input: PricingInput,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<PricingHistoryItem>;
  listPricingHistory(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<PricingHistoryItem[]>;
  configureTimeZone(
    listingId: string,
    timeZone: string,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<{ listingId: string; timeZone: string; etag: string }>;
  getWeeklySchedule(
    listingId: string,
    signal?: AbortSignal,
  ): Promise<ScheduleData | null>;
  updateWeeklySchedule(
    listingId: string,
    input: ScheduleInput,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<ScheduleData>;
  listPhotos(listingId: string, signal?: AbortSignal): Promise<GalleryPhoto[]>;
  getPhotoUploadStatus(
    listingId: string,
    idempotencyKey: string,
    signal?: AbortSignal,
  ): Promise<GalleryUploadStatus>;
  uploadPhoto(
    listingId: string,
    input?: PhotoUploadInput,
    idempotencyKey?: string,
    signal?: AbortSignal,
  ): Promise<GalleryPhoto>;
  replacePhoto(
    listingId: string,
    photoId: string,
    input: PhotoUploadInput,
    idempotencyKey: string,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<GalleryPhoto>;
  setCoverPhoto(
    listingId: string,
    photoId: string,
    signal?: AbortSignal,
  ): Promise<void>;
  removePhoto(
    listingId: string,
    photoId: string,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<string>;
  movePhoto(
    listingId: string,
    photoId: string,
    targetPosition: number,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<string>;
}
