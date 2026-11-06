import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { readFile, lstat, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import path from "node:path";

const enabled = process.env.PHASE_DOCUMENT_REVIEW_NATIVE_E2E === "1";
test.skip(
  !enabled,
  "Requires the explicitly authorized synthetic reviewer trial",
);
test.use({ trace: "off", screenshot: "off", video: "off" });
const origin = "https://127.0.0.1:4203";
const destination = "/mi-cuenta/revision-documental";
const backend = "/home/nandev/Documentos/projects/espaciGo";

type Account = { email: string; password: string; account_id: string };
type Fixture = {
  case_id: string;
  file_id: string;
  owner_id: string;
  reviewer_id: string;
  purpose: "identity" | "space";
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
  return JSON.parse(await readFile(selected, "utf8"));
}
async function login(page: Page, account: Account, target: string) {
  await page.goto(`/acceso?return_to=${encodeURIComponent(target)}`);
  await page.getByLabel("Correo electrónico").fill(account.email);
  await page.getByLabel("Contraseña", { exact: true }).fill(account.password);
  const response = page.waitForResponse(
    (r) => new URL(r.url()).pathname === "/api/backend/auth/login",
  );
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  const loginStatus = (await response).status();
  console.info("SYNTHETIC_REVIEW_LOGIN_STATUS", loginStatus);
  expect(loginStatus).toBe(200);
  await expect(page).toHaveURL((url) => url.pathname === target);
}
function assignment(
  purpose: Fixture["purpose"],
  operation: "grant" | "withdraw",
) {
  // Explicit administrative synthetic-fixture preparation, not a UI capability
  // or product assignment-management flow. The helper verifies exact target and
  // reads assignment + immutable audit back after commit.
  const result = execFileSync(
    "python",
    [
      path.join(backend, "scripts/prepare_document_reviewer_trial.py"),
      "--authorized-local-synthetic-only",
      "--purpose",
      purpose,
      "--operation",
      operation,
    ],
    { cwd: backend, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] },
  );
  expect(result.includes("AND_AUDIT_READBACK_PASS")).toBe(true);
}
async function openedPDF(page: Page, fileID: string) {
  const downloadEvent = page.waitForEvent("download");
  await page.locator(`[data-review-file-id="${fileID}"]`).click();
  const download = await downloadEvent;
  const stream = await download.createReadStream();
  if (!stream) throw new Error("No native download stream");
  const chunks: Buffer[] = [];
  for await (const chunk of stream) chunks.push(Buffer.from(chunk));
  const bytes = Buffer.concat(chunks);
  expect(createHash("sha256").update(bytes).digest("hex")).toBe(
    "a4e52a381a299bdd2e0b6ea21bfcaec8cb7c25d1384061d6d760250ccb10d9c4",
  );
  const metadata = execFileSync("pdfinfo", ["-"], {
    input: bytes,
    stdio: ["pipe", "pipe", "pipe"],
  }).toString();
  expect(/^Pages:\s+1$/m.test(metadata)).toBe(true);
  expect(await download.failure()).toBeNull();
}

