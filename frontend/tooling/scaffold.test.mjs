import assert from "node:assert/strict";
import { access, readFile, readdir } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const read = (relativePath) => readFile(path.join(root, relativePath), "utf8");
const exists = async (relativePath) =>
  access(path.join(root, relativePath)).then(
    () => true,
    () => false,
  );

async function collectSource(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(
    entries.map(async (entry) => {
      const absolutePath = path.join(directory, entry.name);
      if (entry.isDirectory()) return collectSource(absolutePath);
      if (!/\.(?:ts|tsx|mjs)$/.test(entry.name)) return [];
      return [await readFile(absolutePath, "utf8")];
    }),
  );
  return files.flat().join("\n");
}

test("the single Next application identifies EspaciGo and supports Spanish navigation", async () => {
  const layout = await read("src/app/layout.tsx");
  const marketplace = await read("src/app/(marketplace)/layout.tsx");
  assert.match(layout, /lang="es"/);
  assert.match(layout, /Omitir al contenido principal/);
  assert.match(layout, /EspaciGo/);
  assert.match(marketplace, /EspaciGo/);
  assert.match(await read("public/espacigo-mark.svg"), /<svg/);
});

test("marketplace routes use the Go public catalog without local listing fixtures", async () => {
  const home = await read("src/app/(marketplace)/page.tsx");
  const explore = await read("src/app/(marketplace)/explorar/page.tsx");
  const detail = await read("src/app/(marketplace)/espacios/[id]/page.tsx");
  const grid = await read("src/components/sections/product-grid.tsx");
  const source = await collectSource(path.join(root, "src"));
  for (const templateSection of [
    "Hero",
    "ProductGrid",
    "CategoryRail",
    "OwnerCallout",
  ]) {
    assert.match(home, new RegExp(templateSection));
  }
  assert.match(home, /publicCatalog\.spaces/);
  assert.match(explore, /spacesFromParams/);
  assert.match(detail, /publicCatalog\.space/);
  assert.match(detail, /MediaGallery/);
  assert.match(grid, /PublicCatalogCard/);
  assert.match(
    await read("src/app/(marketplace)/products/page.tsx"),
    /redirectLegacyCatalog/,
  );
  assert.match(
    await read("src/app/(marketplace)/search/page.tsx"),
    /redirectLegacyCatalog/,
  );
  assert.doesNotMatch(source, /marketplace-fixtures/);
  assert.equal(await exists("src/lib/marketplace-fixtures.ts"), false);
  assert.equal(await exists("src/app/search-suggest.ts"), false);
});

test("SaaS Starter pages are present; auth uses its API facade and pricing stays demo-only", async () => {
  for (const route of [
    "src/app/(workspace)/saas/page.tsx",
    "src/app/(workspace)/login/page.tsx",
    "src/app/(workspace)/dashboard/page.tsx",
    "src/app/(workspace)/dashboard/general/page.tsx",
    "src/app/(workspace)/dashboard/security/page.tsx",
    "src/app/(workspace)/dashboard/activity/page.tsx",
  ]) {
    assert.equal(await exists(route), true, `missing route file ${route}`);
  }
  const login = await read("src/app/(workspace)/login.tsx");
  const pricing = await read("src/app/(workspace)/pricing/page.tsx");
  assert.match(login, /useAuth/);
  assert.match(login, /await login\(\{ email, password \}\)/);
  assert.doesNotMatch(login, /\bfetch\s*\(/);
  assert.match(login, /disabled/);
  assert.match(pricing, /no funcional/i);
  assert.match(pricing, /no hay precios, suscripciones, checkout/);
  assert.match(pricing, /disabled/);
  assert.equal(await exists("src/app/(workspace)/panel/page.tsx"), false);
  assert.equal(
    await exists("src/app/(marketplace)/design-system/page.tsx"),
    false,
  );
});

test("copied source notice records both frozen commits and MIT licensing", async () => {
  const notice = await read("THIRD_PARTY_NOTICES.md");
  assert.match(notice, /1de63c614f4c57c1862b1b14710f2133784749c1/);
  assert.match(notice, /6e33e58b1e553a41fe22e6b941a7229a002de361/);
  assert.match(notice, /MIT/);
});

test("application source contains no removed commerce, billing, database, or auth SDK imports", async () => {
  const source = await collectSource(path.join(root, "src"));
  assert.doesNotMatch(
    source,
    /from\s+["'](?:@your-next-store\/commerce-kit|@\/lib\/commerce|@\/lib\/db|stripe|drizzle-orm|bcrypt|jose|jsonwebtoken)["']/i,
  );
  assert.doesNotMatch(source, /\b(?:cookies|headers)\(\)/);
});
