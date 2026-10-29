import { expect, test } from "@playwright/test";
import { createBffFixtureState } from "./helpers/bff-fixture";

test.use({ trace: "off", screenshot: "off", video: "off" });

test("@phase7 listado publica y oculta con ETag; 412 y GET fallido no reintentan", async ({
  page,
}) => {
  const id = "00000000-0000-4000-8000-000000000081";
  const accountId = "770e8400-e29b-41d4-a716-446655440000";
  const expiresAt = new Date(Date.now() + 3600_000).toISOString();
  const bff = createBffFixtureState();
  let revision = 0;
  let detailGets = 0;
  let rejectMutation = false;
  let rejectRead = false;
  const mutations: Array<{ tag: string; state: string }> = [];
  const tag = () =>
    `"${Buffer.from(`${id}\0${new Date(Date.UTC(2026, 9, 28, 12, 0, revision)).toISOString().replace(".000Z", "Z")}`).toString("base64url")}"`;
  const space = {
    id,
    category_code: "oficina",
    category_name: "Oficina",
    title: "Oficina CAS sintética",
    description:
      "Descripción sintética de oficina para comprobar acciones de publicación con concurrencia y persistencia simulada.",
    area_m2: 20,
    capacity: 4,
    usage_rules: "Sin fumar",
    rate_unit: "hora",
    base_price_clp: 8000,
    address: "Dirección sintética",
    state: "borrador",
    time_zone: "America/Santiago",
    attribute_schema_version: 1,
    attributes: {},
  };
  page.on("request", (request) => {
    expect(new URL(request.url()).hostname).toBe("127.0.0.1");
  });
  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    let result: {
      status: number;
      body?: unknown;
      headers?: Record<string, string>;
    };
    const error = (status: number, code: string) => ({
      status,
      body: { error: { code, message: "Synthetic error" } },
    });
    if (path === "/api/backend/auth/login") {
      result = {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario", "anfitrion"],
          capabilities: [],
          expires_at: expiresAt,
          restricted_mode: false,
        },
      };
    } else if (path === "/api/backend/auth/session") {
      result = {
        status: 200,
        body: {
          account_id: accountId,
          roles: ["arrendatario", "anfitrion"],
          capabilities: [],
          restricted_mode: false,
        },
      };
    } else if (path === "/api/backend/spaces" && method === "GET") {
      result = { status: 200, body: { items: [space] } };
    } else if (path === `/api/backend/spaces/${id}` && method === "GET") {
      detailGets++;
      result = rejectRead
        ? error(503, "unavailable")
        : { status: 200, body: space, headers: { etag: tag() } };
    } else if (
      path === `/api/backend/spaces/${id}/publication` &&
      method === "PUT"
    ) {
      const body = request.postDataJSON() as { state: string };
      const expected = tag();
      mutations.push({
        tag: request.headers()["if-match"] ?? "",
        state: body.state,
      });
      expect(mutations.at(-1)?.tag).toBe(expected);
      if (rejectMutation) result = error(412, "precondition_failed");
      else {
        space.state = body.state;
        revision++;
        result = { status: 200, body: space, headers: { etag: tag() } };
      }
    } else if (path === `/api/backend/spaces/${id}/gallery`) {
      result = { status: 200, body: { items: [] } };
    } else result = error(404, "not_found");
    const adapted = bff.adapt(request, result);
    if (!adapted) throw new Error("Missing fixture reply");
    await route.fulfill({
      status: adapted.status,
      headers: {
        "content-type": "application/json",
        "cache-control": "no-store",
        ...adapted.headers,
      },
      body: JSON.stringify(adapted.body ?? {}),
    });
  });
  await page.goto("/acceso?return_to=%2Fanfitrion%2Fespacios");
  await page.getByLabel("Correo electrónico").fill("list-cas@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("SyntheticOnly123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL((url) => url.pathname === "/anfitrion/espacios");
  const publish = page.getByRole("button", {
    name: `Publicar ${space.title}`,
    exact: true,
  });
  await publish.click();
  await expect(
    page.getByText("Espacio publicado exitosamente con estado activo.", {
      exact: true,
    }),
  ).toBeVisible();
  expect(mutations).toHaveLength(1);
  expect(detailGets).toBe(1);
  await page
    .getByRole("button", { name: `Archivar ${space.title}`, exact: true })
    .click();
  await expect(
    page.getByText(
      "Espacio archivado. Ya no será visible en el catálogo público.",
      { exact: true },
    ),
  ).toBeVisible();
  expect(mutations).toHaveLength(2);
  expect(mutations[1]?.tag).not.toBe(mutations[0]?.tag);
  const reactivate = page.getByRole("button", {
    name: `Reactivar ${space.title}`,
    exact: true,
  });
  rejectMutation = true;
  await reactivate.click();
  await expect(
    page.getByRole("alert").filter({ hasText: "412" }),
  ).toBeVisible();
  expect(mutations).toHaveLength(3);
  expect(detailGets).toBe(3);
  await expect(reactivate).toBeEnabled();
  expect(space.state).toBe("oculta");
  rejectRead = true;
  await reactivate.click();
  await expect(
    page.getByRole("alert").filter({ hasText: "503" }),
  ).toBeVisible();
  expect(mutations).toHaveLength(3);
  expect(detailGets).toBe(4);
  await expect(reactivate).toBeEnabled();
});
