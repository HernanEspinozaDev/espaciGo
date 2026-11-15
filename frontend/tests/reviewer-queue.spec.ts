import { expect, test, type Page } from "@playwright/test";

const accountId = "550e8400-e29b-41d4-a716-446655440000";
const identityCaseId = "770e8400-e29b-41d4-a716-446655440000";
const secondIdentityCaseId = "880e8400-e29b-41d4-a716-446655440000";
const identityFileId = "00000000-0000-4000-8000-000000000001";
const spaceCaseId = "660e8400-e29b-41d4-a716-446655440000";
const sessionToken = "synthetic-reviewer-session-token";
const queuePath = "/api/backend/document-verification/reviewer/cases";
const reviewerPngFixture =
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j2ioAAAAASUVORK5CYII=";

test.use({ trace: "off", screenshot: "off", video: "off" });

interface ReviewerQueueFixture {
  readonly items?: readonly unknown[];
  readonly detail?: unknown;
  readonly secondDetail?: unknown;
  readonly queueStatus?: number;
  readonly queueErrorMessage?: string;
  readonly reviewStatus?: number;
  readonly reviewStatuses?: readonly number[];
  readonly reviewResponse?: unknown;
  readonly reviewAbortAfterCommit?: boolean;
  readonly detailStatuses?: readonly number[];
  readonly holdReview?: boolean;
  readonly expiresInMs?: number;
  readonly holdQueue?: boolean;
  readonly holdDownload?: boolean;
}

async function installReviewerQueueMock(
  page: Page,
  fixture: ReviewerQueueFixture = {},
) {
  let loggedIn = false;
  const queueRequests: string[] = [];
  const detailRequests: string[] = [];
  const downloadRequests: string[] = [];
  const reviewRequests: { readonly body: unknown; readonly path: string }[] =
    [];
  let currentDetail = fixture.detail;
  let releaseQueue = () => {};
  let releaseDownload = () => {};
  let releaseReview = () => {};
  let downloadFulfilled = false;
  let reviewFulfilled = false;
  let resolveDownloadSettled = () => {};
  let resolveReviewSettled = () => {};
  const queuePending = fixture.holdQueue
    ? new Promise<void>((resolve) => {
        releaseQueue = resolve;
      })
    : Promise.resolve();
  const downloadPending = fixture.holdDownload
    ? new Promise<void>((resolve) => {
        releaseDownload = resolve;
      })
    : Promise.resolve();
  const reviewPending = fixture.holdReview
    ? new Promise<void>((resolve) => {
        releaseReview = resolve;
      })
    : Promise.resolve();
  const downloadSettled = new Promise<void>((resolve) => {
    resolveDownloadSettled = resolve;
  });
  const reviewSettled = new Promise<void>((resolve) => {
    resolveReviewSettled = resolve;
  });

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    const json = (status: number, body: unknown) =>
      route.fulfill({
        status,
        headers: { "content-type": "application/json; charset=utf-8" },
        body: JSON.stringify(body),
      });

    if (pathname === "/api/backend/auth/session" && method === "GET") {
      return loggedIn
        ? json(200, {
            account_id: accountId,
            roles: ["arrendador"],
            capabilities: [],
            restricted_mode: false,
          })
        : json(401, {
            error: {
              code: "unauthenticated",
              message: "Unauthenticated",
              request_id: "synthetic-reviewer-session",
            },
          });
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      loggedIn = true;
      return json(200, {
        account_id: accountId,
        roles: ["arrendador"],
        capabilities: [],
        access_token: sessionToken,
        token_type: "Bearer",
        expires_at: new Date(
          Date.now() + (fixture.expiresInMs ?? 60_000),
        ).toISOString(),
        restricted_mode: false,
      });
    }
    if (pathname === queuePath && method === "GET") {
      expect(loggedIn).toBe(true);
      expect(request.headers().authorization).toBeUndefined();
      queueRequests.push(`${method} ${pathname}`);
      await queuePending;
      const status = fixture.queueStatus ?? 200;
      if (status !== 200) {
        return json(status, {
          error: {
            code: "synthetic_queue_failure",
            message:
              fixture.queueErrorMessage ?? "Synthetic queue failure detail",
            request_id: "synthetic-reviewer-queue",
          },
        });
      }
      return json(200, { items: fixture.items ?? [] });
    }
    if (
      (pathname === `${queuePath}/${identityCaseId}` ||
        pathname === `${queuePath}/${secondIdentityCaseId}`) &&
      method === "GET"
    ) {
      expect(loggedIn).toBe(true);
      expect(request.headers().authorization).toBeUndefined();
      detailRequests.push(`${method} ${pathname}`);
      const detailStatus =
        fixture.detailStatuses?.[detailRequests.length - 1] ?? 200;
      if (detailStatus !== 200) {
        return json(detailStatus, {
          error: {
            code: "synthetic_detail_failure",
            message: "Synthetic private detail failure",
            request_id: "synthetic-reviewer-detail",
          },
        });
      }
      return json(
        200,
        pathname === `${queuePath}/${identityCaseId}`
          ? currentDetail
          : fixture.secondDetail,
      );
    }
    if (
      pathname === `${queuePath}/${identityCaseId}/review` &&
      method === "POST"
    ) {
      expect(loggedIn).toBe(true);
      expect(request.headers().authorization).toBeUndefined();
      const body = request.postDataJSON() as {
        readonly decision?: string;
        readonly expected_revision?: number;
      };
      reviewRequests.push({ body, path: `${method} ${pathname}` });
      try {
        await reviewPending;
        if (request.failure()) return;
        const stateByDecision: Readonly<Record<string, string>> = {
          approve: "aprobada",
          reject: "rechazada",
          needs_evidence: "requiere_antecedentes",
        };
        const nextDetail =
          fixture.reviewResponse ??
          (fixture.detail && {
            ...fixture.detail,
            state: stateByDecision[body.decision ?? ""] ?? "en_revision",
            revision: (body.expected_revision ?? 0) + 1,
          });
        if (fixture.reviewAbortAfterCommit) {
          currentDetail = nextDetail;
          await route.abort("failed");
          return;
        }
        const status =
          fixture.reviewStatuses?.[reviewRequests.length - 1] ??
          fixture.reviewStatus ??
          200;
        if (status !== 200) {
          if (status === 409) {
            return route.fulfill({
              status,
              contentType: "text/plain; charset=utf-8",
              body: "case revision conflict\n",
            });
          }
          return json(status, {
            error: {
              code: "synthetic_review_failure",
              message: "Synthetic private review failure",
              request_id: "synthetic-review-decision",
            },
          });
        }
        currentDetail = nextDetail;
        await json(200, currentDetail);
        reviewFulfilled = true;
      } catch (error) {
        if (!request.failure()) throw error;
      } finally {
        resolveReviewSettled();
      }
      return;
    }
    if (
      pathname ===
        `${queuePath}/${identityCaseId}/files/${identityFileId}/content` &&
      method === "GET"
    ) {
      expect(loggedIn).toBe(true);
      expect(request.headers().authorization).toBeUndefined();
      downloadRequests.push(`${method} ${pathname}`);
      try {
        await downloadPending;
        await route.fulfill({
          status: 200,
          contentType: "image/png",
          body: Buffer.from(reviewerPngFixture, "base64"),
        });
        downloadFulfilled = true;
      } catch (error) {
        if (!request.failure()) throw error;
      } finally {
        resolveDownloadSettled();
      }
      return;
    }
    return json(404, {
      error: {
        code: "not_found",
        message: "Not found",
        request_id: "synthetic-reviewer-default",
      },
    });
  });

  return {
    queueRequests,
    detailRequests,
    downloadRequests,
    reviewRequests,
    releaseQueue,
    releaseDownload,
    releaseReview,
    downloadSettled,
    reviewSettled,
    get downloadFulfilled() {
      return downloadFulfilled;
    },
    get reviewFulfilled() {
      return reviewFulfilled;
    },
  };
}

