import test from "node:test";
import assert from "node:assert/strict";
import { isSelectedCover, loadOwnGalleryView, ownSpaceCoverLabel } from "../../internal/mockserver/static/gallery-cover-state.js";

function fixture() {
  const server = { cover_photo_id: null, items: [] };
  let spaceReads = 0, galleryReads = 0;
  const refresh = () => loadOwnGalleryView(
    async () => { spaceReads++; return { cover_photo_id: server.cover_photo_id }; },
    async () => { galleryReads++; return { items: [...server.items] }; },
  );
  return { server, refresh, reads: () => ({ spaceReads, galleryReads }) };
}

test("elegir A como portada y agregar B conserva A como portada vigente de la API", async () => {
  const { server, refresh, reads } = fixture();
  server.items = [{ id: "A" }];
  server.cover_photo_id = "A"; // Respuesta persistida de PUT /cover.
  let view = await refresh();
  assert.equal(isSelectedCover("A", view.coverPhotoID), true);
  assert.equal(ownSpaceCoverLabel(view.coverPhotoID), "Portada: A");

  server.items.push({ id: "B" }); // Respuesta persistida de POST /gallery.
  view = await refresh();
  assert.equal(isSelectedCover("A", view.coverPhotoID), true);
  assert.equal(isSelectedCover("B", view.coverPhotoID), false);
  assert.equal(ownSpaceCoverLabel(view.coverPhotoID), "Portada: A");
  assert.deepEqual(reads(), { spaceReads: 2, galleryReads: 2 });
});

test("retirar B conserva A marcada tras volver a consultar espacio y galería", async () => {
  const { server, refresh } = fixture();
  server.items = [{ id: "A" }, { id: "B" }];
  server.cover_photo_id = "A";
  await refresh();

  server.items = server.items.filter(photo => photo.id !== "B"); // DELETE /gallery/B.
  const view = await refresh();
  assert.deepEqual(view.photos.map(photo => photo.id), ["A"]);
  assert.equal(isSelectedCover("A", view.coverPhotoID), true);
  assert.equal(ownSpaceCoverLabel(view.coverPhotoID), "Portada: A");
});

test("retirar A actualiza galería y listado para indicar que no hay portada", async () => {
  const { server, refresh } = fixture();
  server.items = [{ id: "A" }];
  server.cover_photo_id = "A";
  await refresh();

  server.items = []; // DELETE /gallery/A clears the DB cover in the same transaction.
  server.cover_photo_id = null;
  const view = await refresh();
  assert.deepEqual(view.photos, []);
  assert.equal(view.coverPhotoID, "");
  assert.equal(ownSpaceCoverLabel(view.coverPhotoID), "Sin portada");
});
