import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { lstat, mkdir, readFile } from "node:fs/promises";
import path from "node:path";

const enabled = process.env.PHASE_PUBLIC_VERIFICATION_NATIVE_E2E === "1";
test.skip(
  !enabled,
  "Requires the explicitly authorized local native verification trial",
);
test.use({ trace: "off", screenshot: "off", video: "off" });

const origin = "https://127.0.0.1:4203";
const reviewPath = "/mi-cuenta/revision-documental";
const scenario = process.env.PHASE_PUBLIC_VERIFICATION_SCENARIO;

type Account = { email: string; password: string; account_id: string };
type Fixture = {
  case_id: string;
  file_id: string;
  owner_id: string;
  reviewer_id: string;
  purpose: "identity" | "space";
  space_id: string;
  revision: number;
  scanner: string;
};

async function privateInput(root: string, selected: string) {
  const relative = path.relative(root, selected);
  if (
    !path.isAbsolute(root) ||
    !path.isAbsolute(selected) ||
    relative.startsWith("..") ||
    path.isAbsolute(relative)
  )
    throw new Error("Private trial input outside authorized root");
  const info = await lstat(selected);
  if (!info.isFile() || info.isSymbolicLink() || (info.mode & 0o777) !== 0o600)
    throw new Error("Trial input must be a private regular file");
  return JSON.parse(await readFile(selected, "utf8")) as unknown;
}

async function login(page: Page, account: Account) {
  await page.goto(`/acceso?return_to=${encodeURIComponent(reviewPath)}`);
  await page.getByLabel("Correo electrónico").fill(account.email);
  await page.getByLabel("Contraseña", { exact: true }).fill(account.password);
  const response = page.waitForResponse(
    (r) =>
      new URL(r.url()).pathname === "/api/backend/auth/login" &&
      r.request().method() === "POST",
  );
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  expect((await response).status()).toBe(200);
  await expect(page).toHaveURL((url) => url.pathname === reviewPath);
}

function watchOrigin(context: BrowserContext, state: { external: boolean }) {
  context.on("request", (request) => {
    const url = new URL(request.url());
    if (
      ["http:", "https:", "blob:"].includes(url.protocol) &&
      url.origin !== origin
    )
      state.external = true;
  });
}

async function publicState(context: BrowserContext, spaceId: string) {
  const response = await context.request.get(
    `/api/backend/public/spaces/${encodeURIComponent(spaceId)}`,
  );
  expect(response.status()).toBe(200);
  const payload: unknown = await response.json();
  expect(payload).toHaveProperty("data");
  const data = (payload as { data: Record<string, unknown> }).data;
  expect(data).toHaveProperty("verification");
  const verification = data.verification as Record<string, unknown>;
  expect(typeof verification.identity_verified).toBe("boolean");
  expect(typeof verification.space_verified).toBe("boolean");
  const serialized = JSON.stringify(payload).toLowerCase();
  for (const forbidden of [
    "owner_id",
    "reviewer_id",
    "case_id",
    "file_id",
    "document_expires_at",
    "verified_until",
    "evidence",
    "address",
    "email",
    "phone",
  ])
    expect(serialized).not.toContain(forbidden);
  return {
    identity: verification.identity_verified as boolean,
    space: verification.space_verified as boolean,
    payload,
  };
}

