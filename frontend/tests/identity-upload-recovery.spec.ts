import {
  expect,
  test,
  type Page,
  type Request,
  type Route,
} from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";

type MockResult = {
  status: number;
  body?: unknown;
  headers?: Record<string, string>;
};

const accountId = "770e8400-e29b-41d4-a716-446655440021";
const caseId = "990e8400-e29b-41d4-a716-446655440021";

async function installMock(
  page: Page,
  handler: (path: string, method: string, request: Request) => MockResult,
) {
  const fixture = createBffFixtureState();
  await page.route("**/api/backend/**", async (route: Route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const expiresAt = new Date(Date.now() + 3_600_000).toISOString();
    let response: MockResult;
    if (path === "/api/backend/auth/login" && request.method() === "POST") {
      response = {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendador"],
          capabilities: [],
          restricted_mode: false,
          expires_at: expiresAt,
        },
      };
    } else if (
      path === "/api/backend/auth/session" &&
      request.method() === "GET"
    ) {
      response = {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendador"],
          capabilities: [],
          restricted_mode: false,
          expires_at: expiresAt,
        },
      };
    } else {
      response = handler(path, request.method(), request);
    }
    const adapted = fixture.adapt(request, response);
    if (!adapted) throw new Error("Missing controlled BFF response");
    await route.fulfill({
      status: adapted.status,
      headers: {
        "content-type": "application/json; charset=utf-8",
        "cache-control": "no-store",
        ...adapted.headers,
      },
      body: JSON.stringify(adapted.body ?? {}),
    });
  });
}

async function signIn(page: Page) {
  const target = "/mi-cuenta/documentos";
  await page.goto(`/acceso?return_to=${encodeURIComponent(target)}`);
  await page
    .getByLabel("Correo electrónico")
    .fill("identity-test@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("SyntheticOnly123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname === target);
}

test("clears a selected identity file after refresh makes the case non-editable", async ({
  page,
}) => {
  let created = false;
  let caseState = "borrador";
  let revision = 1;
  let etag = '"1"';
  const uploadEtags: string[] = [];
  let uploadAttempts = 0;
  let submitAttempts = 0;

  const filesForCurrentState = () => {
    if (caseState === "borrador") return [];
    const backState =
      caseState === "requiere_antecedentes" ? "bloqueado" : "limpio";
    return [
      {
        id: "880e8400-e29b-41d4-a716-446655440021",
        evidence_role: "identity_front",
        state: "limpio",
        mime: "image/png",
        size: 128,
        pages: 0,
      },
      {
        id: "880e8400-e29b-41d4-a716-446655440022",
        evidence_role: "identity_back",
        state: backState,
        mime: "image/png",
        size: 128,
        pages: 0,
      },
      {
        id: "880e8400-e29b-41d4-a716-446655440023",
        evidence_role: "identity_selfie",
        state: "limpio",
        mime: "image/png",
        size: 128,
        pages: 0,
      },
    ];
  };
  const ownerCase = () => ({
    id: caseId,
    purpose: "identity",
    state: caseState,
    revision,
    files: filesForCurrentState(),
  });

  await installMock(page, (path, method, request) => {
    if (path === "/api/backend/spaces" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      return { status: 200, body: created ? [ownerCase()] : [] };
    }
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "POST"
    ) {
      created = true;
      return {
        status: 201,
        body: ownerCase(),
        headers: { etag },
      };
    }
    if (
      path === `/api/backend/document-verification/cases/${caseId}` &&
      method === "GET"
    ) {
      return { status: 200, body: ownerCase(), headers: { etag } };
    }
    if (
      path === `/api/backend/document-verification/cases/${caseId}/files` &&
      method === "POST"
    ) {
      uploadAttempts += 1;
      uploadEtags.push(request.headers()["if-match"] ?? "");
      caseState = "en_revision";
      revision = 2;
      etag = '"2"';
      return {
        status: 409,
        body: {
          error: {
            code: "etag_conflict",
            message: "Synthetic stale revision",
          },
        },
      };
    }
    if (
      path === `/api/backend/document-verification/cases/${caseId}/submit` &&
      method === "POST"
    ) {
      submitAttempts += 1;
      return { status: 409, body: { error: { code: "unexpected_submit" } } };
    }
    return { status: 404, body: { error: { code: "not_found" } } };
  });

  await signIn(page);
  await page
    .getByLabel("Formato declarado de la cédula")
    .selectOption("unsure");
  await page
    .getByLabel(
      "Confirmo el aviso de prueba local para un expediente sintético",
    )
    .check();
  await page.getByRole("button", { name: "Crear expediente" }).click();
  await expect(
    page.getByText("Expediente de identidad · Borrador"),
  ).toBeVisible();

  await page.getByLabel("Evidencia a cargar").selectOption("identity_back");
  await page.getByLabel("JPEG o PNG · máximo 10 MiB").setInputFiles({
    name: "synthetic-back.png",
    mimeType: "image/png",
    buffer: Buffer.from("synthetic identity evidence"),
  });
  await page.getByRole("button", { name: "Cargar evidencia" }).click();
  await expect(
    page.getByText(
      "El expediente cambió desde la última consulta. Actualízalo antes de reintentar.",
    ),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Cargar evidencia" }),
  ).toBeDisabled();
  expect(uploadAttempts).toBe(1);
  expect(uploadEtags).toEqual(['"1"']);

  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Expediente de identidad · En revisión", { exact: true }),
  ).toBeVisible();
  await expect(page.locator('input[type="file"]')).toHaveCount(0);

  caseState = "requiere_antecedentes";
  revision = 3;
  etag = '"3"';
  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByText("Expediente de identidad · Requiere antecedentes", {
      exact: true,
    }),
  ).toBeVisible();
  const fileInput = page.locator('input[type="file"]');
  await expect(fileInput).toBeVisible();
  expect(
    await fileInput.evaluate(
      (input) => (input as HTMLInputElement).files?.length ?? 0,
    ),
  ).toBe(0);
  await expect(
    page.getByRole("button", { name: "Cargar evidencia" }),
  ).toBeDisabled();
  expect(uploadAttempts).toBe(1);
  expect(submitAttempts).toBe(0);
});

