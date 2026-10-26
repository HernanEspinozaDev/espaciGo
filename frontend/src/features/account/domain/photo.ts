export const SYNTHETIC_PHOTO_FIXTURE_CODE = "synthetic-png-v1" as const;
export const MAX_PHOTO_SIZE_BYTES = 2 * 1024 * 1024; // 2MB limit per RQF-030

export interface SyntheticProfilePhoto {
  readonly id: string;
  readonly fixtureCode: typeof SYNTHETIC_PHOTO_FIXTURE_CODE;
  readonly mimeType: "image/png";
  readonly sha256: string;
  readonly sizeBytes: number;
  readonly createdAt: string;
  readonly storageKind: "synthetic_fixed";
}

export interface UploadedAvatarProfilePhoto {
  readonly id: string;
  readonly mimeType: "image/webp";
  readonly sha256: string;
  readonly sizeBytes: number;
  readonly width?: number;
  readonly height?: number;
  readonly createdAt: string;
  readonly storageKind: "uploaded_avatar";
}

export type ProfilePhotoMetadata =
  SyntheticProfilePhoto | UploadedAvatarProfilePhoto;

export function isSyntheticPhoto(
  photo: ProfilePhotoMetadata | null | undefined,
): photo is SyntheticProfilePhoto {
  if (!photo) return false;
  return (
    photo.storageKind === "synthetic_fixed" ||
    ("fixtureCode" in photo &&
      photo.fixtureCode === SYNTHETIC_PHOTO_FIXTURE_CODE)
  );
}

export function isValidPhotoSize(sizeBytes: number): boolean {
  return sizeBytes > 0 && sizeBytes <= MAX_PHOTO_SIZE_BYTES;
}
