import type {
  AvailabilityConfigResponse,
  AvailabilityResult,
  LocalWeeklyHoursEnvelope,
  ManualBlock,
  ManualBlockList,
  PriceSimulation,
  PriceSimulationInput,
  PublishedSpaceContentInput,
  Rate,
  RateInput,
  SpaceAttributeProfile,
  SpaceCategoryList,
  PublicLocationList,
  SpaceDeletionReceipt,
  SpaceDraft,
  SpaceDraftInput,
  SpaceDraftList,
  SpacePublicationInput,
  SyntheticSpaceCover,
  SyntheticSpaceGalleryPhoto,
  SyntheticSpaceGalleryRemoval,
  SpaceGalleryPhoto,
  SpaceMediaPhoto,
  SpaceMediaPhotoUpdate,
} from '../../../../core/contracts/models';

export class HostingApiError extends Error {
  constructor(
    message = 'Go no pudo completar la solicitud.',
    readonly status?: number,
  ) {
    super(message);
    this.name = 'HostingApiError';
  }
}

export abstract class HostingPort {
  abstract list(token: string): Promise<SpaceDraftList>;
  abstract categories(token: string): Promise<SpaceCategoryList>;
  abstract publicLocations(): Promise<PublicLocationList>;
  abstract profile(
    token: string,
    categoryCode: string,
    version?: number,
  ): Promise<SpaceAttributeProfile>;
  abstract detail(token: string, spaceId: string): Promise<SpaceDraft>;
  abstract create(token: string, payload: SpaceDraftInput): Promise<SpaceDraft>;
  abstract update(token: string, spaceId: string, payload: SpaceDraftInput): Promise<SpaceDraft>;
  abstract updatePublished(
    token: string,
    spaceId: string,
    payload: PublishedSpaceContentInput,
  ): Promise<SpaceDraft>;
  abstract publication(
    token: string,
    spaceId: string,
    payload: SpacePublicationInput,
  ): Promise<SpaceDraft>;
  abstract remove(token: string, spaceId: string): Promise<SpaceDeletionReceipt>;
  abstract gallery(
    token: string,
    spaceId: string,
  ): Promise<{ items: SpaceGalleryPhoto[]; max_photos: number }>;
  abstract uploadPhoto(
    token: string,
    spaceId: string,
    file: File,
    key: string,
    onProgress: (percent: number | null) => void,
  ): Promise<SpaceMediaPhoto>;
  abstract updateMediaPhoto(
    token: string,
    spaceId: string,
    photoId: string,
    update: SpaceMediaPhotoUpdate,
  ): Promise<SpaceMediaPhoto>;
  abstract mediaVariant(token: string, spaceId: string, photoId: string, variant: 'display' | 'thumbnail'): Promise<Blob>;
  abstract addPhoto(
    token: string,
    spaceId: string,
    key: string,
  ): Promise<SyntheticSpaceGalleryPhoto>;
  abstract cover(token: string, spaceId: string, photoId: string): Promise<SyntheticSpaceCover>;
  abstract setPhotoPublic(token: string, spaceId: string, photoId: string, publicInCatalog: boolean): Promise<SyntheticSpaceGalleryPhoto>;
  abstract removePhoto(
    token: string,
    spaceId: string,
    photoId: string,
  ): Promise<SyntheticSpaceGalleryRemoval>;
  abstract photoContent(token: string, spaceId: string, photoId: string): Promise<Blob>;
  abstract timeZone(token: string, spaceId: string): Promise<AvailabilityConfigResponse>;
  abstract configureTimeZone(
    token: string,
    spaceId: string,
    time_zone: string,
  ): Promise<AvailabilityConfigResponse>;
  abstract checkAvailability(
    token: string,
    spaceId: string,
    from: string,
    to: string,
  ): Promise<AvailabilityResult>;
  abstract tariff(token: string, spaceId: string): Promise<Rate>;
  abstract updateTariff(token: string, spaceId: string, payload: RateInput): Promise<Rate>;
  abstract tariffVersions(token: string, spaceId: string): Promise<{ items: Rate[] }>;
  abstract simulate(
    token: string,
    spaceId: string,
    payload: PriceSimulationInput,
  ): Promise<PriceSimulation>;
  abstract blocks(
    token: string,
    spaceId: string,
    from: string,
    to: string,
  ): Promise<ManualBlockList>;
  abstract addBlock(
    token: string,
    spaceId: string,
    payload: { start_at: string; end_at: string; reason: string },
  ): Promise<ManualBlock>;
  abstract removeBlock(token: string, spaceId: string, blockId: string): Promise<void>;
  abstract weekly(token: string, spaceId: string): Promise<LocalWeeklyHoursEnvelope>;
  abstract saveWeekly(
    token: string,
    spaceId: string,
    payload: { enabled: boolean; days: LocalWeeklyHoursEnvelope['data']['days'] },
  ): Promise<LocalWeeklyHoursEnvelope>;
}