test("@native read-only document demo and sanitized captures", async ({
  browser,
}) => {
  test.skip(
    process.env.PHASE_DOCUMENT_REVIEW_DEMO_CAPTURE !== "1",
    "Explicit retained demo capture only",
  );
  const root = process.env.PHASE6_PRIVATE_TESTING_DIR;
  const inventoryFile = process.env.PHASE6_ACCOUNT_INVENTORY_FILE;
  const fixtureFile = process.env.PHASE_DOCUMENT_REVIEW_FIXTURE_FILE;
  if (!root || !inventoryFile || !fixtureFile)
    throw Error("Explicit private demo inputs required");
  const policy = await privateInput(
    root,
    path.join(root, "test-account-policy.json"),
  );
  expect(
    policy.document_reviewer_authorization_20261105?.existing_accounts_only,
  ).toBe(true);
  const fixture = (await privateInput(root, fixtureFile)) as Fixture;
  const inventory = (await privateInput(root, inventoryFile)) as {
    accounts: Account[];
  };
  const reviewer = inventory.accounts.find(
    (a) => a.account_id === fixture.reviewer_id,
  );
  if (!reviewer || fixture.scanner !== "real-ClamAV")
    throw Error("Authorized existing reviewer fixture required");
  const context = await browser.newContext({
    baseURL: origin,
    ignoreHTTPSErrors: true,
  });
  const page = await context.newPage();
  let external = false;
  context.on("request", (request) => {
    const url = new URL(request.url());
    if (
      ["http:", "https:", "blob:"].includes(url.protocol) &&
      url.origin !== origin
    )
      external = true;
  });
  try {
    await login(page, reviewer, destination);
    await page.locator(`[data-review-case-id="${fixture.case_id}"]`).click();
    await expect(
      page.locator('[data-review-case-state="enviada"]'),
    ).toBeVisible();
    await openedPDF(page, fixture.file_id);
    for (const [name, viewport] of [
      ["desktop", { width: 1365, height: 900 }],
      ["mobile", { width: 390, height: 844 }],
    ] as const) {
      await page.setViewportSize(viewport);
      await page.evaluate(() => document.fonts.ready);
      const axe = await new AxeBuilder({ page }).analyze();
      expect(axe.violations.map((v) => v.id)).toEqual([]);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      await page.screenshot({
        path: path.resolve(
          "..",
          "planning/frontend_next/evidence",
          `reviewer-pending-${name}-20261105.png`,
        ),
        fullPage: true,
      });
    }
    expect(external).toBe(false);
    const metadata = await page.request.get(
      `/api/backend/document-verification/reviewer/cases/${fixture.case_id}`,
    );
    expect(metadata.status()).toBe(200);
    expect((await metadata.json()).state).toBe("enviada");
  } catch {
    for (const name of ["Correo electrónico", "Contraseña"]) {
      const field = page.getByLabel(name, { exact: true });
      if (await field.count().catch(() => 0))
        await field.fill("", { timeout: 500 }).catch(() => undefined);
    }
    throw Error(
      "Retained synthetic reviewer demo failed; private values suppressed",
    );
  } finally {
    await context.request
      .post("/api/backend/auth/logout", { headers: { Origin: origin } })
      .catch(() => undefined);
    await context.close();
  }
});

