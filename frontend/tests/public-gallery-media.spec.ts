import { expect, test, type Page, type Request } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

const spaceId = "00000000-0000-4000-8000-000000000007";
const imageIds = [
  "00000000-0000-4000-8000-000000000071",
  "00000000-0000-4000-8000-000000000072",
];
const viewports = [
  { width: 1365, height: 900 },
  { width: 390, height: 844 },
];
const publicMediaPath = (imageId: string) =>
  `/api/backend/public/spaces/${spaceId}/images/${imageId}/variants/display`;
const publicMediaPrefix = `/api/backend/public/spaces/${spaceId}/images/`;

type PublicImageExpectation = {
  pathname: string;
  status: number;
  mime?: string;
  cache?: string;
};

function trackPublicImageEvents(page: Page) {
  const requests: Array<{
    request: Request;
    method: string;
    pathname: string;
  }> = [];
  const responses: Array<{
    request: Request;
    pathname: string;
    status: number;
    mime: string;
    cache: string;
  }> = [];

  page.on("request", (request) => {
    const pathname = new URL(request.url()).pathname;
    if (pathname.startsWith(publicMediaPrefix)) {
      requests.push({ request, method: request.method(), pathname });
    }
  });
  page.on("response", (response) => {
    const pathname = new URL(response.url()).pathname;
    if (pathname.startsWith(publicMediaPrefix)) {
      responses.push({
        request: response.request(),
        pathname,
        status: response.status(),
        mime: response.headers()["content-type"] ?? "",
        cache: response.headers()["cache-control"] ?? "",
      });
    }
  });

  return { requests, responses };
}

function assertPublicImageEvents(
  events: ReturnType<typeof trackPublicImageEvents>,
  requestStart: number,
  expected: PublicImageExpectation[],
) {
  const expectedPaths = expected.map(({ pathname }) => pathname);
  const requests = events.requests
    .slice(requestStart)
    .filter((request) => expectedPaths.includes(request.pathname));
  const responses = events.responses.filter(
    (response) =>
      expectedPaths.includes(response.pathname) &&
      requests.some((request) => request.request === response.request),
  );

  for (const request of requests) {
    expect(request.method).toBe("GET");
    expect(expectedPaths).toContain(request.pathname);
    expect(
      responses.filter((response) => response.request === request.request),
    ).toHaveLength(1);
  }
  for (const pathname of expectedPaths) {
    expect(requests.some((request) => request.pathname === pathname)).toBe(
      true,
    );
  }
  expect(responses).toHaveLength(requests.length);
  for (const response of responses) {
    const expectation = expected.find(
      (candidate) => candidate.pathname === response.pathname,
    );
    expect(expectation).toBeDefined();
    expect(
      requests.some((request) => request.request === response.request),
    ).toBe(true);
    expect(response.status).toBe(expectation?.status);
    if (expectation?.mime !== undefined) {
      expect(response.mime).toBe(expectation.mime);
    }
    if (expectation?.cache !== undefined) {
      expect(response.cache).toBe(expectation.cache);
    }
  }
}

