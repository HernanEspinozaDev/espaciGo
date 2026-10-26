import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

test.use({ trace: "off", screenshot: "off", video: "off" });
const enabled = process.env.PHASE4_NATIVE_API_E2E === "1";
const apiBase =
  process.env.API_BACKEND_BASE_URL ?? "http://127.0.0.1:18082/api/v1";
const appOrigin = `http://127.0.0.1:${process.env.PLAYWRIGHT_PORT ?? 4203}`;
const viewports = [
  { name: "390x844", width: 390, height: 844 },
  { name: "1365x900", width: 1365, height: 900 },
] as const;

interface SpaceSummary {
  space_id: string;
  title: string;
  category_code: string;
  images?: { image_id: string }[];
}

interface CategorySummary {
  code: string;
  name: string;
}

let categories: CategorySummary[] = [];
let publicSpaces: SpaceSummary[] = [];
let residentialCount = -1;
let detailCandidate: SpaceSummary | undefined;
let detailImageCount = 0;
let originalImageStatus: number | undefined;
let variantImageStatus: number | undefined;
let termsStatus = 0;
let termsHasContent = false;
let cursorPageStatus = 0;
let cursorDuplicateCount = 0;

async function getApi(path: string) {
  return fetch(new URL(`${apiBase.replace(/\/$/, "")}${path}`), {
    headers: { Accept: "application/json" },
    cache: "no-store",
  });
}

async function captureLocalOnly(page: Page) {
  const externalRequests: string[] = [];
  await page.route("**/*", async (route) => {
    const requestUrl = new URL(route.request().url());
    if (requestUrl.origin !== appOrigin) {
      externalRequests.push(requestUrl.origin);
      await route.abort();
      return;
    }
    await route.continue();
  });
  return externalRequests;
}

test.skip(
  !enabled,
  "Requiere API Go compatible, local y explícitamente habilitada",
);
test.describe.configure({ mode: "serial" });

test.beforeAll(async () => {
  const [
    categoriesResponse,
    locationsResponse,
    spacesResponse,
    residentialResponse,
    filtersResponse,
    termsResponse,
  ] = await Promise.all([
    getApi("/public/space-categories"),
    getApi("/public/locations"),
    getApi("/public/spaces?page_size=48"),
    getApi("/public/spaces?category_code=inmuebles&page_size=3"),
    getApi("/public/space-categories/inmuebles/filters"),
    getApi("/auth/terms"),
  ]);
  expect(categoriesResponse.status).toBe(200);
  expect(locationsResponse.status).toBe(200);
  expect(spacesResponse.status).toBe(200);
  expect(residentialResponse.status).toBe(200);
  expect(filtersResponse.status).toBe(200);
  categories = (
    (await categoriesResponse.json()) as { items: CategorySummary[] }
  ).items;
  publicSpaces = ((await spacesResponse.json()) as { items: SpaceSummary[] })
    .items;
  residentialCount = (
    (await residentialResponse.json()) as { items: unknown[] }
  ).items.length;
  const firstPageResponse = await getApi("/public/spaces?page_size=3");
  expect(firstPageResponse.status).toBe(200);
  const firstPage = (await firstPageResponse.json()) as {
    items: SpaceSummary[];
    next_cursor?: string;
  };
  expect(firstPage.next_cursor).toBeTruthy();
  const nextPageResponse = await getApi(
    `/public/spaces?page_size=3&cursor=${encodeURIComponent(firstPage.next_cursor!)}`,
  );
  cursorPageStatus = nextPageResponse.status;
  expect(cursorPageStatus).toBe(200);
  const nextPage = (await nextPageResponse.json()) as {
    items: SpaceSummary[];
  };
  const firstPageIds = new Set(firstPage.items.map((space) => space.space_id));
  cursorDuplicateCount = nextPage.items.filter((space) =>
    firstPageIds.has(space.space_id),
  ).length;
  termsStatus = termsResponse.status;
  if (termsStatus === 200) {
    const termsBody = (await termsResponse.json()) as {
      items: { type: string; document?: { sections?: unknown[] } }[];
    };
    termsHasContent = Boolean(
      termsBody.items.find(
        (item) => item.type === "terminos" && item.document?.sections,
      ),
    );
  }
  detailCandidate = publicSpaces.find(
    (space) => (space.images ?? []).length > 0,
  );
  if (detailCandidate) {
    const detailResponse = await getApi(
      `/public/spaces/${detailCandidate.space_id}`,
    );
    expect(detailResponse.status).toBe(200);
    const detail = (await detailResponse.json()) as {
      data: { images: { image_id: string }[] };
    };
    detailImageCount = detail.data.images.length;
    if (detailImageCount > 0) {
      const imageId = detail.data.images[0]?.image_id;
      if (imageId) {
        const [original, variant] = await Promise.all([
          getApi(
            `/public/spaces/${detailCandidate.space_id}/images/${imageId}`,
          ),
          getApi(
            `/public/spaces/${detailCandidate.space_id}/images/${imageId}/variants/display`,
          ),
        ]);
        originalImageStatus = original.status;
        variantImageStatus = variant.status;
      }
    }
  }
  expect(categories.some((category) => category.code === "inmuebles")).toBe(
    true,
  );
  expect(publicSpaces.length).toBeGreaterThan(0);
});

