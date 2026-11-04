import { expect, test, type Page } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { readFile } from "node:fs/promises";
import path from "node:path";

test.use({ trace: "off", screenshot: "off", video: "off" });

const workerFree = process.env.PHASE6_WORKER_FREE_E2E === "1";
const scanOnly = process.env.PHASE6_DOCUMENT_SCAN_E2E === "1";
const privateTestingDir = process.env.PHASE6_PRIVATE_TESTING_DIR;
const accountInventoryFile = process.env.PHASE6_ACCOUNT_INVENTORY_FILE;
const fixtureSuffix = process.env.PHASE6_SPACE_FIXTURE_SUFFIX;
if (fixtureSuffix && !/^\d{8}-[a-z0-9]{1,8}$/.test(fixtureSuffix)) {
  throw new Error("Synthetic space fixture selector is invalid");
}
const fixtureTitle = `F6 local synthetic document fixture${fixtureSuffix ? ` ${fixtureSuffix}` : ""}`;

function selectAccountInventoryFile(
  privateDir: string | undefined,
  selectedFile: string | undefined,
): string {
  if (!privateDir || !path.isAbsolute(privateDir)) {
    throw new Error("Authorized private test-data directory is unavailable");
  }
  const canonicalPrivateDir = path.resolve(privateDir);
  const candidate = selectedFile
    ? path.resolve(selectedFile)
    : path.resolve(
        canonicalPrivateDir,
        "f5-gate-20261101-f5a3/f5-a3-accounts-20261103T102133Z.json",
      );
  if (!path.isAbsolute(candidate)) {
    throw new Error("Authorized private account inventory path is invalid");
  }
  const relative = path.relative(canonicalPrivateDir, candidate);
  if (
    (selectedFile !== undefined &&
      (!path.isAbsolute(selectedFile) ||
        path.resolve(selectedFile) !== selectedFile)) ||
    relative === "" ||
    relative === ".." ||
    relative.startsWith(`..${path.sep}`) ||
    path.isAbsolute(relative)
  ) {
    throw new Error("Authorized private account inventory path is invalid");
  }
  return candidate;
}

test("@f6 @native @fixture selects only canonical inventory paths in private dir", () => {
  const privateDir = "/fixture/private-testing";
  expect(
    selectAccountInventoryFile(privateDir, "/fixture/private-testing/new.json"),
  ).toBe("/fixture/private-testing/new.json");
  expect(selectAccountInventoryFile(privateDir, undefined)).toBe(
    "/fixture/private-testing/f5-gate-20261101-f5a3/f5-a3-accounts-20261103T102133Z.json",
  );
  for (const invalid of [
    "relative.json",
    "/fixture/private-testing/../outside.json",
    "/fixture/outside.json",
    "/fixture/private-testing/./new.json",
  ]) {
    expect(() => selectAccountInventoryFile(privateDir, invalid)).toThrow(
      "Authorized private account inventory path is invalid",
    );
  }
});

const privateFile = (name: string) => {
  if (!privateTestingDir || !path.isAbsolute(privateTestingDir)) {
    throw new Error("Authorized private test-data directory is unavailable");
  }
  return path.join(privateTestingDir, name);
};

type Account = {
  slot: string;
  email: string;
  password: string;
  account_id: string;
  registration_http: number;
  verification_http: number;
};
type Inventory = { target: Record<string, unknown>; accounts: Account[] };
type Policy = {
  database: { host: string; port: number; name: string };
  account_policy: { never_print_credentials_or_tokens: boolean };
};