async function signInToReviewerQueue(page: Page) {
  await page.goto("/acceso?return_to=%2Fmi-cuenta%2Frevisiones");
  await page.getByLabel("Correo electrónico").fill("reviewer@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/revisiones$/);
}

test("reviewer queue is minimal and clears when the synthetic session expires", async ({
  page,
}) => {
  const fixture = await installReviewerQueueMock(page, {
    expiresInMs: 4_000,
    holdQueue: true,
    items: [
      {
        id: identityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
        document_expires_at_observed: "2030-02-03",
        identity_declaration: {
          first_name: "Synthetic Private Name",
          last_name: "Fixture",
          rut: "12.345.678-5",
          date_of_birth: "1990-01-01",
        },
        files: [{ id: "synthetic-file", storage_key: "private-object-key" }],
      },
      {
        id: spaceCaseId,
        purpose: "space",
        state: "en_revision",
        revision: 4,
        files: [{ id: "space-file", storage_key: "space-object-key" }],
      },
    ],
  });
  await signInToReviewerQueue(page);
  await expect(page.getByRole("status")).toHaveText("Cargando casos…");
  fixture.releaseQueue();

  await expect(
    page.getByRole("heading", { level: 1, name: "Mi Cuenta" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { level: 2, name: "Revisión de identidad" }),
  ).toBeVisible();
  await expect(page.getByText(`Expediente: ${identityCaseId}`)).toBeVisible();
  await expect(page.getByText("Estado: En revisión")).toBeVisible();
  await expect(page.getByText("Revisión: 7")).toBeVisible();
  await expect(
    page.getByText("Vencimiento observado: 2030-02-03"),
  ).toBeVisible();
  await expect(page.getByText(spaceCaseId)).toHaveCount(0);
  await expect(page.getByText("Synthetic Private Name")).toHaveCount(0);
  await expect(page.getByText("12.345.678-5")).toHaveCount(0);
  await expect(page.getByRole("main")).toHaveCount(1);
  await expect(
    page.getByRole("link", { name: "Revisión de identidad" }),
  ).toHaveAttribute("href", "/mi-cuenta/revisiones");
  expect(fixture.queueRequests).toEqual([`GET ${queuePath}`]);
  await expect(page.getByRole("status")).toHaveText("Casos no disponibles.", {
    timeout: 10_000,
  });
  await expect(page.getByText(`Expediente: ${identityCaseId}`)).toHaveCount(0);
});

test("reviewer queue shows an explicit empty state", async ({ page }) => {
  const fixture = await installReviewerQueueMock(page, { items: [] });
  await signInToReviewerQueue(page);

  await expect(page.getByRole("status")).toHaveText("No hay casos asignados.");
  expect(fixture.queueRequests).toEqual([`GET ${queuePath}`]);
});

test("reviewer queue errors are neutral and do not expose API detail", async ({
  page,
}) => {
  const privateMessage = "Synthetic private backend diagnostic";
  const fixture = await installReviewerQueueMock(page, {
    queueStatus: 503,
    queueErrorMessage: privateMessage,
  });
  await signInToReviewerQueue(page);

  await expect(page.getByRole("status")).toHaveText("Casos no disponibles.");
  await expect(page.getByText(privateMessage)).toHaveCount(0);
  expect(fixture.queueRequests).toEqual([`GET ${queuePath}`]);
});

test("assigned identity detail is fetched on demand and shows only allowlisted fields", async ({
  page,
}) => {
  const fixture = await installReviewerQueueMock(page, {
    items: [
      {
        id: identityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
        files: [],
      },
    ],
    detail: {
      id: identityCaseId,
      purpose: "identity",
      state: "en_revision",
      revision: 7,
      document_format_declared: "new",
      document_format_observed: "undetermined",
      document_expires_at_observed: "2030-02-03",
      identity_declaration: {
        first_name: "Synthetic Private Name",
        last_name: "Fixture",
        rut: "12.345.678-5",
        date_of_birth: "1990-01-01",
      },
      files: [
        {
          id: identityFileId,
          evidence_role: "identity_front",
          state: "limpio",
          size: Buffer.from(reviewerPngFixture, "base64").byteLength,
          mime: "image/png",
          pages: 1,
          storage_key: "private-object-key",
        },
        {
          id: "00000000-0000-4000-8000-000000000002",
          evidence_role: "identity_back",
          state: "limpio",
          size: Buffer.from(reviewerPngFixture, "base64").byteLength,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000003",
          evidence_role: "identity_selfie",
          state: "limpio",
          size: Buffer.from(reviewerPngFixture, "base64").byteLength,
          mime: "image/png",
          pages: 1,
        },
      ],
    },
  });
  await signInToReviewerQueue(page);
  await expect(page.getByText(`Expediente: ${identityCaseId}`)).toBeVisible();
  await page.getByRole("link", { name: "Abrir expediente" }).click();

  await expect(page).toHaveURL(`/mi-cuenta/revisiones/${identityCaseId}`);
  await expect(
    page.getByRole("heading", { level: 2, name: "Expediente de identidad" }),
  ).toBeVisible();
  await expect(page.getByText("Synthetic Private Name Fixture")).toBeVisible();
  await expect(page.getByText("12.345.678-5")).toBeVisible();
  await expect(page.getByText("1990-01-01")).toBeVisible();
  await expect(page.getByText("Formato declarado")).toBeVisible();
  await expect(
    page.getByRole("definition").filter({ hasText: "Nuevo" }),
  ).toBeVisible();
  await expect(
    page.getByText("Formato observado", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("definition").filter({ hasText: "Indeterminado" }),
  ).toBeVisible();
  await expect(page.getByText("Vencimiento observado")).toBeVisible();
  await expect(page.getByText("2030-02-03", { exact: true })).toBeVisible();
  await expect(page.getByText("Anverso · image/png · 1 pág.")).toBeVisible();
  await page.getByRole("button", { name: "Ver anverso" }).click();
  const evidence = page.getByRole("img", { name: "Evidencia de anverso" });
  await expect(evidence).toBeVisible();
  await expect(evidence).toHaveJSProperty("naturalWidth", 1);
  await expect(page.getByText("private-object-key")).toHaveCount(0);
  expect(fixture.queueRequests).toEqual([`GET ${queuePath}`]);
  expect(fixture.detailRequests).toEqual([
    `GET ${queuePath}/${identityCaseId}`,
  ]);
  expect(fixture.downloadRequests).toEqual([
    `GET ${queuePath}/${identityCaseId}/files/${identityFileId}/content`,
  ]);
});

test("does not publish a late identity detail after navigating to another case", async ({
  page,
}) => {
  const fixture = await installReviewerQueueMock(page, {
    items: [
      {
        id: identityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
      },
      {
        id: secondIdentityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 3,
      },
    ],
    detail: {
      id: identityCaseId,
      purpose: "identity",
      state: "en_revision",
      revision: 7,
      identity_declaration: {
        first_name: "Synthetic Late",
        last_name: "Case A",
        rut: "12.345.678-5",
        date_of_birth: "1990-01-01",
      },
      files: [
        {
          id: "00000000-0000-4000-8000-000000000001",
          evidence_role: "identity_front",
          state: "limpio",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000002",
          evidence_role: "identity_back",
          state: "limpio",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000003",
          evidence_role: "identity_selfie",
          state: "limpio",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
      ],
    },
    secondDetail: {
      id: secondIdentityCaseId,
      purpose: "identity",
      state: "en_revision",
      revision: 3,
      identity_declaration: {
        first_name: "Synthetic Current",
        last_name: "Case B",
        rut: "12.345.678-5",
        date_of_birth: "1991-02-03",
      },
      files: [
        {
          id: "00000000-0000-4000-8000-000000000004",
          evidence_role: "identity_front",
          state: "limpio",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000005",
          evidence_role: "identity_back",
          state: "limpio",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000006",
          evidence_role: "identity_selfie",
          state: "limpio",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
      ],
    },
  });
  let releaseLateDetail!: () => void;
  let resolveLateDetailStarted!: () => void;
  const lateDetailPending = new Promise<void>((resolve) => {
    releaseLateDetail = resolve;
  });
  const lateDetailStarted = new Promise<void>((resolve) => {
    resolveLateDetailStarted = resolve;
  });
  await page.route(`**${queuePath}/${identityCaseId}`, async (route) => {
    if (route.request().method() === "GET") {
      resolveLateDetailStarted();
      await lateDetailPending;
    }
    await route.fallback();
  });

  await signInToReviewerQueue(page);
  await page.getByRole("link", { name: "Abrir expediente" }).first().click();
  await lateDetailStarted;
  await page.getByRole("link", { name: "Volver a revisiones" }).click();
  await expect(
    page.getByRole("heading", { name: "Revisión de identidad" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Abrir expediente" }).nth(1).click();
  await expect(page.getByText("Synthetic Current Case B")).toBeVisible();

  const lateDetailResponse = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      url.pathname === `${queuePath}/${identityCaseId}` &&
      response.request().method() === "GET" &&
      response.status() === 200
    );
  });
  releaseLateDetail();
  await (await lateDetailResponse).finished();
  await page.evaluate(
    () =>
      new Promise<void>((resolve) => {
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
      }),
  );
  await expect(page.getByText("Synthetic Late Case A")).toHaveCount(0);
  await expect(page.getByText("Synthetic Current Case B")).toBeVisible();
  expect(fixture.detailRequests).toHaveLength(2);
  expect(fixture.detailRequests).toContain(
    `GET ${queuePath}/${identityCaseId}`,
  );
  expect(fixture.detailRequests).toContain(
    `GET ${queuePath}/${secondIdentityCaseId}`,
  );
  expect(fixture.reviewRequests).toHaveLength(0);
});

test("revokes the private evidence preview when navigating to another case", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const trackedWindow = window as typeof window & {
      __revokedBlobUrls?: string[];
    };
    trackedWindow.__revokedBlobUrls = [];
    const revoke = URL.revokeObjectURL.bind(URL);
    URL.revokeObjectURL = (url: string) => {
      trackedWindow.__revokedBlobUrls?.push(url);
      revoke(url);
    };
  });
  const fixture = await installReviewerQueueMock(page, {
    items: [
      {
        id: identityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
      },
      {
        id: secondIdentityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 3,
      },
    ],
    detail: {
      id: identityCaseId,
      purpose: "identity",
      state: "en_revision",
      revision: 7,
      identity_declaration: {
        first_name: "Synthetic Preview",
        last_name: "Case A",
        rut: "12.345.678-5",
        date_of_birth: "1990-01-01",
      },
      files: [
        {
          id: identityFileId,
          evidence_role: "identity_front",
          state: "limpio",
          size: Buffer.from(reviewerPngFixture, "base64").byteLength,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000002",
          evidence_role: "identity_back",
          state: "limpio",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000003",
          evidence_role: "identity_selfie",
          state: "limpio",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
      ],
    },
    secondDetail: {
      id: secondIdentityCaseId,
      purpose: "identity",
      state: "en_revision",
      revision: 3,
      identity_declaration: {
        first_name: "Synthetic Preview",
        last_name: "Case B",
        rut: "12.345.678-5",
        date_of_birth: "1991-02-03",
      },
      files: [
        {
          id: "00000000-0000-4000-8000-000000000004",
          evidence_role: "identity_front",
          state: "limpio",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000005",
          evidence_role: "identity_back",
          state: "limpio",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000006",
          evidence_role: "identity_selfie",
          state: "limpio",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
      ],
    },
  });

  await signInToReviewerQueue(page);
  await page.getByRole("link", { name: "Abrir expediente" }).first().click();
  await expect(page.getByText("Synthetic Preview Case A")).toBeVisible();
  await page.getByRole("button", { name: "Ver anverso" }).click();
  const evidence = page.getByRole("img", { name: "Evidencia de anverso" });
  await expect(evidence).toBeVisible();
  const evidenceUrl = await evidence.getAttribute("src");
  expect(evidenceUrl).toMatch(/^blob:/);

  await page.getByRole("link", { name: "Volver a revisiones" }).click();
  await expect(
    page.getByRole("heading", { name: "Revisión de identidad" }),
  ).toBeVisible();
  await expect
    .poll(() =>
      page.evaluate(
        () =>
          (window as typeof window & { __revokedBlobUrls?: string[] })
            .__revokedBlobUrls ?? [],
      ),
    )
    .toContain(evidenceUrl);
  await page.getByRole("link", { name: "Abrir expediente" }).nth(1).click();
  await expect(page.getByText("Synthetic Preview Case B")).toBeVisible();
  await expect(page.getByText("Synthetic Preview Case A")).toHaveCount(0);
  await expect(
    page.getByRole("img", { name: "Evidencia de anverso" }),
  ).toHaveCount(0);
  expect(fixture.downloadRequests).toEqual([
    `GET ${queuePath}/${identityCaseId}/files/${identityFileId}/content`,
  ]);
  expect(fixture.reviewRequests).toHaveLength(0);
});

test("assigned identity reviewer completes the checklist and records an approval", async ({
  page,
}) => {
  const fixture = await installReviewerQueueMock(page, {
    items: [
      {
        id: identityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
      },
    ],
    detail: {
      id: identityCaseId,
      purpose: "identity",
      state: "en_revision",
      revision: 7,
      identity_declaration: {
        first_name: "Synthetic Decision",
        last_name: "Fixture",
        rut: "12.345.678-5",
        date_of_birth: "1990-01-01",
      },
      files: [
        {
          id: identityFileId,
          evidence_role: "identity_front",
          state: "pendiente",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000002",
          evidence_role: "identity_back",
          state: "pendiente",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000003",
          evidence_role: "identity_selfie",
          state: "pendiente",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
      ],
    },
  });
  await signInToReviewerQueue(page);
  await page.getByRole("link", { name: "Abrir expediente" }).click();
  await expect(page.getByText("Synthetic Decision Fixture")).toBeVisible();

  const decisionForm = page.getByRole("form", {
    name: "Checklist y decisión",
  });
  await decisionForm.getByLabel("Decisión").selectOption("approve");
  await decisionForm
    .getByLabel("Formato observado por revisor")
    .selectOption("new");
  await decisionForm
    .getByLabel("Fecha de vencimiento observada")
    .fill("2030-02-03");
  for (const label of [
    "Nombres coinciden",
    "RUN/RUT coincide",
    "Fecha de nacimiento y mayoría de edad verificadas",
    "Retrato coincide con selfie",
    "Evidencias legibles",
    "Documento vigente",
    "Fecha de emisión y formato verificados",
  ]) {
    await decisionForm.getByLabel(label).selectOption("true");
  }
  await page.getByRole("button", { name: "Registrar decisión" }).click();

  await expect(page.getByText("Estado actualizado: Aprobada")).toBeVisible();
  await expect(page.getByText("Revisión", { exact: true })).toBeVisible();
  await expect(page.getByText("8", { exact: true })).toBeVisible();
  await expect(page.getByLabel("Decisión")).toHaveCount(0);
  expect(fixture.reviewRequests).toEqual([
    {
      path: `POST ${queuePath}/${identityCaseId}/review`,
      body: {
        capability: "review_identity",
        decision: "approve",
        expected_revision: 7,
        document_format_observed: "new",
        document_expires_at_observed: "2030-02-03",
        identity_review_checklist: {
          names_match: true,
          rut_matches: true,
          birth_date_and_majority_match: true,
          portrait_matches_selfie: true,
          evidence_legible: true,
          document_unexpired: true,
          issue_date_and_format_verified: true,
        },
      },
    },
  ]);
  expect(fixture.detailRequests).toEqual([
    `GET ${queuePath}/${identityCaseId}`,
    `GET ${queuePath}/${identityCaseId}`,
  ]);
});

test("keeps a second case isolated from a pending decision on the prior case", async ({
  page,
}) => {
  const detail = (id: string, firstName: string) => ({
    id,
    purpose: "identity",
    state: "en_revision",
    revision: 7,
    identity_declaration: {
      first_name: firstName,
      last_name: "Synthetic Reviewer",
      rut: "12.345.678-5",
      date_of_birth: "1990-01-01",
    },
    files: [
      {
        id: identityFileId,
        evidence_role: "identity_front",
        state: "pendiente",
        size: 1,
        mime: "image/png",
        pages: 1,
      },
      {
        id: "00000000-0000-4000-8000-000000000002",
        evidence_role: "identity_back",
        state: "pendiente",
        size: 1,
        mime: "image/png",
        pages: 1,
      },
      {
        id: "00000000-0000-4000-8000-000000000003",
        evidence_role: "identity_selfie",
        state: "pendiente",
        size: 1,
        mime: "image/png",
        pages: 1,
      },
    ],
  });
  const fixture = await installReviewerQueueMock(page, {
    holdReview: true,
    items: [
      {
        id: identityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
      },
      {
        id: secondIdentityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
      },
    ],
    detail: detail(identityCaseId, "Case A"),
    secondDetail: detail(secondIdentityCaseId, "Case B"),
  });
  await signInToReviewerQueue(page);
  await page.getByRole("link", { name: "Abrir expediente" }).first().click();
  await expect(page.getByText("Case A Synthetic Reviewer")).toBeVisible();

  const priorForm = page.getByRole("form", { name: "Checklist y decisión" });
  await priorForm.getByLabel("Decisión").selectOption("approve");
  await priorForm
    .getByLabel("Formato observado por revisor")
    .selectOption("new");
  await priorForm
    .getByLabel("Fecha de vencimiento observada")
    .fill("2030-02-03");
  for (const label of [
    "Nombres coinciden",
    "RUN/RUT coincide",
    "Fecha de nacimiento y mayoría de edad verificadas",
    "Retrato coincide con selfie",
    "Evidencias legibles",
    "Documento vigente",
    "Fecha de emisión y formato verificados",
  ]) {
    await priorForm.getByLabel(label).selectOption("true");
  }
  await page.getByRole("button", { name: "Registrar decisión" }).click();
  await expect.poll(() => fixture.reviewRequests.length).toBe(1);

  await page.getByRole("link", { name: "Volver a revisiones" }).click();
  await page.getByRole("link", { name: "Abrir expediente" }).nth(1).click();
  await expect(page.getByText("Case B Synthetic Reviewer")).toBeVisible();
  const secondForm = page.getByRole("form", { name: "Checklist y decisión" });
  await expect(secondForm.getByLabel("Decisión")).toHaveValue("needs_evidence");
  await expect(
    secondForm.getByLabel("Fecha de vencimiento observada"),
  ).toHaveValue("");
  await expect(secondForm.getByLabel("Nombres coinciden")).toHaveValue("");

  const lateReviewResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().includes(`${queuePath}/${identityCaseId}/review`),
  );
  fixture.releaseReview();
  const response = await lateReviewResponse;
  await fixture.reviewSettled;
  expect(response.status()).toBe(200);
  await page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 0)));
  await expect(
    page.getByRole("button", { name: "Registrar decisión" }),
  ).toBeVisible();
  await expect(page.getByText("Estado actualizado: Aprobada")).toHaveCount(0);
  await expect(page.getByText("Case B Synthetic Reviewer")).toBeVisible();
  expect(fixture.reviewRequests).toHaveLength(1);
  expect(fixture.reviewRequests[0]?.path).toBe(
    `POST ${queuePath}/${identityCaseId}/review`,
  );
  expect(fixture.detailRequests).toEqual([
    `GET ${queuePath}/${identityCaseId}`,
    `GET ${queuePath}/${secondIdentityCaseId}`,
  ]);
});

test("quarantines an uncertain reviewer decision until explicit readback succeeds", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const trackedWindow = window as typeof window & {
      __revokedBlobUrls?: string[];
    };
    trackedWindow.__revokedBlobUrls = [];
    const revoke = URL.revokeObjectURL.bind(URL);
    URL.revokeObjectURL = (url: string) => {
      trackedWindow.__revokedBlobUrls?.push(url);
      revoke(url);
    };
  });
  const imageSize = Buffer.from(reviewerPngFixture, "base64").byteLength;
  const fixture = await installReviewerQueueMock(page, {
    reviewAbortAfterCommit: true,
    detailStatuses: [200, 503, 200],
    items: [
      {
        id: identityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
      },
    ],
    detail: {
      id: identityCaseId,
      purpose: "identity",
      state: "en_revision",
      revision: 7,
      identity_declaration: {
        first_name: "Synthetic Uncertain",
        last_name: "Reviewer",
        rut: "12.345.678-5",
        date_of_birth: "1990-01-01",
      },
      files: [
        {
          id: identityFileId,
          evidence_role: "identity_front",
          state: "limpio",
          size: imageSize,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000002",
          evidence_role: "identity_back",
          state: "limpio",
          size: imageSize,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000003",
          evidence_role: "identity_selfie",
          state: "limpio",
          size: imageSize,
          mime: "image/png",
          pages: 1,
        },
      ],
    },
  });
  await signInToReviewerQueue(page);
  await page.getByRole("link", { name: "Abrir expediente" }).click();
  await expect(page.getByText("Synthetic Uncertain Reviewer")).toBeVisible();
  await page.getByRole("button", { name: "Ver anverso" }).click();
  await expect(
    page.getByRole("img", { name: "Evidencia de anverso" }),
  ).toBeVisible();

  const form = page.getByRole("form", { name: "Checklist y decisión" });
  await form.getByLabel("Decisión").selectOption("approve");
  await form.getByLabel("Formato observado por revisor").selectOption("new");
  await form.getByLabel("Fecha de vencimiento observada").fill("2030-02-03");
  for (const label of [
    "Nombres coinciden",
    "RUN/RUT coincide",
    "Fecha de nacimiento y mayoría de edad verificadas",
    "Retrato coincide con selfie",
    "Evidencias legibles",
    "Documento vigente",
    "Fecha de emisión y formato verificados",
  ]) {
    await form.getByLabel(label).selectOption("true");
  }
  await page.getByRole("button", { name: "Registrar decisión" }).click();
  await expect.poll(() => fixture.reviewRequests.length).toBe(1);
  await fixture.reviewSettled;

  await expect(
    page.getByRole("heading", { name: "Resultado no confirmado" }),
  ).toBeVisible();
  await expect(page.getByText("Synthetic Uncertain Reviewer")).toHaveCount(0);
  await expect(form).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Ver anverso" })).toHaveCount(
    0,
  );
  await expect
    .poll(() =>
      page.evaluate(
        () =>
          (window as typeof window & { __revokedBlobUrls?: string[] })
            .__revokedBlobUrls?.length ?? 0,
      ),
    )
    .toBeGreaterThan(0);
  expect(fixture.detailRequests).toEqual([
    `GET ${queuePath}/${identityCaseId}`,
  ]);

  await page.getByRole("button", { name: "Actualizar expediente" }).click();
  await expect(
    page.getByText(
      "No se pudo actualizar el expediente. Vuelve a intentarlo.",
      {
        exact: true,
      },
    ),
  ).toBeVisible();
  await expect(page.getByText("Synthetic Uncertain Reviewer")).toHaveCount(0);
  expect(fixture.detailRequests).toEqual([
    `GET ${queuePath}/${identityCaseId}`,
    `GET ${queuePath}/${identityCaseId}`,
  ]);

  await page.getByRole("button", { name: "Actualizar expediente" }).click();
  await expect(
    page.getByRole("definition").filter({ hasText: "Aprobada" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Resultado no confirmado" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("form", { name: "Checklist y decisión" }),
  ).toHaveCount(0);
  expect(fixture.reviewRequests).toHaveLength(1);
  expect(fixture.detailRequests).toEqual([
    `GET ${queuePath}/${identityCaseId}`,
    `GET ${queuePath}/${identityCaseId}`,
    `GET ${queuePath}/${identityCaseId}`,
  ]);
});

