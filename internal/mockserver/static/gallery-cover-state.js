// Re-read both resources after each gallery mutation so the persisted API
// selection, rather than a stale event argument, remains the display source.
export async function loadOwnGalleryView(getSpace, getGallery) {
    const [space, gallery] = await Promise.all([getSpace(), getGallery()]);
    return {
        coverPhotoID: typeof space.cover_photo_id === "string" ? space.cover_photo_id : "",
        photos: Array.isArray(gallery.items) ? gallery.items : [],
    };
}
export function isSelectedCover(photoID, coverPhotoID) {
    return coverPhotoID !== "" && photoID === coverPhotoID;
}
export function ownSpaceCoverLabel(coverPhotoID) {
    return typeof coverPhotoID === "string" && coverPhotoID !== ""
        ? `Portada: ${coverPhotoID}`
        : "Sin portada";
}