async function privateAccounts() {
  let policy: Policy;
  let inventory: Inventory;
  try {
    [policy, inventory] = await Promise.all([
      readFile(privateFile("test-account-policy.json"), "utf8").then(
        (s) => JSON.parse(s) as Policy,
      ),
      readFile(
        selectAccountInventoryFile(privateTestingDir, accountInventoryFile),
        "utf8",
      ).then((s) => JSON.parse(s) as Inventory),
    ]);
  } catch {
    throw new Error(
      "Authorized private synthetic-account materials are unavailable",
    );
  }
  expect(policy.account_policy.never_print_credentials_or_tokens).toBe(true);
  expect(policy.database).toMatchObject({
    host: "127.0.0.1",
    port: 15434,
    name: "espacigo_f5_gate_20261101",
  });
  expect(inventory.target).toMatchObject({
    host: "127.0.0.1",
    port: 15434,
    database: "espacigo_f5_gate_20261101",
  });
  const owner = inventory.accounts.find((a) => a.slot === "owner");
  const second = inventory.accounts.find((a) => a.slot !== "owner");
  for (const account of [owner, second]) {
    expect(account?.email).toBeTruthy();
    expect(account?.password).toBeTruthy();
    expect(account?.account_id).toBeTruthy();
    expect(account?.registration_http).toBe(201);
    expect(account?.verification_http).toBe(204);
  }
  return { owner: owner!, second: second! };
}

async function login(
  page: Page,
  account: Account,
  returnTo = "/mi-cuenta/documentos",
) {
  await page.goto(`/acceso?return_to=${encodeURIComponent(returnTo)}`);
  const email = page.getByLabel("Correo electrónico");
  const password = page.getByLabel("Contraseña", { exact: true });
  const loginResponsePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/backend/auth/login",
    { timeout: 15_000 },
  );
  try {
    await email.fill(account.email);
    await password.fill(account.password);
    const [, loginResponse] = await Promise.all([
      page
        .getByRole("button", { name: "Iniciar sesión" })
        .click({ noWaitAfter: true }),
      loginResponsePromise,
    ]);
    expect(loginResponse.status()).toBe(200);
    await expect(page).toHaveURL((url) => url.pathname === returnTo);
  } catch {
    await email.fill("", { timeout: 500 }).catch(() => undefined);
    await password.fill("", { timeout: 500 }).catch(() => undefined);
    throw new Error(
      "Authorized synthetic login did not reach its local destination",
    );
  }
}

async function logout(page: Page) {
  if (new URL(page.url()).pathname !== "/acceso") {
    const accessLink = page.getByRole("link", { name: "Acceso" });
    if (await accessLink.count()) await accessLink.first().click();
  }
  await expect(page).toHaveURL(/\/acceso$/);
  await expect(page.getByText("Sesión activa")).toBeVisible();
  const logoutResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/backend/auth/logout",
    { timeout: 15_000 },
  );
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  expect((await logoutResponse).status()).toBe(204);
  await page
    .getByLabel("Correo electrónico")
    .fill("", { timeout: 500 })
    .catch(() => undefined);
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("", { timeout: 500 })
    .catch(() => undefined);
}

function createOnePagePdf(): Buffer {
  const content = "BT /F1 12 Tf 20 80 Td (Synthetic F6 document) Tj ET";
  const objects = [
    "<< /Type /Catalog /Pages 2 0 R >>",
    "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
    "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 144] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
    `<< /Length ${Buffer.byteLength(content, "ascii")} >>\nstream\n${content}\nendstream`,
    "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
  ];
  let pdf = "%PDF-1.4\n";
  const offsets = [0];
  for (const [index, object] of objects.entries()) {
    offsets.push(Buffer.byteLength(pdf, "ascii"));
    pdf += `${index + 1} 0 obj\n${object}\nendobj\n`;
  }
  const xrefOffset = Buffer.byteLength(pdf, "ascii");
  pdf += `xref\n0 ${objects.length + 1}\n0000000000 65535 f \n`;
  for (const offset of offsets.slice(1)) {
    pdf += `${String(offset).padStart(10, "0")} 00000 n \n`;
  }
  pdf += `trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xrefOffset}\n%%EOF\n`;
  return Buffer.from(pdf, "ascii");
}

const onePagePdf = createOnePagePdf();

test("@f6 @native @pdf-fixture one-page PDF is valid through pdfinfo stdin", () => {
  const result = execFileSync("/usr/bin/pdfinfo", ["-"], {
    input: onePagePdf,
    encoding: "utf8",
    timeout: 5_000,
  });
  expect(result).toMatch(/Pages:\s+1\b/);
});