async function detailAndA11y(page: Page, spaceId: string) {
  await page.goto(`/espacios/${encodeURIComponent(spaceId)}`);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  for (const viewport of [
    { width: 1440, height: 1000 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport);
    const axe = await new AxeBuilder({ page }).analyze();
    expect(axe.violations.map((v) => v.id)).toEqual([]);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  }
}

async function capture(page: Page, stage: string) {
  const dir = process.env.PHASE_PUBLIC_VERIFICATION_CAPTURE_DIR;
  if (!dir) return;
  await mkdir(dir, { recursive: true, mode: 0o700 });
  await page.screenshot({
    path: path.join(dir, `${scenario}-${stage}.png`),
    fullPage: true,
  });
}

test("@native anonymous public verification reflects reviewer decisions, revocation and expiry", async ({
  browser,
}) => {
  expect([
    "identity",
    "space-revoke",
    "space-expire",
    "identity-revoke",
    "demo",
  ]).toContain(scenario);
  const root = process.env.PHASE6_PRIVATE_TESTING_DIR;
  const inventoryFile = process.env.PHASE6_ACCOUNT_INVENTORY_FILE;
  const fixtureFile = process.env.PHASE_DOCUMENT_REVIEW_FIXTURE_FILE;
  if (!root || !inventoryFile || !fixtureFile)
    throw new Error("Explicit private synthetic inputs required");
  const policy = (await privateInput(
    root,
    path.join(root, "test-account-policy.json"),
  )) as Record<string, { existing_accounts_only?: boolean }>;
  expect(
    policy.document_reviewer_authorization_20261105?.existing_accounts_only,
  ).toBe(true);
  const fixture = (await privateInput(root, fixtureFile)) as Fixture;
  if (
    fixture.scanner !== "real-ClamAV" ||
    !fixture.case_id ||
    !fixture.file_id ||
    !fixture.space_id
  )
    throw new Error("Parent-prepared exact clean document fixture required");
  expect(fixture.purpose).toBe(
    scenario === "identity" ||
      scenario === "identity-revoke" ||
      scenario === "demo"
      ? "identity"
      : "space",
  );
  const inventory = (await privateInput(root, inventoryFile)) as {
    accounts: Account[];
  };
  const reviewer = inventory.accounts.find(
    (a) => a.account_id === fixture.reviewer_id,
  );
  const owner = inventory.accounts.find(
    (a) => a.account_id === fixture.owner_id,
  );
  if (!reviewer || !owner || owner.account_id === reviewer.account_id)
    throw new Error("Two distinct existing trial accounts required");

  const state = { external: false };
  const reviewerContext = await browser.newContext({
    baseURL: origin,
    ignoreHTTPSErrors: true,
    timezoneId: "UTC",
  });
  const anonymousContext = await browser.newContext({
    baseURL: origin,
    ignoreHTTPSErrors: true,
    timezoneId: "UTC",
  });
  watchOrigin(reviewerContext, state);
  watchOrigin(anonymousContext, state);
  const page = await reviewerContext.newPage();
  const publicPage = await anonymousContext.newPage();
  const api = `/api/backend/document-verification/reviewer/cases/${encodeURIComponent(fixture.case_id)}`;
  const approvedScenario = scenario === "identity-revoke";
  const requiredExpiry = scenario === "space-expire";
  let stage = "login";
  try {
    await login(page, reviewer);
    stage = "open-assigned-case";
    await page.locator(`[data-review-case-id="${fixture.case_id}"]`).click();
    await expect(
      page.locator(
        `[data-review-case-state="${approvedScenario ? "aprobada" : "enviada"}"]`,
      ),
    ).toBeVisible();
    if (fixture.purpose === "identity" && !approvedScenario) {
      const downloadEvent = page.waitForEvent("download");
      await page.locator(`[data-review-file-id="${fixture.file_id}"]`).click();
      const download = await downloadEvent;
      expect(await download.failure()).toBeNull();
    }
    const beforeDetail = (await (
      await anonymousContext.request.get(
        `/api/backend/public/spaces/${encodeURIComponent(fixture.space_id)}`,
      )
    ).json()) as { data: { title: string } };
    stage = "public-before-and-a11y";
    await detailAndA11y(publicPage, fixture.space_id);
    let result = await publicState(anonymousContext, fixture.space_id);

    if (scenario === "demo") {
      expect(result.identity).toBe(false);
      expect(result.space).toBe(false);
      await publicPage.goto(
        `/explorar?q=${encodeURIComponent(beforeDetail.data.title)}`,
      );
      const card = publicPage
        .locator(`a[href="/espacios/${fixture.space_id}"]`)
        .filter({ has: publicPage.locator("h2") });
      await expect(card).toHaveCount(1);
      await expect(
        card.getByText("Identidad verificada", { exact: true }),
      ).toHaveCount(0);
      await expect(
        card.getByText("Antecedentes del espacio revisados", { exact: true }),
      ).toHaveCount(0);
      expect(state.external).toBe(false);
      return;
    }

    if (approvedScenario) {
      expect(result.identity).toBe(true);
      expect(result.space).toBe(false);
    } else {
      expect(result.identity).toBe(
        scenario === "space-revoke" || scenario === "space-expire",
      );
      expect(result.space).toBe(false);
      await capture(publicPage, "before");
      const beforeResponse = await page.request.get(api);
      expect(beforeResponse.status()).toBe(200);
      const before = (await beforeResponse.json()) as { revision: number };
      stage = "approve-through-interface";
      await page
        .getByRole("combobox", { name: "Decisión", exact: true })
        .selectOption("approve");
      const expiryDate = new Date(
        Date.now() + (requiredExpiry ? 120_000 : 7 * 24 * 60 * 60 * 1000),
      );
      const expiry = expiryDate.toISOString().slice(0, 16);
      await page.getByLabel("Vigencia hasta").fill(expiry);
      await expect(
        page.getByRole("button", { name: "Confirmar decisión", exact: true }),
      ).toBeEnabled();
      const approvedResponse = page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname === `${api}/review` &&
          r.request().method() === "POST",
      );
      await page
        .getByRole("button", { name: "Confirmar decisión", exact: true })
        .click();
      expect((await approvedResponse).status()).toBe(200);
      stage = "approval-readback";
      await expect(
        page.locator('[data-review-case-state="aprobada"]'),
      ).toBeVisible();
      const readback = (await (await page.request.get(api)).json()) as {
        state: string;
        revision: number;
      };
      expect(readback.state).toBe("aprobada");
      expect(Number(readback.revision)).toBeGreaterThan(
        Number(before.revision),
      );
      if (requiredExpiry) {
        stage = "public-visible-before-expiry";
        result = await publicState(anonymousContext, fixture.space_id);
        expect(result.identity).toBe(true);
        expect(result.space).toBe(true);
        await detailAndA11y(publicPage, fixture.space_id);
        await expect(
          publicPage.getByText("Antecedentes del espacio revisados", {
            exact: true,
          }),
        ).toBeVisible();
        await capture(publicPage, "approved");
        await expect
          .poll(
            async () =>
              (await publicState(anonymousContext, fixture.space_id)).space,
            { timeout: 150_000, intervals: [1_000, 2_000, 5_000] },
          )
          .toBe(true);
      } else {
        result = await publicState(anonymousContext, fixture.space_id);
        expect(result.identity).toBe(true);
        expect(result.space).toBe(
          scenario === "space-revoke" || scenario === "space-expire",
        );
        await detailAndA11y(publicPage, fixture.space_id);
        await expect(
          publicPage.getByText("Identidad verificada", { exact: true }),
        ).toBeVisible();
        if (scenario === "space-revoke")
          await expect(
            publicPage.getByText("Antecedentes del espacio revisados", {
              exact: true,
            }),
          ).toBeVisible();
        await capture(publicPage, "approved");
        if (scenario === "space-revoke") {
          await page
            .getByRole("combobox", { name: "Motivo", exact: true })
            .selectOption("solicitud_titular");
          const revokeResponse = page.waitForResponse(
            (r) =>
              new URL(r.url()).pathname === `${api}/revoke` &&
              r.request().method() === "POST",
          );
          await page
            .getByRole("button", { name: "Revocar", exact: true })
            .click();
          expect((await revokeResponse).status()).toBe(200);
          await expect(
            page.locator('[data-review-case-state="revocada"]'),
          ).toBeVisible();
          result = await publicState(anonymousContext, fixture.space_id);
          expect(result.identity).toBe(true);
          expect(result.space).toBe(false);
          await detailAndA11y(publicPage, fixture.space_id);
          await expect(
            publicPage.getByText("Identidad verificada", { exact: true }),
          ).toBeVisible();
          await expect(
            publicPage.getByText("Antecedentes del espacio revisados", {
              exact: true,
            }),
          ).toHaveCount(0);
          await capture(publicPage, "revoked");
        }
      }
    }

    if (requiredExpiry) {
      stage = "expiry-without-worker";
      await expect
        .poll(
          async () =>
            (await publicState(anonymousContext, fixture.space_id)).space,
          { timeout: 150_000, intervals: [1_000, 2_000, 5_000] },
        )
        .toBe(false);
      result = await publicState(anonymousContext, fixture.space_id);
      expect(result.identity).toBe(true);
      await detailAndA11y(publicPage, fixture.space_id);
      await expect(
        publicPage.getByText("Identidad verificada", { exact: true }),
      ).toBeVisible();
      await expect(
        publicPage.getByText("Antecedentes del espacio revisados", {
          exact: true,
        }),
      ).toHaveCount(0);
      await capture(publicPage, "expired");
    }
    if (scenario === "identity-revoke") {
      stage = "revoke-through-interface";
      await page
        .getByRole("combobox", { name: "Motivo", exact: true })
        .selectOption("solicitud_titular");
      const revokeResponse = page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname === `${api}/revoke` &&
          r.request().method() === "POST",
      );
      await page.getByRole("button", { name: "Revocar", exact: true }).click();
      expect((await revokeResponse).status()).toBe(200);
      await expect(
        page.locator('[data-review-case-state="revocada"]'),
      ).toBeVisible();
      result = await publicState(anonymousContext, fixture.space_id);
      expect(result.identity).toBe(false);
      expect(result.space).toBe(false);
      await detailAndA11y(publicPage, fixture.space_id);
      await expect(
        publicPage.getByText("Identidad verificada", { exact: true }),
      ).toHaveCount(0);
      await expect(
        publicPage.getByText("Antecedentes del espacio revisados", {
          exact: true,
        }),
      ).toHaveCount(0);
      const list = await anonymousContext.request.get(
        `/api/backend/public/spaces?q=${encodeURIComponent(beforeDetail.data.title)}`,
      );
      expect(list.status()).toBe(200);
      const listPayload: unknown = await list.json();
      expect(JSON.stringify(listPayload)).toContain(fixture.space_id);
      await expect(publicPage).toHaveURL(
        new RegExp(`/espacios/${fixture.space_id}$`),
      );
      await capture(publicPage, "revoked");
    }
    expect(state.external).toBe(false);
  } catch {
    console.info("PUBLIC_NATIVE_FAILED_STAGE", stage);
    for (const name of ["Correo electrónico", "Contraseña"]) {
      const field = page.getByLabel(name, { exact: true });
      if (await field.count().catch(() => 0))
        await field.fill("", { timeout: 500 }).catch(() => undefined);
    }
    throw new Error(
      `Native public verification scenario ${scenario} failed; private values suppressed`,
    );
  } finally {
    await reviewerContext.request
      .post("/api/backend/auth/logout", { headers: { Origin: origin } })
      .catch(() => undefined);
    await reviewerContext.close();
    await anonymousContext.close();
  }
});