test("keeps a selected identity file after refreshing an editable case", async ({
  page,
}) => {
  let created = false;
  let uploaded = false;
  let revision = 1;
  let etag = '"1"';
  let uploadAttempts = 0;
  let submitAttempts = 0;
  const uploadEtags: string[] = [];
  const uploadedFile = {
    id: "880e8400-e29b-41d4-a716-446655440024",
    evidence_role: "identity_back",
    state: "pendiente",
    mime: "image/png",
    size: 128,
    pages: 0,
  };
  const ownerCase = () => ({
    id: caseId,
    purpose: "identity",
    state: "borrador",
    revision,
    files: uploaded ? [uploadedFile] : [],
  });

  await installMock(page, (path, method, request) => {
    if (path === "/api/backend/spaces" && method === "GET") {
      return { status: 200, body: { items: [] } };
    }
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "GET"
    ) {
      return { status: 200, body: created ? [ownerCase()] : [] };
    }
    if (
      path === "/api/backend/document-verification/cases" &&
      method === "POST"
    ) {
      created = true;
      return { status: 201, body: ownerCase(), headers: { etag } };
    }
    if (
      path === `/api/backend/document-verification/cases/${caseId}` &&
      method === "GET"
    ) {
      return { status: 200, body: ownerCase(), headers: { etag } };
    }
    if (
      path === `/api/backend/document-verification/cases/${caseId}/files` &&
      method === "POST"
    ) {
      uploadAttempts += 1;
      uploadEtags.push(request.headers()["if-match"] ?? "");
      if (uploadAttempts === 1) {
        return {
          status: 409,
          body: {
            error: {
              code: "etag_conflict",
              message: "Synthetic stale revision",
            },
          },
        };
      }
      uploaded = true;
      revision += 1;
      etag = `"${revision}"`;
      return {
        status: 201,
        body: { revision, file: uploadedFile },
        headers: { etag },
      };
    }
    if (
      path === `/api/backend/document-verification/cases/${caseId}/submit` &&
      method === "POST"
    ) {
      submitAttempts += 1;
      return { status: 409, body: { error: { code: "unexpected_submit" } } };
    }
    return { status: 404, body: { error: { code: "not_found" } } };
  });

  await signIn(page);
  await page
    .getByLabel("Formato declarado de la cédula")
    .selectOption("unsure");
  await page
    .getByLabel(
      "Confirmo el aviso de prueba local para un expediente sintético",
    )
    .check();
  await page.getByRole("button", { name: "Crear expediente" }).click();
  await expect(
    page.getByText("Expediente de identidad · Borrador"),
  ).toBeVisible();
  await page.getByLabel("Evidencia a cargar").selectOption("identity_back");
  await page.getByLabel("JPEG o PNG · máximo 10 MiB").setInputFiles({
    name: "synthetic-back-recovery.png",
    mimeType: "image/png",
    buffer: Buffer.from("synthetic identity evidence"),
  });
  await page.getByRole("button", { name: "Cargar evidencia" }).click();
  await expect(
    page.getByText(
      "El expediente cambió desde la última consulta. Actualízalo antes de reintentar.",
    ),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Cargar evidencia" }),
  ).toBeDisabled();
  expect(uploadAttempts).toBe(1);

  await page.getByRole("button", { name: "Actualizar expedientes" }).click();
  await expect(
    page.getByRole("button", { name: "Cargar evidencia" }),
  ).toBeEnabled();
  const fileInput = page.locator('input[type="file"]');
  expect(
    await fileInput.evaluate(
      (input) => (input as HTMLInputElement).files?.length ?? 0,
    ),
  ).toBe(1);
  expect(uploadAttempts).toBe(1);

  await page.getByRole("button", { name: "Cargar evidencia" }).click();
  await expect(
    page.getByText("Evidencia recibida y pendiente de inspección."),
  ).toBeVisible();
  expect(uploadAttempts).toBe(2);
  expect(uploadEtags).toEqual(['"1"', '"1"']);
  expect(submitAttempts).toBe(0);
});
