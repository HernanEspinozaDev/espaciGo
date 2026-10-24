import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const root = new URL("../", import.meta.url);

test("public route uses the EspaciGo marketplace shell", async () => {
  const page = await readFile(
    new URL("src/app/(marketplace)/page.tsx", root),
    "utf8",
  );
  assert.match(page, /EspaciGo|Encuentra el espacio/);
  assert.match(page, /datos de ejemplo/);
  assert.doesNotMatch(page, /Next\.js logo|Deploy Now|Templates/);
});

test("layout sets Spanish metadata and the EspaciGo icon", async () => {
  const layout = await readFile(new URL("src/app/layout.tsx", root), "utf8");
  assert.match(layout, /lang="es"/);
  assert.match(layout, /Omitir al contenido principal/);
  assert.match(
    await readFile(new URL("src/app/icon.svg", root), "utf8"),
    /EspaciGo/,
  );
});

test("the local design system includes both visual shells and all feedback states", async () => {
  const designPage = await readFile(
    new URL("src/app/(marketplace)/design-system/page.tsx", root),
    "utf8",
  );
  const workspace = await readFile(
    new URL("src/app/(workspace)/panel/page.tsx", root),
    "utf8",
  );
  assert.match(designPage, /loading/);
  assert.match(designPage, /restricted/);
  assert.match(designPage, /offline/);
  assert.match(workspace, /Datos sintéticos/);
  assert.match(workspace, /no crea ni modifica\s+publicaciones/);
});
