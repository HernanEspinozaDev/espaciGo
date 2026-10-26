import type { PublicSpace, PublicSpaceImageRef } from "./types";

const UUID =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function isPublicUuid(value: string): boolean {
  return UUID.test(value);
}

export function publicImageUrl(
  spaceId: string,
  image: PublicSpaceImageRef,
  variant: "thumbnail" | "display",
): string | null {
  if (!isPublicUuid(spaceId) || !isPublicUuid(image.image_id)) return null;
  const base = `/api/backend/public/spaces/${encodeURIComponent(spaceId)}/images/${encodeURIComponent(image.image_id)}`;
  if (image.source === "synthetic") return base;
  return `${base}/variants/${variant}`;
}

export function publicOriginalImageUrl(
  spaceId: string,
  image: PublicSpaceImageRef,
): string | null {
  if (!isPublicUuid(spaceId) || !isPublicUuid(image.image_id)) return null;
  return `/api/backend/public/spaces/${encodeURIComponent(spaceId)}/images/${encodeURIComponent(image.image_id)}`;
}

export function publicCoverUrl(
  space: PublicSpace,
  variant: "thumbnail" | "display",
): string | null {
  if (!space.cover_image_id || !isPublicUuid(space.space_id)) return null;
  const reference = space.images?.find(
    (image) => image.image_id === space.cover_image_id,
  );
  if (reference) return publicImageUrl(space.space_id, reference, variant);
  return `/api/backend/public/spaces/${encodeURIComponent(space.space_id)}/images/${encodeURIComponent(space.cover_image_id)}`;
}

export function publicGalleryImages(
  spaceId: string,
  images: readonly PublicSpaceImageRef[],
): { src: string; fallbackSrc?: string }[] {
  return images
    .map((image) => {
      const src = publicImageUrl(spaceId, image, "display");
      const fallbackSrc =
        image.source !== "synthetic"
          ? (publicOriginalImageUrl(spaceId, image) ?? undefined)
          : undefined;
      return { src: src ?? "", ...(fallbackSrc ? { fallbackSrc } : {}) };
    })
    .filter((image) => image.src !== "");
}