test("requires readback and a deliberate new submit after a review conflict", async ({
  page,
}) => {
  const fixture = await installReviewerQueueMock(page, {
    reviewStatuses: [409, 200],
    items: [
      {
        id: identityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
      },
    ],
    detail: {
      id: identityCaseId,
      purpose: "identity",
      state: "en_revision",
      revision: 7,
      identity_declaration: {
        first_name: "Synthetic Conflict",
        last_name: "Reviewer",
        rut: "12.345.678-5",
        date_of_birth: "1990-01-01",
      },
      files: [
        {
          id: identityFileId,
          evidence_role: "identity_front",
          state: "pendiente",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000002",
          evidence_role: "identity_back",
          state: "pendiente",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000003",
          evidence_role: "identity_selfie",
          state: "pendiente",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
      ],
    },
  });
  await signInToReviewerQueue(page);
  await page.getByRole("link", { name: "Abrir expediente" }).click();
  await expect(page.getByText("Synthetic Conflict Reviewer")).toBeVisible();

  const labels = [
    "Nombres coinciden",
    "RUN/RUT coincide",
    "Fecha de nacimiento y mayoría de edad verificadas",
    "Retrato coincide con selfie",
    "Evidencias legibles",
    "Documento vigente",
    "Fecha de emisión y formato verificados",
  ];
  const fillChecklist = async () => {
    const form = page.getByRole("form", { name: "Checklist y decisión" });
    for (const label of labels) {
      await form.getByLabel(label).selectOption("true");
    }
  };

  await fillChecklist();
  const conflictResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().includes(`${queuePath}/${identityCaseId}/review`),
  );
  await page.getByRole("button", { name: "Registrar decisión" }).click();
  const conflict = await conflictResponse;
  expect(conflict.status()).toBe(409);
  expect(await conflict.text()).toBe("case revision conflict\n");
  await expect.poll(() => fixture.reviewRequests.length).toBe(1);
  await expect(
    page.getByRole("heading", { name: "Resultado no confirmado" }),
  ).toBeVisible();
  await expect(
    page.getByRole("form", { name: "Checklist y decisión" }),
  ).toHaveCount(0);
  expect(fixture.reviewRequests).toHaveLength(1);

  await page.getByRole("button", { name: "Actualizar expediente" }).click();
  await expect(page.getByText("Synthetic Conflict Reviewer")).toBeVisible();
  const refreshedForm = page.getByRole("form", {
    name: "Checklist y decisión",
  });
  await expect(refreshedForm.getByLabel("Decisión")).toHaveValue(
    "needs_evidence",
  );
  await expect(refreshedForm.getByLabel("Nombres coinciden")).toHaveValue("");
  await expect(
    refreshedForm.getByLabel("Fecha de vencimiento observada"),
  ).toHaveValue("");
  expect(fixture.reviewRequests).toHaveLength(1);

  await fillChecklist();
  await page.getByRole("button", { name: "Registrar decisión" }).click();
  await expect(
    page.getByText("Estado actualizado: Antecedentes solicitados"),
  ).toBeVisible();
  await expect.poll(() => fixture.reviewRequests.length).toBe(2);
  await expect.poll(() => fixture.detailRequests.length).toBe(3);
  expect(fixture.reviewRequests.map((request) => request.path)).toEqual([
    `POST ${queuePath}/${identityCaseId}/review`,
    `POST ${queuePath}/${identityCaseId}/review`,
  ]);
  expect(
    (fixture.reviewRequests[1]?.body as { expected_revision?: number })
      .expected_revision,
  ).toBe(7);
});