const accessibleRoutes = [
  { name: "portada", path: "/" },
  { name: "explorar", path: "/explorar?page_size=3" },
  { name: "categorias", path: "/categorias" },
  { name: "categoria-inmuebles", path: "/categorias/inmuebles" },
  { name: "arriendos-residenciales", path: "/arriendos" },
  { name: "anfitriones", path: "/anfitriones" },
  { name: "como-funciona", path: "/como-funciona" },
  { name: "terminos", path: "/terminos-y-condiciones" },
  { name: "privacidad", path: "/politica-de-privacidad" },
] as const;

for (const route of accessibleRoutes) {
  for (const viewport of viewports) {
    test(`@a11y ${route.name} ${viewport.name}: loopback, responsive y Axe`, async ({
      page,
    }) => {
      await page.setViewportSize({
        width: viewport.width,
        height: viewport.height,
      });
      await page.emulateMedia({ reducedMotion: "reduce" });
      const externalRequests = await captureLocalOnly(page);
      const response = await page.goto(route.path, {
        waitUntil: "networkidle",
      });
      expect(response?.status(), route.name).toBe(200);
      await expect(page.getByRole("main")).toBeVisible();
      await expect(page.locator("main h1").first()).toBeVisible();
      const widths = await page.evaluate(() => ({
        viewport: document.documentElement.clientWidth,
        document: document.documentElement.scrollWidth,
        body: document.body.scrollWidth,
      }));
      expect(widths.document).toBeLessThanOrEqual(widths.viewport);
      expect(widths.body).toBeLessThanOrEqual(widths.viewport);
      expect(externalRequests).toEqual([]);
      const violations = await new AxeBuilder({ page })
        .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
        .analyze();
      expect(
        violations.violations.map((violation) => violation.id),
        `${route.name} ${viewport.name}`,
      ).toEqual([]);
    });
  }
}

test("SSR incluye resultados elegibles de Go y el navegador no consulta el catálogo directamente", async ({
  page,
}) => {
  const externalRequests = await captureLocalOnly(page);
  const response = await fetch(`${appOrigin}/explorar?page_size=3`, {
    cache: "no-store",
  });
  expect(response.status).toBe(200);
  const html = await response.text();
  expect(html).toContain("Publicaciones del catálogo");
  expect(
    (html.match(/text-base font-medium text-foreground/g) ?? []).length,
  ).toBeGreaterThan(0);
  await page.goto("/explorar?page_size=3", { waitUntil: "networkidle" });
  const browserCatalogJsonCalls = await page.evaluate(() =>
    performance
      .getEntriesByType("resource")
      .map((entry) => new URL(entry.name).pathname)
      .filter(
        (pathname) =>
          pathname.startsWith("/api/backend/public/") &&
          !pathname.includes("/images/"),
      ),
  );
  expect(browserCatalogJsonCalls).toEqual([]);
  expect(externalRequests).toEqual([]);
});

test("arriendos residenciales refleja el inventario público real sin inventar anuncios", async ({
  page,
}) => {
  expect(residentialCount).toBe(0);
  const externalRequests = await captureLocalOnly(page);
  await page.goto("/arriendos", { waitUntil: "networkidle" });
  await expect(
    page.getByRole("heading", { name: "Arrienda una casa" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", {
      name: "Aún no hay publicaciones residenciales",
    }),
  ).toBeVisible();
  expect(
    await page
      .getByRole("link", { name: "Conoce cómo ofrecer tu inmueble" })
      .getAttribute("href"),
  ).toBe("/anfitriones");
  expect(externalRequests).toEqual([]);
});