test("@native @document-review assigned clean PDF, scope denial, withdrawal, decision and revocation", async ({
  browser,
}) => {
  const root = process.env.PHASE6_PRIVATE_TESTING_DIR;
  const inventoryFile = process.env.PHASE6_ACCOUNT_INVENTORY_FILE;
  const fixtureFile = process.env.PHASE_DOCUMENT_REVIEW_FIXTURE_FILE;
  if (!root || !inventoryFile || !fixtureFile)
    throw new Error("Explicit private synthetic inputs required");
  const policy = await privateInput(
    root,
    path.join(root, "test-account-policy.json"),
  );
  expect(
    Boolean(
      policy.document_reviewer_authorization_20261105?.existing_accounts_only,
    ),
  ).toBe(true);
  const fixture = (await privateInput(root, fixtureFile)) as Fixture;
  expect(fixture.scanner === "real-ClamAV").toBe(true);
  const inventory = (await privateInput(root, inventoryFile)) as {
    accounts: Account[];
  };
  const owner = inventory.accounts.find(
    (a) => a.account_id === fixture.owner_id,
  );
  const reviewer = inventory.accounts.find(
    (a) => a.account_id === fixture.reviewer_id,
  );
  if (!owner || !reviewer || owner.account_id === reviewer.account_id)
    throw new Error("Existing two distinct accounts required");
  const context = await browser.newContext({
    baseURL: origin,
    ignoreHTTPSErrors: true,
  });
  const ownerContext = await browser.newContext({
    baseURL: origin,
    ignoreHTTPSErrors: true,
  });
  const page = await context.newPage();
  const ownerPage = await ownerContext.newPage();
  let external = false,
    stage = "login";
  for (const c of [context, ownerContext])
    c.on("request", (request) => {
      const url = new URL(request.url());
      if (
        ["http:", "https:", "blob:"].includes(url.protocol) &&
        url.origin !== origin
      )
        external = true;
    });
  const base = `/api/backend/document-verification/reviewer/cases/${fixture.case_id}`;
  const content = `${base}/files/${fixture.file_id}/content`;
  try {
    await login(page, reviewer, destination);
    await login(ownerPage, owner, "/mi-cuenta");
    await expect(
      page.getByRole("heading", { name: "Revisión documental", exact: true }),
    ).toBeVisible();
    console.info("REVIEWER_NATIVE_STAGE", (stage = "owner-self-access-denial"));
    expect((await ownerPage.request.get(base)).status()).toBe(404);
    expect((await ownerPage.request.get(content)).status()).toBe(404);
    console.info(
      "REVIEWER_NATIVE_STAGE",
      (stage = "assigned-case-open-and-real-pdf"),
    );
    await page.locator(`[data-review-case-id="${fixture.case_id}"]`).click();
    await expect(
      page.locator('[data-review-case-state="enviada"]'),
    ).toBeVisible();
    await openedPDF(page, fixture.file_id);
    const beforeResponse = await page.request.get(base);
    expect(beforeResponse.status()).toBe(200);
    const before = await beforeResponse.json();
    const until = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString();
    const review = {
      capability: `review_${fixture.purpose}`,
      decision: "approve",
      expected_revision: Number(before.revision),
      verified_until: until,
    };
    if (fixture.purpose === "space") {
      console.info(
        "REVIEWER_NATIVE_STAGE",
        (stage = "same-owner-same-capability-outside-space-scope-denial"),
      );
      const outside = await privateInput(
        root,
        path.join(root, "space-gallery-demo/reviewer-outside-scope.json"),
      );
      const other = `/api/backend/document-verification/reviewer/cases/${outside.case_id}`;
      expect((await page.request.get(other)).status()).toBe(404);
      expect(
        (
          await page.request.get(`${other}/files/${outside.file_id}/content`)
        ).status(),
      ).toBe(404);
      const denial = await page.request.post(`${other}/review`, {
        headers: { Origin: origin },
        data: { ...review, expected_revision: Number(outside.revision) },
      });
      expect([403, 404].includes(denial.status())).toBe(true);
    }
    console.info(
      "REVIEWER_NATIVE_STAGE",
      (stage = "assignment-withdrawal-before-decision"),
    );
    assignment(fixture.purpose, "withdraw");
    await page.locator(`[data-review-file-id="${fixture.file_id}"]`).click();
    await expect(page.getByRole("alert")).toBeVisible();
    await expect(page.locator("[data-review-case-state]")).toHaveCount(0);
    expect((await page.request.get(base)).status()).toBe(404);
    expect((await page.request.get(content)).status()).toBe(404);
    for (const [action, data] of [
      ["review", review],
      [
        "revoke",
        {
          purpose: fixture.purpose,
          reason_code: "solicitud_titular",
          expected_revision: Number(before.revision),
        },
      ],
    ] as const) {
      const denied = await page.request.post(`${base}/${action}`, {
        headers: { Origin: origin },
        data,
      });
      expect([403, 404].includes(denied.status())).toBe(true);
    }
    console.info(
      "REVIEWER_NATIVE_STAGE",
      (stage = "explicit-reassignment-and-unchanged-denied-state"),
    );
    assignment(fixture.purpose, "grant");
    await page.reload();
    await page.locator(`[data-review-case-id="${fixture.case_id}"]`).click();
    const unchanged = await (await page.request.get(base)).json();
    expect(
      unchanged.state === "enviada" && unchanged.revision === before.revision,
    ).toBe(true);
    console.info(
      "REVIEWER_NATIVE_STAGE",
      (stage = "approve-in-interface-with-post-readback"),
    );
    await page
      .getByRole("combobox", { name: "Decisión", exact: true })
      .selectOption("approve");
    await page.getByLabel("Vigencia hasta").fill(until.slice(0, 16));
    try {
      await expect(page.getByLabel("Vigencia hasta")).toHaveValue(
        until.slice(0, 16),
      );
      await expect(
        page.getByRole("button", { name: "Confirmar decisión", exact: true }),
      ).toBeEnabled({ timeout: 3000 });
    } catch {
      console.info("SYNTHETIC_APPROVAL_VALIDITY_DIAGNOSTIC", {
        documentExpiresAt: unchanged.document_expires_at,
        requestedExpiry: until,
        input: await page.getByLabel("Vigencia hasta").evaluate((node) => {
          const input = node as HTMLInputElement;
          return {
            value: input.value,
            valid: input.validity.valid,
            validationMessage: input.validationMessage,
          };
        }),
      });
      throw new Error("Synthetic expiry did not enable decision");
    }
    const approvedEvent = page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname === `${base}/review` &&
        r.request().method() === "POST",
    );
    await page
      .getByRole("button", { name: "Confirmar decisión", exact: true })
      .click();
    expect((await approvedEvent).status()).toBe(200);
    await expect(
      page.locator('[data-review-case-state="aprobada"]'),
    ).toBeVisible();
    const approved = await (await page.request.get(base)).json();
    expect(
      approved.state === "aprobada" &&
        Number(approved.revision) > Number(before.revision),
    ).toBe(true);
    if (fixture.purpose === "space") {
      console.info(
        "REVIEWER_NATIVE_STAGE",
        (stage = "revoke-approved-case-in-interface"),
      );
      await page
        .getByRole("combobox", { name: "Motivo", exact: true })
        .selectOption("solicitud_titular");
      const revokedEvent = page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname === `${base}/revoke` &&
          r.request().method() === "POST",
      );
      await page.getByRole("button", { name: "Revocar", exact: true }).click();
      expect((await revokedEvent).status()).toBe(200);
      await expect(
        page.locator('[data-review-case-state="revocada"]'),
      ).toBeVisible();
      const revoked = await (await page.request.get(base)).json();
      expect(
        revoked.state === "revocada" &&
          Number(revoked.revision) > Number(approved.revision),
      ).toBe(true);
    }
    console.info(
      "REVIEWER_NATIVE_STAGE",
      (stage = "desktop-mobile-accessibility"),
    );
    for (const viewport of [
      { width: 1440, height: 1000 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(viewport);
      const result = await new AxeBuilder({ page }).analyze();
      expect(result.violations.length).toBe(0);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
      ).toBe(true);
    }
    expect(external).toBe(false);
    await writeFile(
      path.join(
        root,
        `space-gallery-demo/reviewer-native-${fixture.purpose}-readback.json`,
      ),
      JSON.stringify(
        {
          purpose: fixture.purpose,
          actual_browser_bff_go: true,
          real_pdf_stream_parser: true,
          owner_self_denied: true,
          withdrawn_read_write_denied: true,
          denied_revision_unchanged: true,
          approved: true,
          revoked: fixture.purpose === "space",
          axe_desktop_mobile: true,
          same_origin: true,
        },
        null,
        2,
      ),
      { mode: 0o600 },
    );
  } catch {
    for (const p of [page, ownerPage])
      for (const name of ["Correo electrónico", "Contraseña"]) {
        if (p.isClosed()) continue;
        const field = p.getByLabel(name, { exact: true });
        if (await field.count().catch(() => 0))
          await field.fill("", { timeout: 500 }).catch(() => undefined);
      }
    throw new Error(
      `Native synthetic reviewer failed at ${stage}; private values suppressed`,
    );
  } finally {
    for (const c of [context, ownerContext]) {
      await c.request
        .post("/api/backend/auth/logout", { headers: { Origin: origin } })
        .catch(() => undefined);
      await c.close();
    }
  }
});