test("@f6 @native @preflight authenticated account reads its own spaces and document cases", async ({
  page,
}) => {
  test.skip(
    !scanOnly,
    "Requires the explicitly selected local-document-scan-only profile",
  );
  const { owner } = await privateAccounts();
  let mayHaveSession = false;
  const observed = new Map<
    string,
    { status?: number; bearer: boolean; cookie: boolean }
  >();
  page.on("request", (request) => {
    const pathname = new URL(request.url()).pathname;
    if (
      pathname !== "/api/backend/spaces" &&
      pathname !== "/api/backend/document-verification/cases"
    )
      return;
    const headers = request.headers();
    observed.set(pathname, {
      bearer: headers.authorization?.startsWith("Bearer ") ?? false,
      cookie: "cookie" in headers,
    });
  });
  page.on("response", (response) => {
    const pathname = new URL(response.url()).pathname;
    const entry = observed.get(pathname);
    if (entry) observed.set(pathname, { ...entry, status: response.status() });
  });
  try {
    mayHaveSession = true;
    await login(page, owner);
    await page.getByLabel("Tipo de expediente").selectOption("space");
    const optionValues = await page
      .getByLabel("Espacio propio")
      .locator("option")
      .evaluateAll((options) =>
        options
          .map((option) => (option as HTMLOptionElement).value)
          .filter(Boolean),
      );
    for (const pathname of [
      "/api/backend/spaces",
      "/api/backend/document-verification/cases",
    ]) {
      const request = observed.get(pathname);
      expect(request?.status).toBe(200);
      expect(request?.bearer).toBe(true);
      expect(request?.cookie).toBe(false);
    }
    console.log(`native_f6_preflight own_spaces=${optionValues.length}`);
    await page.getByRole("link", { name: "Acceso" }).click();
    await expect(page).toHaveURL(/\/acceso$/);
    await expect(page.getByText("Sesión activa")).toBeVisible();
    const logoutResponse = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/auth/logout",
    );
    await page.getByRole("button", { name: "Cerrar sesión" }).click();
    expect((await logoutResponse).status()).toBe(204);
    mayHaveSession = false;
  } finally {
    if (mayHaveSession) {
      await page
        .getByRole("link", { name: "Acceso" })
        .click({ timeout: 500 })
        .catch(() => undefined);
      await page
        .getByRole("button", { name: "Cerrar sesión" })
        .click({ timeout: 500 })
        .catch(() => undefined);
    }
  }
});

