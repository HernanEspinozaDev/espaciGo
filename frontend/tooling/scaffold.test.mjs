import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const root = new URL("../", import.meta.url);

test("root route uses the EspaciGo neutral shell", async () => {
  const page = await readFile(new URL("src/app/page.tsx", root), "utf8");
  assert.match(page, /EspaciGo/);
  assert.match(page, /Encuentra tu espacio/);
  assert.doesNotMatch(page, /Next\.js logo|Deploy Now|Templates/);
});

test("layout sets Spanish metadata and the EspaciGo icon", async () => {
  const layout = await readFile(new URL("src/app/layout.tsx", root), "utf8");
  assert.match(layout, /lang="es"/);
  assert.match(layout, /title: "EspaciGo"/);
  assert.match(layout, /espacigo-mark\.svg/);
});
