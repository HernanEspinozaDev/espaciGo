import { expect, test } from "@playwright/test";

test.use({ trace: "off", screenshot: "off", video: "off" });

test("late A declaration GET cannot overwrite account B after logout/login", async ({
  page,
}) => {
  const accounts = {
    "owner-a@example.invalid": "550e8400-e29b-41d4-a716-446655440000",
    "owner-b@example.invalid": "660e8400-e29b-41d4-a716-446655440000",
  } as const;
  const declarations = {
    [accounts["owner-a@example.invalid"]]: {
      id: "990e8400-e29b-41d4-a716-446655440000",
      revision: 1,
      first_name: "Synthetic Account A",
      last_name: "Synthetic A Last",
      date_of_birth: "1990-01-02",
      rut: "11222333-9",
      id_country: "CHL",
      document_type: "ID",
      consent_version: "id-consent-20261108",
      consent_given_at: "2026-11-13T12:00:00Z",
    },
    [accounts["owner-b@example.invalid"]]: {
      id: "880e8400-e29b-41d4-a716-446655440000",
      revision: 1,
      first_name: "Synthetic Account B",
      last_name: "Synthetic B Last",
      date_of_birth: "1991-02-03",
      rut: "76543210-K",
      id_country: "CHL",
      document_type: "ID",
      consent_version: "id-consent-20261108",
      consent_given_at: "2026-11-13T12:00:00Z",
    },
  };
  const outsideLoopback: string[] = [];
  const backendCalls: string[] = [];
  const declarationCalls: string[] = [];
  let accountId: string | null = null;
  let email: string | null = null;

  await page.addInitScript(() => {
    const nativeFetch = window.fetch.bind(window);
    let holdNextDeclarationGet = true;
    let releaseHeld: (() => void) | undefined;
    (
      window as typeof window & {
        __identityLateResponse: {
          dispatched: boolean;
          release: () => void;
        };
      }
    ).__identityLateResponse = {
      dispatched: false,
      release: () => releaseHeld?.(),
    };
    window.fetch = async (input, init) => {
      const request = input instanceof Request ? input : undefined;
      const url = new URL(request?.url ?? String(input), location.href);
      const method = (init?.method ?? request?.method ?? "GET").toUpperCase();
      const shouldHold =
        holdNextDeclarationGet &&
        method === "GET" &&
        url.pathname === "/api/backend/verifications/identity-declaration";
      if (!shouldHold) return nativeFetch(input, init);
      holdNextDeclarationGet = false;
      // Deliberately drop AbortSignal: this dispatched request resolves after its UI owner is obsolete.
      const initWithoutSignal = init
        ? { ...init, signal: undefined }
        : undefined;
      const response = await nativeFetch(input, initWithoutSignal);
      (
        window as typeof window & {
          __identityLateResponse: { dispatched: boolean };
        }
      ).__identityLateResponse.dispatched = true;
      await new Promise<void>((resolve) => {
        releaseHeld = resolve;
      });
      return response;
    };
  });
  page.on("request", (request) => {
    if (!request.url().startsWith("http://127.0.0.1:")) {
      outsideLoopback.push(request.url());
    }
  });

  await page.route("**/api/backend/**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    const method = request.method();
    const respond = (status: number, value: unknown) =>
      route.fulfill({
        status,
        headers: {
          "content-type": "application/json; charset=utf-8",
          "cache-control": "private, no-store",
        },
        body: JSON.stringify(value),
      });
    if (pathname === "/api/backend/auth/session" && method === "GET") {
      backendCalls.push("session");
      return accountId
        ? respond(200, {
            account_id: accountId,
            roles: ["arrendador"],
            capabilities: [],
            restricted_mode: false,
          })
        : respond(401, { error: { code: "unauthenticated" } });
    }
    if (pathname === "/api/backend/auth/login" && method === "POST") {
      const body = request.postDataJSON() as { email?: string };
      email = body.email ?? null;
      accountId =
        email && email in accounts
          ? accounts[email as keyof typeof accounts]
          : null;
      backendCalls.push(`login:${email}`);
      return accountId
        ? respond(200, {
            account_id: accountId,
            roles: ["arrendador"],
            capabilities: [],
            access_token: `synthetic-token-${email}`,
            token_type: "Bearer",
            expires_at: new Date(Date.now() + 60_000).toISOString(),
            restricted_mode: false,
          })
        : respond(401, { error: { code: "unauthenticated" } });
    }
    if (pathname === "/api/backend/auth/logout" && method === "POST") {
      backendCalls.push("logout");
      accountId = null;
      email = null;
      return route.fulfill({ status: 204 });
    }
    if (
      pathname === "/api/backend/verifications/identity-declaration" &&
      method === "GET"
    ) {
      declarationCalls.push(accountId ?? "anonymous");
      return accountId
        ? respond(200, declarations[accountId as keyof typeof declarations])
        : respond(401, { error: { code: "unauthenticated" } });
    }
    if (
      (pathname === "/api/backend/verifications" ||
        pathname === "/api/backend/verifications/eligibility" ||
        pathname === "/api/backend/spaces") &&
      method === "GET"
    )
      return respond(200, { items: [] });
    if (
      pathname === "/api/backend/document-verification/cases" &&
      method === "GET"
    )
      return respond(200, []);
    backendCalls.push(`${method}:${pathname}`);
    return respond(404, { error: { code: "not_found" } });
  });

  await page.goto("/acceso?return_to=%2Fmi-cuenta%2Fverificaciones");
  await page.getByLabel("Correo electrónico").fill("owner-a@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/verificaciones$/);
  const declaration = page.getByRole("region", {
    name: "Declaración de identidad",
  });
  await page.waitForFunction(
    () =>
      (
        window as typeof window & {
          __identityLateResponse: { dispatched: boolean };
        }
      ).__identityLateResponse.dispatched,
  );
  expect(declarationCalls).toEqual([accounts["owner-a@example.invalid"]]);
  await expect(
    declaration.getByText("Consultando declaración..."),
  ).toBeVisible();

  await page.getByRole("link", { name: "Acceso" }).click();
  await expect(page).toHaveURL(/\/acceso$/);
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  // Keep the SPA realm alive so the deliberately delayed A response can resolve.
  // Use the access screen's supported return target for B's login.
  await page.evaluate(() => {
    window.history.replaceState(
      {},
      "",
      "/acceso?return_to=%2Fmi-cuenta%2Fverificaciones",
    );
  });
  await expect(page).toHaveURL(/\/acceso\?return_to=/);
  await page.getByLabel("Correo electrónico").fill("owner-b@example.invalid");
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Synthetic-only-123!");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await expect(page).toHaveURL(/\/mi-cuenta\/verificaciones$/);
  await expect(declaration.getByText("Synthetic Account B")).toBeVisible();
  await expect(declaration.getByText("Synthetic B Last")).toBeVisible();
  await expect(declaration.getByText("1991-02-03")).toBeVisible();
  await expect(declaration.getByText("76543210-K")).toBeVisible();
  expect(declarationCalls).toEqual([
    accounts["owner-a@example.invalid"],
    accounts["owner-b@example.invalid"],
  ]);
  await expect(declaration.getByText("Synthetic Account A")).toHaveCount(0);
  await expect(declaration.getByText("Synthetic A Last")).toHaveCount(0);
  await expect(declaration.getByText("1990-01-02")).toHaveCount(0);
  await expect(declaration.getByText("11222333-9")).toHaveCount(0);

  await page.evaluate(() =>
    (
      window as typeof window & {
        __identityLateResponse: { release: () => void };
      }
    ).__identityLateResponse.release(),
  );
  await expect(declaration.getByText("Synthetic Account A")).toHaveCount(0);
  await expect(declaration.getByText("Synthetic A Last")).toHaveCount(0);
  await expect(declaration.getByText("1990-01-02")).toHaveCount(0);
  await expect(declaration.getByText("11222333-9")).toHaveCount(0);
  await expect(declaration.getByText("Synthetic Account B")).toBeVisible();
  await expect(declaration.getByText("Synthetic B Last")).toBeVisible();
  await expect(declaration.getByText("1991-02-03")).toBeVisible();
  await expect(declaration.getByText("76543210-K")).toBeVisible();
  await expect(declaration.getByText("Consultando declaración...")).toHaveCount(
    0,
  );
  await expect(declaration.getByText(/No se pudo|Error|error/i)).toHaveCount(0);
  expect(backendCalls.filter((call) => call === "logout")).toEqual(["logout"]);
  expect(backendCalls.filter((call) => call.startsWith("login:"))).toEqual([
    "login:owner-a@example.invalid",
    "login:owner-b@example.invalid",
  ]);
  expect(declarationCalls).toEqual([
    accounts["owner-a@example.invalid"],
    accounts["owner-b@example.invalid"],
  ]);
  expect(outsideLoopback).toEqual([]);
});