test("@f5 @native @local-http-worker-free owner can create a draft through the real BFF", async ({
  page,
}) => {
  test.skip(
    !workerFree,
    "Requires the explicitly selected local-http-worker-free profile",
  );
  test.setTimeout(120_000);
  const { owner } = await privateAccounts();
  const title = fixtureTitle;
  const secrets = [owner.email, owner.password];
  let mayHaveSession = false;
  let nonLoopback = false;
  let secretInConsole = false;
  let spacesReadStatus: number | undefined;
  let categoriesReadStatus: number | undefined;
  let createdStatus: number | undefined;
  let createdNow = false;
  let createAuthorizationValid = false;
  let createSentCookie = false;
  let createSentUnsupportedIdempotencyKey = false;
  page.on("request", (request) => {
    if (!request.url().startsWith("http://127.0.0.1:")) nonLoopback = true;
    const pathname = new URL(request.url()).pathname;
    if (request.method() !== "POST" || pathname !== "/api/backend/spaces")
      return;
    const headers = request.headers();
    createAuthorizationValid =
      headers.authorization?.startsWith("Bearer ") ?? false;
    createSentCookie = "cookie" in headers;
    createSentUnsupportedIdempotencyKey = "idempotency-key" in headers;
  });
  page.on("response", (response) => {
    const pathname = new URL(response.url()).pathname;
    if (
      pathname === "/api/backend/spaces" &&
      response.request().method() === "GET"
    ) {
      spacesReadStatus = response.status();
    }
    if (pathname === "/api/backend/spaces/categories")
      categoriesReadStatus = response.status();
    if (
      pathname === "/api/backend/spaces" &&
      response.request().method() === "POST"
    ) {
      createdStatus = response.status();
    }
  });
  page.on("console", (message) => {
    const text = message.text();
    if (
      secrets.some((secret) => text.includes(secret)) ||
      /\b(?:bearer\s+\S+|access[_ -]?token|authorization)\b/i.test(text)
    ) {
      secretInConsole = true;
    }
  });

  const spacesResponsePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/backend/spaces" &&
      response.request().method() === "GET",
  );
  try {
    mayHaveSession = true;
    await login(page, owner, "/anfitrion/espacios");
    await expect(page.locator("h1")).toHaveText("Mis Espacios");
    const spacesResponse = await spacesResponsePromise;
    expect(spacesResponse.status()).toBe(200);
    const spaces = (await spacesResponse.json()) as {
      items: Array<{ id: string; title: string }>;
    };
    let fixture = spaces.items.find((space) => space.title === title);
    if (!fixture) {
      createdNow = true;
      await page
        .getByRole("link", { name: "Publicar un espacio" })
        .first()
        .click();
      await expect(page).toHaveURL(/\/anfitrion\/espacios\/nuevo$/);
      await expect(page.locator("h1")).toHaveText("Publicar un nuevo espacio");
      await expect(page.locator("#wizard-category")).toBeVisible();
      await expect(
        page.locator('#wizard-category option[value="oficina"]'),
      ).toHaveCount(1, { timeout: 15_000 });
      await page.locator("#wizard-category").selectOption("oficina");
      await page.locator("#wizard-title").fill(title);
      await page
        .locator("#wizard-description")
        .fill(
          "Borrador sintético local para verificar documentos con datos recreables y sin información personal. Espacio ficticio de ensayo: no publicar ni ofrecer a personas reales.",
        );
      await page.getByRole("button", { name: "Siguiente" }).click();
      await expect(
        page.getByText("Paso 2: Ubicación y características"),
      ).toBeVisible();
      await page
        .locator("#wizard-address")
        .fill("Dirección ficticia de ensayo F6");
      await page.locator("#wizard-comuna").fill("13101");
      await page.locator("#wizard-region").selectOption("13");
      await page.locator("#wizard-area").fill("35");
      await page.locator("#wizard-capacity").fill("2");
      await page.getByRole("checkbox", { name: "Wi-Fi", exact: true }).check();
      await page.getByRole("button", { name: "Siguiente" }).click();
      await expect(
        page.getByText("Paso 3: Tarifas iniciales y condiciones"),
      ).toBeVisible();
      await page.locator("#wizard-rate-unit").selectOption("hora");
      await page.locator("#wizard-price").fill("6000");
      await page.getByRole("button", { name: "Siguiente" }).click();
      await expect(
        page.getByText("Paso 4: Disponibilidad semanal y zona horaria"),
      ).toBeVisible();
      await page.getByRole("button", { name: "Siguiente" }).click();
      await expect(
        page.getByText("Paso 5: Resumen y confirmación de alta"),
      ).toBeVisible();
      await expect(page.getByText(title, { exact: true })).toBeVisible();
      const createResponsePromise = page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname === "/api/backend/spaces" &&
          response.request().method() === "POST",
        { timeout: 30_000 },
      );
      await page
        .getByRole("button", { name: "Crear espacio y continuar a edición" })
        .click();
      const createResponse = await createResponsePromise;
      createdStatus = createResponse.status();
      if (createdStatus !== 201) {
        const rejected = (await createResponse.json()) as {
          error?: {
            code?: string;
            details?: Array<{ field?: string; code?: string }>;
          };
        };
        const code = rejected.error?.code;
        const fields = (rejected.error?.details ?? [])
          .filter(
            (item) =>
              /^[a-z_.]+$/.test(item.field ?? "") &&
              /^[a-z_]+$/.test(item.code ?? ""),
          )
          .map((item) => `${item.field}:${item.code}`)
          .join(",");
        throw new Error(
          `Native draft rejected: HTTP ${createdStatus}; code ${code && /^[a-z_]+$/.test(code) ? code : "unavailable"}; fields ${fields || "unavailable"}`,
        );
      }
      expect(createdStatus).toBe(201);
      await page.waitForURL((url) =>
        /\/anfitrion\/espacios\/[0-9a-f-]{36}\/editar$/.test(url.pathname),
      );
      await expect(page.locator("h1")).toHaveText(title);
      fixture = {
        id: new URL(page.url()).pathname.split("/").at(-2)!,
        title,
      };
    }
    expect(spacesReadStatus).toBe(200);
    expect(categoriesReadStatus).toBe(200);
    expect(
      /^[0-9a-f]{8}-[0-9a-f-]{27}$/i.test(fixture?.id ?? ""),
      "the synthetic space must resolve to an opaque UUID",
    ).toBe(true);
    if (createdNow) {
      expect(createdStatus).toBe(201);
      expect(createAuthorizationValid).toBe(true);
      expect(createSentCookie).toBe(false);
      // POST /spaces has no idempotency header in Go/OpenAPI; do not widen
      // the gateway allowlist or claim replay safety that is not implemented.
      expect(createSentUnsupportedIdempotencyKey).toBe(false);
    }
    expect(nonLoopback).toBe(false);
    expect(secretInConsole).toBe(false);
    await logout(page);
    mayHaveSession = false;
  } finally {
    if (mayHaveSession) await logout(page).catch(() => undefined);
  }
});