test("public listing renders and navigates its eligible image gallery", async ({
  page,
}) => {
  const imageEvents = trackPublicImageEvents(page);
  const writes: string[] = [];
  page.on("request", (request) => {
    if (!["GET", "HEAD", "OPTIONS"].includes(request.method()))
      writes.push(`${request.method()} ${request.url()}`);
  });
  for (const viewport of viewports) {
    await page.setViewportSize(viewport);
    const requestStart = imageEvents.requests.length;
    const response = await page.goto(`/espacios/${spaceId}`);
    expect(response?.status()).toBe(200);
    const renderedHtml = (await response?.text()) ?? "";
    expect(renderedHtml).not.toContain("PRIVATE_OWNER_ID_MARKER");
    expect(renderedHtml).not.toContain("PRIVATE_ADDRESS_MARKER");
    expect(renderedHtml).not.toContain("PRIVATE_OBJECT_KEY_MARKER");

    const gallery = page.getByRole("region", {
      name: "Galería de imágenes del espacio",
    });
    await expect(gallery).toBeVisible();
    const firstImage = gallery.getByRole("img", {
      name: "Espacio sintético — vista 1",
    });
    await expect(firstImage).toBeVisible();
    await expect
      .poll(() =>
        firstImage.evaluate(
          (image) =>
            image instanceof HTMLImageElement &&
            image.complete &&
            image.naturalWidth > 0,
        ),
      )
      .toBe(true);
    await expect(gallery.getByText("1 / 2", { exact: true })).toBeVisible();
    assertPublicImageEvents(imageEvents, requestStart, [
      {
        pathname: publicMediaPath(imageIds[0]),
        status: 200,
        mime: "image/webp",
        cache: "no-store",
      },
    ]);

    await gallery.getByRole("button", { name: "Siguiente imagen" }).click();
    const secondImage = gallery.getByRole("img", {
      name: "Espacio sintético — vista 2",
    });
    await expect(secondImage).toBeVisible();
    await expect
      .poll(() =>
        secondImage.evaluate(
          (image) =>
            image instanceof HTMLImageElement &&
            image.complete &&
            image.naturalWidth > 0,
        ),
      )
      .toBe(true);
    await expect(gallery.getByText("2 / 2", { exact: true })).toBeVisible();
    assertPublicImageEvents(imageEvents, requestStart, [
      {
        pathname: publicMediaPath(imageIds[0]),
        status: 200,
        mime: "image/webp",
        cache: "no-store",
      },
      {
        pathname: publicMediaPath(imageIds[1]),
        status: 200,
        mime: "image/webp",
        cache: "no-store",
      },
    ]);

    const a11y = await new AxeBuilder({ page }).analyze();
    expect(a11y.violations).toEqual([]);
  }

  expect(writes).toEqual([]);
});

test("public gallery recovers from a missing display variant through legacy media", async ({
  page,
}) => {
  const imageId = imageIds[0];
  const variantPath = publicMediaPath(imageId);
  const originalPath = `/api/backend/public/spaces/${spaceId}/images/${imageId}`;
  const imageEvents = trackPublicImageEvents(page);
  const pageMutationRequests: string[] = [];
  page.on("request", (request) => {
    if (!["GET", "HEAD", "OPTIONS"].includes(request.method())) {
      pageMutationRequests.push(`${request.method()} ${request.url()}`);
    }
  });
  await page.route(`**${variantPath}`, (route) =>
    route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({ error: { code: "not_found" } }),
    }),
  );
  await page.route(`**${originalPath}`, (route) =>
    route.fulfill({
      status: 200,
      contentType: "image/webp",
      body: Buffer.from(
        "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEAAUAmJaQAA3AA/v89WAAAAA==",
        "base64",
      ),
      headers: { "cache-control": "no-store" },
    }),
  );

  for (const viewport of viewports) {
    await page.setViewportSize(viewport);
    const requestStart = imageEvents.requests.length;
    const response = await page.goto(`/espacios/${spaceId}`);
    expect(response?.status()).toBe(200);
    const gallery = page.getByRole("region", {
      name: "Galería de imágenes del espacio",
    });
    const firstImage = gallery.getByRole("img", {
      name: "Espacio sintético — vista 1",
    });
    await expect(firstImage).toBeVisible();
    await expect
      .poll(() =>
        firstImage.evaluate(
          (image) =>
            image instanceof HTMLImageElement &&
            image.complete &&
            image.naturalWidth > 0,
        ),
      )
      .toBe(true);
    await expect(gallery.getByText("Imagen no disponible")).toHaveCount(0);
    assertPublicImageEvents(imageEvents, requestStart, [
      { pathname: variantPath, status: 404 },
      {
        pathname: originalPath,
        status: 200,
        mime: "image/webp",
        cache: "no-store",
      },
    ]);
    const recoveryA11y = await new AxeBuilder({ page })
      .include('[aria-label="Galería de imágenes del espacio"]')
      .analyze();
    expect(recoveryA11y.violations).toEqual([]);
  }
  expect(pageMutationRequests).toEqual([]);
});