test("reviewer requires a past Santiago civil date before rejecting an expired document", async ({
  page,
}) => {
  const fixture = await installReviewerQueueMock(page, {
    items: [
      {
        id: identityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
      },
    ],
    detail: {
      id: identityCaseId,
      purpose: "identity",
      state: "en_revision",
      revision: 7,
      identity_declaration: {
        first_name: "Synthetic Expired",
        last_name: "Fixture",
        rut: "12.345.678-5",
        date_of_birth: "1990-01-01",
      },
      files: [
        {
          id: identityFileId,
          evidence_role: "identity_front",
          state: "pendiente",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000002",
          evidence_role: "identity_back",
          state: "pendiente",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000003",
          evidence_role: "identity_selfie",
          state: "pendiente",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
      ],
    },
  });
  await signInToReviewerQueue(page);
  await page.getByRole("link", { name: "Abrir expediente" }).click();

  const form = page.getByRole("form", { name: "Checklist y decisión" });
  await form.getByLabel("Decisión").selectOption("reject");
  await form.getByLabel("Motivo").selectOption("documento_vencido");
  for (const label of [
    "Nombres coinciden",
    "RUN/RUT coincide",
    "Fecha de nacimiento y mayoría de edad verificadas",
    "Retrato coincide con selfie",
    "Evidencias legibles",
    "Documento vigente",
    "Fecha de emisión y formato verificados",
  ]) {
    await form
      .getByLabel(label)
      .selectOption(label === "Documento vigente" ? "false" : "true");
  }
  const santiagoToday = await page.evaluate(() => {
    const parts = new Intl.DateTimeFormat("en-CA", {
      timeZone: "America/Santiago",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).formatToParts(new Date());
    const part = (type: string) =>
      parts.find((item) => item.type === type)?.value;
    return `${part("year")}-${part("month")}-${part("day")}`;
  });
  const priorCivilDate = new Date(`${santiagoToday}T00:00:00Z`);
  priorCivilDate.setUTCDate(priorCivilDate.getUTCDate() - 1);
  const priorDate = priorCivilDate.toISOString().slice(0, 10);
  const expiryInput = form.getByLabel("Fecha de vencimiento observada");
  await expiryInput.fill(santiagoToday);
  await page.getByRole("button", { name: "Registrar decisión" }).click();

  await expect(
    form.getByText(
      "No se pudo validar la decisión. Revisa la fecha y los datos ingresados.",
      { exact: true },
    ),
  ).toBeVisible();
  expect(fixture.reviewRequests).toEqual([]);

  await expiryInput.fill(priorDate);
  await page.getByRole("button", { name: "Registrar decisión" }).click();
  await expect(page.getByText("Estado actualizado: Rechazada")).toBeVisible();
  expect(fixture.reviewRequests).toEqual([
    {
      path: `POST ${queuePath}/${identityCaseId}/review`,
      body: {
        capability: "review_identity",
        decision: "reject",
        expected_revision: 7,
        reason_code: "documento_vencido",
        document_format_observed: "undetermined",
        document_expires_at_observed: priorDate,
        identity_review_checklist: {
          names_match: true,
          rut_matches: true,
          birth_date_and_majority_match: true,
          portrait_matches_selfie: true,
          evidence_legible: true,
          document_unexpired: false,
          issue_date_and_format_verified: true,
        },
      },
    },
  ]);
});

test("does not replay or publish a pending reviewer decision after session expiry", async ({
  page,
}) => {
  const fixture = await installReviewerQueueMock(page, {
    expiresInMs: 7_000,
    holdReview: true,
    items: [
      {
        id: identityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
      },
    ],
    detail: {
      id: identityCaseId,
      purpose: "identity",
      state: "en_revision",
      revision: 7,
      identity_declaration: {
        first_name: "Synthetic Pending",
        last_name: "Decision",
        rut: "12.345.678-5",
        date_of_birth: "1990-01-01",
      },
      files: [
        {
          id: identityFileId,
          evidence_role: "identity_front",
          state: "pendiente",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000002",
          evidence_role: "identity_back",
          state: "pendiente",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000003",
          evidence_role: "identity_selfie",
          state: "pendiente",
          size: 1,
          mime: "image/png",
          pages: 1,
        },
      ],
    },
  });
  await signInToReviewerQueue(page);
  await page.getByRole("link", { name: "Abrir expediente" }).click();
  await expect(page.getByText("Synthetic Pending Decision")).toBeVisible();

  const form = page.getByRole("form", { name: "Checklist y decisión" });
  await form.getByLabel("Decisión").selectOption("approve");
  await form.getByLabel("Formato observado por revisor").selectOption("new");
  await form.getByLabel("Fecha de vencimiento observada").fill("2099-01-01");
  for (const label of [
    "Nombres coinciden",
    "RUN/RUT coincide",
    "Fecha de nacimiento y mayoría de edad verificadas",
    "Retrato coincide con selfie",
    "Evidencias legibles",
    "Documento vigente",
    "Fecha de emisión y formato verificados",
  ]) {
    await form.getByLabel(label).selectOption("true");
  }
  await page.getByRole("button", { name: "Registrar decisión" }).click();
  await expect.poll(() => fixture.reviewRequests.length).toBe(1);
  await expect(
    page.getByText("Expediente no disponible.", { exact: true }),
  ).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText("Synthetic Pending Decision")).toHaveCount(0);
  await expect(form).toHaveCount(0);

  fixture.releaseReview();
  await fixture.reviewSettled;
  expect(fixture.reviewFulfilled).toBe(false);
  expect(fixture.reviewRequests).toHaveLength(1);
  expect(fixture.detailRequests).toEqual([
    `GET ${queuePath}/${identityCaseId}`,
  ]);
});

test("reviewer detail clears declaration data when the synthetic session expires", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const trackedWindow = window as typeof window & {
      __revokedBlobUrls?: string[];
    };
    trackedWindow.__revokedBlobUrls = [];
    const revoke = URL.revokeObjectURL.bind(URL);
    URL.revokeObjectURL = (url: string) => {
      trackedWindow.__revokedBlobUrls?.push(url);
      revoke(url);
    };
  });
  const fixture = await installReviewerQueueMock(page, {
    expiresInMs: 5_000,
    items: [
      {
        id: identityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
        files: [],
      },
    ],
    detail: {
      id: identityCaseId,
      purpose: "identity",
      state: "en_revision",
      revision: 7,
      identity_declaration: {
        first_name: "Synthetic Expiry",
        last_name: "Person",
        rut: "12.345.678-5",
        date_of_birth: "1990-01-01",
      },
      files: [
        {
          id: identityFileId,
          evidence_role: "identity_front",
          state: "limpio",
          size: Buffer.from(reviewerPngFixture, "base64").byteLength,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000002",
          evidence_role: "identity_back",
          state: "limpio",
          size: Buffer.from(reviewerPngFixture, "base64").byteLength,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000003",
          evidence_role: "identity_selfie",
          state: "limpio",
          size: Buffer.from(reviewerPngFixture, "base64").byteLength,
          mime: "image/png",
          pages: 1,
        },
      ],
    },
  });
  await signInToReviewerQueue(page);
  await page.getByRole("link", { name: "Abrir expediente" }).click();

  await expect(page.getByText("Synthetic Expiry Person")).toBeVisible();
  const evidence = page.getByRole("img", { name: "Evidencia de anverso" });
  await page.getByRole("button", { name: "Ver anverso" }).click();
  await expect(evidence).toBeVisible();
  const evidenceUrl = await evidence.getAttribute("src");
  expect(evidenceUrl).toMatch(/^blob:/);
  await expect(
    page.getByText("Expediente no disponible.", { exact: true }),
  ).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText("Synthetic Expiry Person")).toHaveCount(0);
  await expect(page.getByText("12.345.678-5")).toHaveCount(0);
  await expect(evidence).toHaveCount(0);
  await expect
    .poll(() =>
      page.evaluate(
        () =>
          (window as typeof window & { __revokedBlobUrls?: string[] })
            .__revokedBlobUrls ?? [],
      ),
    )
    .toContain(evidenceUrl);
  expect(fixture.detailRequests).toEqual([
    `GET ${queuePath}/${identityCaseId}`,
  ]);
});

