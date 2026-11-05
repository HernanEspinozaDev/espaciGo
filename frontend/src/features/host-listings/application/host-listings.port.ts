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
  PhotoUploadInput,
  PricingInput,
  PricingHistoryItem,
  ScheduleData,
  ScheduleInput,
} from "../domain/types";

export interface HostListingsPort {
  listCategories(token: AccessToken, signal?: AbortSignal): Promise<Category[]>;
  getCategoryProfile(
    token: AccessToken,
    categoryCode: string,
    version?: number,
    signal?: AbortSignal,
  ): Promise<CategoryProfileSchema | null>;
  listListings(
    token: AccessToken,
    filter?: { status?: ListingState },
    signal?: AbortSignal,
  ): Promise<ListingSummary[]>;
  getListing(
    token: AccessToken,
    listingId: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail | null>;
  createDraft(
    token: AccessToken,
    input: ListingDraftInput,
    idempotencyKey?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail>;
  updateDraft(
    token: AccessToken,
    listingId: string,
    input: ListingDraftInput,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail>;
  deleteListing(
    token: AccessToken,
    listingId: string,
    signal?: AbortSignal,
  ): Promise<ListingDeletionReceipt>;
  updateContent(
    token: AccessToken,
    listingId: string,
    input: ContentUpdateInput,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail>;
  publishListing(
    token: AccessToken,
    listingId: string,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail>;
  archiveListing(
    token: AccessToken,
    listingId: string,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ListingDetail>;
  getPricing(
    token: AccessToken,
    listingId: string,
    signal?: AbortSignal,
  ): Promise<PricingHistoryItem | null>;
  updatePricing(
    token: AccessToken,
    listingId: string,
    input: PricingInput,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<PricingHistoryItem>;
  listPricingHistory(
    token: AccessToken,
    listingId: string,
    signal?: AbortSignal,
  ): Promise<PricingHistoryItem[]>;
  configureTimeZone(
    token: AccessToken,
    listingId: string,
    timeZone: string,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<{ listingId: string; timeZone: string }>;
  getWeeklySchedule(
    token: AccessToken,
    listingId: string,
    signal?: AbortSignal,
  ): Promise<ScheduleData | null>;
  updateWeeklySchedule(
    token: AccessToken,
    listingId: string,
    input: ScheduleInput,
    ifMatch?: string,
    signal?: AbortSignal,
  ): Promise<ScheduleData>;
  listPhotos(
    token: AccessToken,
    listingId: string,
    signal?: AbortSignal,
  ): Promise<GalleryPhoto[]>;
  uploadPhoto(
    token: AccessToken,
    listingId: string,
    input?: PhotoUploadInput,
    idempotencyKey?: string,
    signal?: AbortSignal,
  ): Promise<GalleryPhoto>;
  getPhotoUploadStatus?(
    token: AccessToken,
    listingId: string,
    key: string,
    signal?: AbortSignal,
  ): Promise<GalleryUploadStatus>;
  replacePhoto?(
    token: AccessToken,
    listingId: string,
    photoId: string,
    input: PhotoUploadInput,
    key: string,
    ifMatch: string,
    signal?: AbortSignal,
  ): Promise<GalleryPhoto>;
  getPhotoVariant?(
    token: AccessToken,
    listingId: string,
    photoId: string,
    variant: "thumbnail" | "display",
    signal?: AbortSignal,
  ): Promise<Blob>;
  setCoverPhoto(
    token: AccessToken,
    listingId: string,
    photoId: string,
    signal?: AbortSignal,
    ifMatch?: string,
  ): Promise<void>;
  removePhoto(
    token: AccessToken,
    listingId: string,
    photoId: string,
    signal?: AbortSignal,
    ifMatch?: string,
  ): Promise<void>;
  reorderPhotos(
    token: AccessToken,
    listingId: string,
    photoIds: string[],
    signal?: AbortSignal,
    ifMatch?: string,
  ): Promise<void>;
}