test("perfil de filtros y cursor respetan el contrato público del catálogo Go", async () => {
  expect(cursorPageStatus).toBe(200);
  expect(cursorDuplicateCount).toBe(0);
  expect(residentialCount).toBe(0);
});

test("CTAs de oferta informan el límite y llevan a anfitriones o al ingreso existente", async ({
  page,
}) => {
  await page.goto("/anfitriones");
  await expect(
    page.getByRole("heading", { name: /casa de veraneo/i }),
  ).toBeVisible();
  await expect(
    page.getByText(/crear y publicar anuncios todavía no está habilitado/i),
  ).toBeVisible();
  await expect(
    page.getByRole("main").getByRole("link", { name: "Ingresar a EspaciGo" }),
  ).toHaveAttribute("href", "/login");
  await expect(
    page.getByText(/No hay formulario de captación o lista de espera/i),
  ).toBeVisible();
});

test("rutas legacy conservan query compatible y slugs de fixtures dan 404", async ({
  page,
}) => {
  await page.goto("/products?category_code=inmuebles&q=casa", {
    waitUntil: "networkidle",
  });
  await expect(page).toHaveURL(/\/explorar\?q=casa&category_code=inmuebles$/);
  await page.goto("/search?q=estudio", { waitUntil: "networkidle" });
  await expect(page).toHaveURL(/\/explorar\?q=estudio$/);
  const invalidSpace = await page.goto("/product/sala-luz-providencia");
  expect(invalidSpace?.status()).toBe(404);
  const invalidCategory = await page.goto("/categorias/codigo-inexistente");
  expect(invalidCategory?.status()).toBe(404);
});

test("ficha real muestra solo DTO público y resuelve imagen con fallback legacy", async ({
  page,
}) => {
  test.skip(
    !detailCandidate,
    "El endpoint público no entregó un espacio elegible",
  );
  if (!detailCandidate) return;
  const externalRequests = await captureLocalOnly(page);
  await page.setViewportSize({ width: 1365, height: 900 });
  const response = await page.goto(`/espacios/${detailCandidate.space_id}`, {
    waitUntil: "networkidle",
  });
  expect(response?.status()).toBe(200);
  await expect(
    page.getByRole("heading", { name: detailCandidate.title }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: /reservar|pagar/i }),
  ).toHaveCount(0);
  if (detailImageCount === 0) {
    await expect(page.getByText("No hay imágenes disponibles")).toHaveCount(1);
  } else if (variantImageStatus === 404 && originalImageStatus === 404) {
    await expect(page.getByText("Imagen no disponible")).toHaveCount(1);
  } else {
    await expect(page.locator('img[alt^="Vista de"]')).toBeVisible();
  }
  expect(externalRequests).toEqual([]);
});

test("documentos legales reflejan Go y muestran indisponibilidad segura ante 503", async ({
  page,
}) => {
  const response = await getApi("/auth/terms");
  termsStatus = response.status;
  await page.goto("/terminos-y-condiciones", { waitUntil: "networkidle" });
  if (termsStatus === 503) {
    await expect(
      page.getByText(/documentos legales no están disponibles/i),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: /documento sintético/i }),
    ).toHaveCount(0);
  } else {
    expect(termsStatus).toBe(200);
    if (termsHasContent) {
      await expect(
        page.getByText(/Documento sintético de ensayo local/i),
      ).toBeVisible();
    } else {
      await expect(
        page.getByRole("heading", {
          name: "Contenido legal no disponible",
        }),
      ).toBeVisible();
      await expect(
        page.getByText(/No se mostrará texto legal sustituto/i),
      ).toBeVisible();
    }
  }
});

test("skip link es el primer foco y aterriza en el contenido principal", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/", { waitUntil: "networkidle" });
  await page.keyboard.press("Tab");
  const skipLink = page.getByRole("link", {
    name: "Omitir al contenido principal",
  });
  await expect(skipLink).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.locator("#main-content")).toBeFocused();
});