test("@f6 @native @local-document-scan-only completes the visible document flow", async ({
  page,
}) => {
  test.skip(
    !scanOnly,
    "Requires the explicitly selected local-document-scan-only profile",
  );
  test.setTimeout(180_000);
  const { owner, second } = await privateAccounts();
  type NativeFile = {
    id: string;
    state: "pendiente" | "limpio" | "bloqueado" | "error";
    mime: string;
    pages: number;
  };
  type NativeCase = {
    id: string;
    purpose: "identity" | "space";
    space_id?: string;
    state: string;
    revision: number;
    files: NativeFile[];
  };
  type NativeSpace = { id: string; title: string };

  const baseURL = test.info().project.use.baseURL;
  expect(
    typeof baseURL === "string",
    "a local Playwright base URL is required",
  ).toBe(true);
  const browserOrigin = new URL(baseURL as string).origin;
  const spacesPath = "/api/backend/spaces";
  const casesPath = "/api/backend/document-verification/cases";
  const secrets = [owner.email, owner.password, second.email, second.password];
  let casePath: string | undefined;
  let fileContentPath: string | undefined;
  let expectedETag = "";
  let nonLoopback = false;
  let crossOriginBff = false;
  let missingBearer = false;
  let sentCookie = false;
  let secretInConsole = false;
  let sawSpacesList = false;
  let sawCasesList = false;
  let sawCreate = false;
  let sawOwnCaseRead = false;
  let sawUpload = false;
  let uploadIfMatchIsCurrent = false;
  let sawSubmit = false;
  let submitIfMatchIsCurrent = false;
  let sawDownload = false;
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (
      ["http:", "https:"].includes(url.protocol) &&
      url.hostname !== "127.0.0.1"
    ) {
      nonLoopback = true;
    }
    if (url.pathname.startsWith("/api/backend/")) {
      if (url.origin !== browserOrigin) crossOriginBff = true;
      const headers = request.headers();
      const protectedRoute =
        url.pathname === spacesPath ||
        url.pathname.startsWith("/api/backend/document-verification/");
      if (protectedRoute) {
        if (!headers.authorization?.startsWith("Bearer ")) missingBearer = true;
        if ("cookie" in headers) sentCookie = true;
      }
      if (url.pathname === spacesPath && request.method() === "GET") {
        sawSpacesList = true;
      }
      if (url.pathname === casesPath && request.method() === "GET") {
        sawCasesList = true;
      }
      if (url.pathname === casesPath && request.method() === "POST") {
        sawCreate = true;
      }
      if (casePath && url.pathname === casePath && request.method() === "GET") {
        sawOwnCaseRead = true;
      }
      if (
        casePath &&
        url.pathname === `${casePath}/files` &&
        request.method() === "POST"
      ) {
        sawUpload = true;
        uploadIfMatchIsCurrent = headers["if-match"] === expectedETag;
      }
      if (
        casePath &&
        url.pathname === `${casePath}/submit` &&
        request.method() === "POST"
      ) {
        sawSubmit = true;
        submitIfMatchIsCurrent = headers["if-match"] === expectedETag;
      }
      if (
        fileContentPath &&
        url.pathname === fileContentPath &&
        request.method() === "GET"
      ) {
        sawDownload = true;
      }
    }
  });
  page.on("console", (message) => {
    const text = message.text();
    if (
      secrets.some((secret) => text.includes(secret)) ||
      /\b(?:bearer\s+\S+|access[_ -]?token)\b/i.test(text)
    ) {
      secretInConsole = true;
    }
  });

  let mayHaveSession = false;
  try {
    const spacesResponsePromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === spacesPath &&
        response.request().method() === "GET",
      { timeout: 20_000 },
    );
    const casesResponsePromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === casesPath &&
        response.request().method() === "GET",
      { timeout: 20_000 },
    );
    mayHaveSession = true;
    await login(page, owner);
    await expect(
      page.getByRole("heading", { name: "Documentos de verificación" }),
    ).toBeVisible();
    const [spacesResponse, casesResponse] = await Promise.all([
      spacesResponsePromise,
      casesResponsePromise,
    ]);
    expect(spacesResponse.status()).toBe(200);
    expect(casesResponse.status()).toBe(200);
    const spaces = (await spacesResponse.json()) as { items: NativeSpace[] };
    const initialCases = (await casesResponse.json()) as NativeCase[];
    const space = spaces.items.find((item) => item.title === fixtureTitle);
    expect(
      Boolean(space),
      "the authorized owner must have the F5 synthetic space",
    ).toBe(true);
    const spaceID = space?.id ?? "";
    expect(
      /^[0-9a-f]{8}-[0-9a-f-]{27}$/i.test(spaceID),
      "the F5 space must have an opaque UUID",
    ).toBe(true);

    await page.getByLabel("Tipo de expediente").selectOption("space");
    const spaceSelect = page.getByLabel("Espacio propio");
    const selectableSpaceIDs = await spaceSelect
      .locator("option")
      .evaluateAll((options) =>
        options.map((option) => (option as HTMLOptionElement).value),
      );
    expect(
      selectableSpaceIDs.includes(spaceID),
      "the owner UI must offer the F5 space",
    ).toBe(true);
    await spaceSelect.selectOption(spaceID);

    const existingIndex = initialCases.findIndex(
      (item) =>
        item.space_id === spaceID &&
        [
          "borrador",
          "enviada",
          "en_revision",
          "requiere_antecedentes",
        ].includes(item.state),
    );
    let targetCaseID =
      existingIndex >= 0 ? initialCases[existingIndex]!.id : "";
    let caseIndex = existingIndex;
    let createdNow = false;
    if (!targetCaseID) {
      createdNow = true;
      const createResponsePromise = page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname === casesPath &&
          response.request().method() === "POST",
        { timeout: 20_000 },
      );
      await page
        .getByRole("button", { name: "Crear expediente", exact: true })
        .click();
      const createResponse = await createResponsePromise;
      expect(createResponse.status()).toBe(201);
      const created = (await createResponse.json()) as NativeCase;
      targetCaseID = created.id;
      caseIndex = 0;
      await expect(
        page.getByText(
          "Expediente creado. Puedes cargar los documentos requeridos.",
        ),
      ).toBeVisible();
    }
    casePath = `${casesPath}/${encodeURIComponent(targetCaseID)}`;
    const caseButtons = page.locator("ul.divide-y button");
    await expect(caseButtons).toHaveCount(
      initialCases.length + (createdNow ? 1 : 0),
    );
    const targetCaseButton = createdNow
      ? page.locator('button[aria-current="true"]')
      : caseButtons.nth(caseIndex);
    const detailReadPromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === casePath &&
        response.request().method() === "GET",
      { timeout: 20_000 },
    );
    await targetCaseButton.click();
    const detailResponse = await detailReadPromise;
    expect(detailResponse.status()).toBe(200);
    let currentCase = (await detailResponse.json()) as NativeCase;
    expect(
      currentCase.id === targetCaseID,
      "the visible selection must resolve to the target case",
    ).toBe(true);
    let caseETag = detailResponse.headers()["etag"] ?? "";
    expect(
      /^"(?:0|[1-9][0-9]*)"$/.test(caseETag),
      "case reads must return a strong ETag",
    ).toBe(true);

    if (currentCase.files.length === 0) {
      expect(
        ["borrador", "requiere_antecedentes"].includes(currentCase.state),
        "an empty case must be editable",
      ).toBe(true);
      expectedETag = caseETag;
      await page.getByLabel("PDF, JPEG o PNG · máximo 10 MiB").setInputFiles({
        name: "synthetic-f6-document.pdf",
        mimeType: "application/pdf",
        buffer: onePagePdf,
      });
      const uploadResponsePromise = page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname === `${casePath}/files` &&
          response.request().method() === "POST",
        { timeout: 30_000 },
      );
      await page.getByRole("button", { name: "Cargar documento" }).click();
      const uploadResponse = await uploadResponsePromise;
      expect(uploadResponse.status()).toBe(201);
      await expect(
        page.getByText("Documento recibido y pendiente de inspección."),
      ).toBeVisible();
      caseETag = uploadResponse.headers()["etag"] ?? "";
      expect(
        /^"(?:0|[1-9][0-9]*)"$/.test(caseETag),
        "uploads must return a new strong ETag",
      ).toBe(true);
    }

    const cleanLabel = page.getByText("Validado · 1 página", { exact: true });
    await expect
      .poll(
        async () => {
          if (await cleanLabel.isVisible().catch(() => false)) return true;
          const refresh = page.getByRole("button", {
            name: "Actualizar expedientes",
          });
          if (!(await refresh.isEnabled().catch(() => false))) return false;
          try {
            const refreshed = page.waitForResponse(
              (response) =>
                new URL(response.url()).pathname === casePath &&
                response.request().method() === "GET",
              { timeout: 12_000 },
            );
            await refresh.click();
            const response = await refreshed;
            if (response.status() !== 200) return false;
            currentCase = (await response.json()) as NativeCase;
            caseETag = response.headers()["etag"] ?? caseETag;
          } catch {
            return false;
          }
          return cleanLabel.isVisible().catch(() => false);
        },
        { timeout: 90_000, intervals: [500, 1_000, 2_000] },
      )
      .toBe(true);
    expect(
      currentCase.files.length > 0 &&
        currentCase.files.every((file) => file.state === "limpio"),
      "the selected case must have only scanner-approved files",
    ).toBe(true);

    let expectedFinalState = currentCase.state;
    if (["borrador", "requiere_antecedentes"].includes(currentCase.state)) {
      expectedETag = caseETag;
      const submitResponsePromise = page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname === `${casePath}/submit` &&
          response.request().method() === "POST",
        { timeout: 30_000 },
      );
      await page.getByRole("button", { name: "Enviar a revisión" }).click();
      const submitResponse = await submitResponsePromise;
      expect(submitResponse.status()).toBe(200);
      const submittedCase = (await submitResponse.json()) as NativeCase;
      expect(submittedCase.state).toBe("enviada");
      expectedFinalState = "enviada";
      await expect(
        page.getByText("Expediente enviado a revisión."),
      ).toBeVisible();
      caseETag = submitResponse.headers()["etag"] ?? "";
      expect(
        /^"(?:0|[1-9][0-9]*)"$/.test(caseETag),
        "submit must return a new strong ETag",
      ).toBe(true);
    } else {
      expect(
        ["enviada", "en_revision"].includes(currentCase.state),
        "a previously submitted case must remain submitted or in review",
      ).toBe(true);
    }

    const listRefreshPromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === casesPath &&
        response.request().method() === "GET",
      { timeout: 20_000 },
    );
    const finalReadPromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === casePath &&
        response.request().method() === "GET",
      { timeout: 20_000 },
    );
    await page.getByRole("button", { name: "Actualizar expedientes" }).click();
    const [finalListResponse, finalCaseResponse] = await Promise.all([
      listRefreshPromise,
      finalReadPromise,
    ]);
    expect(finalListResponse.status()).toBe(200);
    expect(finalCaseResponse.status()).toBe(200);
    currentCase = (await finalCaseResponse.json()) as NativeCase;
    caseETag = finalCaseResponse.headers()["etag"] ?? "";
    expect(
      currentCase.state === expectedFinalState,
      "an independent read must confirm the committed submitted state",
    ).toBe(true);
    await expect(
      page.getByText(/(?:Enviada|En revisión) · Vence/),
    ).toBeVisible();
    const cleanFile = currentCase.files.find((file) => file.state === "limpio");
    expect(
      Boolean(cleanFile),
      "the independent read must retain a clean space file",
    ).toBe(true);
    const downloadFileID = cleanFile?.id ?? "";
    fileContentPath = `${casesPath.replace(/\/cases$/, "")}/space-cases/${encodeURIComponent(targetCaseID)}/files/${encodeURIComponent(downloadFileID)}/content`;

    const downloadResponsePromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === fileContentPath &&
        response.request().method() === "GET",
      { timeout: 20_000 },
    );
    const downloadPromise = page.waitForEvent("download", { timeout: 20_000 });
    const downloadButtonClick = page
      .getByRole("button", { name: "Descargar" })
      .click();
    const [downloadResponse, download] = await Promise.all([
      downloadResponsePromise,
      downloadPromise,
      downloadButtonClick,
    ]);
    expect(downloadResponse.status()).toBe(200);
    expect(downloadResponse.headers()["content-type"]).toContain(
      "application/pdf",
    );
    // Read the actual user-visible download instead of canceling it and
    // asking Chromium for a response body it may no longer retain.
    expect(await download.failure()).toBeNull();
    const stream = await download.createReadStream();
    expect(
      Boolean(stream),
      "the visible download must have a readable stream",
    ).toBe(true);
    const chunks: Buffer[] = [];
    for await (const chunk of stream!) chunks.push(Buffer.from(chunk));
    const downloadedBytes = Buffer.concat(chunks);
    expect(
      downloadedBytes.subarray(0, 5).toString("ascii") === "%PDF-",
      `the authorized UI download must be a PDF (bytes=${downloadedBytes.length}, magic=${downloadedBytes.subarray(0, 5).toString("hex")})`,
    ).toBe(true);
    const pdfInfo = execFileSync("/usr/bin/pdfinfo", ["-"], {
      input: downloadedBytes,
      encoding: "utf8",
      timeout: 5_000,
    });
    expect(
      /Pages:\s+1\b/.test(pdfInfo),
      "the downloaded synthetic PDF must contain one page",
    ).toBe(true);
    if (createdNow || sawUpload) {
      expect(
        downloadedBytes.equals(onePagePdf),
        "the downloaded bytes must match the synthetic upload",
      ).toBe(true);
    }

    await logout(page);
    mayHaveSession = false;
    const secondCasesResponsePromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === casesPath &&
        response.request().method() === "GET",
      { timeout: 20_000 },
    );
    mayHaveSession = true;
    await login(page, second);
    const secondCasesResponse = await secondCasesResponsePromise;
    expect(secondCasesResponse.status()).toBe(200);
    const secondCases = (await secondCasesResponse.json()) as NativeCase[];
    expect(
      !secondCases.some((item) => item.id === targetCaseID),
      "the second synthetic account must not list the owner's case",
    ).toBe(true);
    await expect(
      page.getByRole("heading", { name: "Documentos de verificación" }),
    ).toBeVisible();
    await logout(page);
    mayHaveSession = false;

    if (createdNow)
      expect(sawCreate, "visible create must reach the BFF").toBe(true);
    if (sawUpload)
      expect(
        uploadIfMatchIsCurrent,
        "visible upload must use the current case ETag",
      ).toBe(true);
    if (sawSubmit)
      expect(
        submitIfMatchIsCurrent,
        "visible submit must use the current case ETag",
      ).toBe(true);
    expect(
      sawSpacesList && sawCasesList && sawOwnCaseRead && sawDownload,
      "all required document operations must traverse the same-origin BFF",
    ).toBe(true);
    expect(nonLoopback, "browser traffic must remain on loopback").toBe(false);
    expect(crossOriginBff, "BFF requests must remain same-origin").toBe(false);
    expect(
      missingBearer,
      "protected BFF requests must carry the session bearer",
    ).toBe(false);
    expect(sentCookie, "protected BFF requests must not carry cookies").toBe(
      false,
    );
    expect(
      secretInConsole,
      "credentials and bearer material must not reach browser logs",
    ).toBe(false);
  } finally {
    if (mayHaveSession) await logout(page).catch(() => undefined);
  }
});
