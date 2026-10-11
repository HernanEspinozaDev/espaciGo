export interface OwnSpaceCoverResponse { cover_photo_id?: unknown; }
export interface OwnSpaceGalleryResponse { items?: unknown; }

export interface OwnGalleryView<TPhoto> {
  coverPhotoID: string;
  photos: TPhoto[];
}

// Re-read both resources after each gallery mutation so the persisted API
// selection, rather than a stale event argument, remains the display source.
export async function loadOwnGalleryView<TPhoto>(
  getSpace: () => Promise<OwnSpaceCoverResponse>,
  getGallery: () => Promise<OwnSpaceGalleryResponse>,
): Promise<OwnGalleryView<TPhoto>> {
  const [space, gallery] = await Promise.all([getSpace(), getGallery()]);
  return {
    coverPhotoID: typeof space.cover_photo_id === "string" ? space.cover_photo_id : "",
    photos: Array.isArray(gallery.items) ? gallery.items as TPhoto[] : [],
  };
}

export function isSelectedCover(photoID: string, coverPhotoID: string): boolean {
  return coverPhotoID !== "" && photoID === coverPhotoID;
}

export function ownSpaceCoverLabel(coverPhotoID: unknown): string {
  return typeof coverPhotoID === "string" && coverPhotoID !== ""
    ? `Portada: ${coverPhotoID}`
    : "Sin portada";
}