test("does not create a blob URL when the reviewer session expires during download", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const browserFetch = window.fetch.bind(window);
    window.fetch = (input: RequestInfo | URL, init?: RequestInit) => {
      const requestUrl = input instanceof Request ? input.url : String(input);
      if (
        new URL(requestUrl, window.location.href).pathname.endsWith("/content")
      ) {
        return browserFetch(input, {
          ...init,
          signal: new AbortController().signal,
        });
      }
      return browserFetch(input, init);
    };
    const trackedWindow = window as typeof window & {
      __createdBlobUrls?: string[];
    };
    trackedWindow.__createdBlobUrls = [];
    const create = URL.createObjectURL.bind(URL);
    URL.createObjectURL = (object: Blob | MediaSource) => {
      const url = create(object);
      trackedWindow.__createdBlobUrls?.push(url);
      return url;
    };
  });
  const fixture = await installReviewerQueueMock(page, {
    expiresInMs: 5_000,
    holdDownload: true,
    items: [
      {
        id: identityCaseId,
        purpose: "identity",
        state: "en_revision",
        revision: 7,
        files: [],
      },
    ],
    detail: {
      id: identityCaseId,
      purpose: "identity",
      state: "en_revision",
      revision: 7,
      identity_declaration: {
        first_name: "Synthetic Pending Download",
        last_name: "Person",
        rut: "12.345.678-5",
        date_of_birth: "1990-01-01",
      },
      files: [
        {
          id: identityFileId,
          evidence_role: "identity_front",
          state: "limpio",
          size: Buffer.from(reviewerPngFixture, "base64").byteLength,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000002",
          evidence_role: "identity_back",
          state: "limpio",
          size: Buffer.from(reviewerPngFixture, "base64").byteLength,
          mime: "image/png",
          pages: 1,
        },
        {
          id: "00000000-0000-4000-8000-000000000003",
          evidence_role: "identity_selfie",
          state: "limpio",
          size: Buffer.from(reviewerPngFixture, "base64").byteLength,
          mime: "image/png",
          pages: 1,
        },
      ],
    },
  });
  await signInToReviewerQueue(page);
  await page.getByRole("link", { name: "Abrir expediente" }).click();
  await expect(
    page.getByText("Synthetic Pending Download Person"),
  ).toBeVisible();
  await page.getByRole("button", { name: "Ver anverso" }).click();
  await expect.poll(() => fixture.downloadRequests.length).toBe(1);

  await expect(
    page.getByText("Expediente no disponible.", { exact: true }),
  ).toBeVisible({ timeout: 10_000 });
  fixture.releaseDownload();
  await fixture.downloadSettled;
  expect(fixture.downloadFulfilled).toBe(true);
  await expect(
    page.getByRole("img", { name: "Evidencia de anverso" }),
  ).toHaveCount(0);
  expect(
    await page.evaluate(
      () =>
        (window as typeof window & { __createdBlobUrls?: string[] })
          .__createdBlobUrls ?? [],
    ),
  ).toEqual([]);
});