test("public gallery shows its current placeholder when display and legacy media are unavailable", async ({
  page,
}) => {
  const imageId = imageIds[0];
  const variantPath = publicMediaPath(imageId);
  const originalPath = `/api/backend/public/spaces/${spaceId}/images/${imageId}`;
  const imageEvents = trackPublicImageEvents(page);
  const pageMutationRequests: string[] = [];
  page.on("request", (request) => {
    if (!["GET", "HEAD", "OPTIONS"].includes(request.method())) {
      pageMutationRequests.push(`${request.method()} ${request.url()}`);
    }
  });
  await page.route(`**${variantPath}`, (route) =>
    route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({ error: { code: "not_found" } }),
    }),
  );

  for (const viewport of viewports) {
    await page.setViewportSize(viewport);
    const requestStart = imageEvents.requests.length;
    const response = await page.goto(`/espacios/${spaceId}`);
    expect(response?.status()).toBe(200);
    const gallery = page.getByRole("region", {
      name: "Galería de imágenes del espacio",
    });
    await expect(gallery.getByText("Imagen no disponible")).toHaveCount(1);
    assertPublicImageEvents(imageEvents, requestStart, [
      { pathname: variantPath, status: 404 },
      { pathname: originalPath, status: 404 },
    ]);
    await expect(
      gallery.getByRole("button", { name: "Ampliar imagen" }),
    ).toHaveCount(0);
    const fallbackA11y = await new AxeBuilder({ page })
      .include('[aria-label="Galería de imágenes del espacio"]')
      .analyze();
    expect(fallbackA11y.violations).toEqual([]);
    await expect(
      gallery.getByRole("button", { name: "Siguiente imagen" }),
    ).toBeVisible();
    await gallery.getByRole("button", { name: "Siguiente imagen" }).click();
    const secondImage = gallery.getByRole("img", {
      name: "Espacio sintético — vista 2",
    });
    await expect(secondImage).toBeVisible();
    await expect
      .poll(() =>
        secondImage.evaluate(
          (image) =>
            image instanceof HTMLImageElement &&
            image.complete &&
            image.naturalWidth > 0,
        ),
      )
      .toBe(true);
    await expect(gallery.getByText("Imagen no disponible")).toHaveCount(0);
    assertPublicImageEvents(imageEvents, requestStart, [
      { pathname: variantPath, status: 404 },
      { pathname: originalPath, status: 404 },
      {
        pathname: publicMediaPath(imageIds[1]),
        status: 200,
        mime: "image/webp",
        cache: "no-store",
      },
    ]);
  }
  expect(pageMutationRequests).toEqual([]);
});

test("public catalog card loads its thumbnail through the public variant route", async ({
  page,
}) => {
  const imageId = "00000000-0000-4000-8000-000000000071";
  const thumbnailPath = `/api/backend/public/spaces/${spaceId}/images/${imageId}/variants/thumbnail`;
  const imageEvents = trackPublicImageEvents(page);
  const writes: string[] = [];
  page.on("request", (request) => {
    if (!["GET", "HEAD", "OPTIONS"].includes(request.method()))
      writes.push(`${request.method()} ${request.url()}`);
  });
  const requestStart = imageEvents.requests.length;

  const pageResponse = await page.goto(
    `/explorar?q=${encodeURIComponent("Espacio sintético")}`,
  );
  expect(pageResponse?.status()).toBe(200);
  const image = page.getByRole("img", { name: "Vista de Espacio sintético" });
  await expect(image).toBeVisible();
  await expect
    .poll(() =>
      image.evaluate(
        (element) =>
          element instanceof HTMLImageElement &&
          element.complete &&
          element.naturalWidth > 0,
      ),
    )
    .toBe(true);
  assertPublicImageEvents(imageEvents, requestStart, [
    {
      pathname: thumbnailPath,
      status: 200,
      mime: "image/webp",
      cache: "no-store",
    },
  ]);
  const card = page
    .getByRole("link", { name: /Espacio sintético/ })
    .filter({ has: image });
  await expect(card).toHaveAttribute("href", `/espacios/${spaceId}`);
  expect(writes).toEqual([]);
});

test("public catalog card recovers from a missing thumbnail through legacy media", async ({
  page,
}) => {
  const imageId = imageIds[0];
  const thumbnailPath = `/api/backend/public/spaces/${spaceId}/images/${imageId}/variants/thumbnail`;
  const originalPath = `/api/backend/public/spaces/${spaceId}/images/${imageId}`;
  const imageEvents = trackPublicImageEvents(page);
  const pageMutationRequests: string[] = [];
  page.on("request", (request) => {
    if (!["GET", "HEAD", "OPTIONS"].includes(request.method())) {
      pageMutationRequests.push(`${request.method()} ${request.url()}`);
    }
  });
  await page.route(`**${thumbnailPath}`, (route) =>
    route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({ error: { code: "not_found" } }),
    }),
  );
  await page.route(`**${originalPath}`, (route) =>
    route.fulfill({
      status: 200,
      contentType: "image/webp",
      body: Buffer.from(
        "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEAAUAmJaQAA3AA/v89WAAAAA==",
        "base64",
      ),
      headers: { "cache-control": "no-store" },
    }),
  );

  for (const viewport of viewports) {
    await page.setViewportSize(viewport);
    const requestStart = imageEvents.requests.length;
    const pageResponse = await page.goto(
      `/explorar?q=${encodeURIComponent("Espacio sintético")}`,
    );
    expect(pageResponse?.status()).toBe(200);
    const image = page.getByRole("img", { name: "Vista de Espacio sintético" });
    await expect(image).toBeVisible();
    await expect
      .poll(() =>
        image.evaluate(
          (element) =>
            element instanceof HTMLImageElement &&
            element.complete &&
            element.naturalWidth > 0,
        ),
      )
      .toBe(true);
    await expect(page.getByText("Imagen no disponible")).toHaveCount(0);
    assertPublicImageEvents(imageEvents, requestStart, [
      { pathname: thumbnailPath, status: 404 },
      {
        pathname: originalPath,
        status: 200,
        mime: "image/webp",
        cache: "no-store",
      },
    ]);
    const card = page
      .getByRole("link", { name: /Espacio sintético/ })
      .filter({ has: image });
    await expect(card).toHaveAttribute("href", `/espacios/${spaceId}`);
    const fallbackA11y = await new AxeBuilder({ page })
      .include(`a[href="/espacios/${spaceId}"]`)
      .analyze();
    expect(fallbackA11y.violations).toEqual([]);
  }
  expect(pageMutationRequests).toEqual([]);
});

test("public catalog card keeps its listing link when thumbnail and legacy media fail", async ({
  page,
}) => {
  const imageId = imageIds[0];
  const thumbnailPath = `/api/backend/public/spaces/${spaceId}/images/${imageId}/variants/thumbnail`;
  const originalPath = `/api/backend/public/spaces/${spaceId}/images/${imageId}`;
  const imageEvents = trackPublicImageEvents(page);
  const pageMutationRequests: string[] = [];
  page.on("request", (request) => {
    if (!["GET", "HEAD", "OPTIONS"].includes(request.method())) {
      pageMutationRequests.push(`${request.method()} ${request.url()}`);
    }
  });
  await page.route(`**${thumbnailPath}`, (route) =>
    route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({ error: { code: "not_found" } }),
    }),
  );

  for (const viewport of viewports) {
    await page.setViewportSize(viewport);
    const requestStart = imageEvents.requests.length;
    const pageResponse = await page.goto(
      `/explorar?q=${encodeURIComponent("Espacio sintético")}`,
    );
    expect(pageResponse?.status()).toBe(200);
    await expect(page.getByText("Imagen no disponible")).toHaveCount(1);
    assertPublicImageEvents(imageEvents, requestStart, [
      { pathname: thumbnailPath, status: 404 },
      { pathname: originalPath, status: 404 },
    ]);
    const card = page.locator(`a[href="/espacios/${spaceId}"]`);
    await expect(card).toHaveCount(1);
    await expect(card.getByText("Espacio sintético")).toBeVisible();
    const fallbackA11y = await new AxeBuilder({ page })
      .include(`a[href="/espacios/${spaceId}"]`)
      .analyze();
    expect(fallbackA11y.violations).toEqual([]);
    await card.click();
    await expect(page).toHaveURL(`/espacios/${spaceId}`);
    await expect(
      page.getByRole("heading", { name: "Espacio sintético" }),
    ).toBeVisible();
  }
  expect(pageMutationRequests).toEqual([]);
});
