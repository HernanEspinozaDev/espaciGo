import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import path from "node:path";

const nativeBaseURL =
  process.env.PHASE7_NATIVE_BASE_URL ?? "https://127.0.0.1:4205";
const mailpitBaseURL = "http://127.0.0.1:11080";

test.use({
  trace: "off",
  screenshot: "off",
  video: "off",
  baseURL: nativeBaseURL,
  ignoreHTTPSErrors: nativeBaseURL.startsWith("https://"),
});
test.describe.configure({ mode: "serial" });

test.skip(
  process.env.PHASE7_NATIVE_BOOTSTRAP !== "1",
  "Requires the authorized loopback Next/BFF, Go API, Mailpit, and private synthetic account inventory",
);

function privatePath(name: string): string {
  return path.resolve(process.cwd(), "../.local/testing", name);
}

type PrivateAccount = {
  email?: string;
  password?: string;
  status?: string;
};

type MailpitMessage = Record<string, unknown>;

function mailpitId(message: MailpitMessage): string | null {
  const id = message.ID ?? message.id;
  return typeof id === "string" ? id : null;
}

function mailpitRecipients(message: MailpitMessage): string[] {
  const raw = message.To ?? message.to;
  if (!Array.isArray(raw)) return [];
  return raw.flatMap((recipient) => {
    if (typeof recipient === "string") return [recipient.toLowerCase()];
    if (typeof recipient !== "object" || recipient === null) return [];
    const record = recipient as Record<string, unknown>;
    const address = record.Address ?? record.address;
    return typeof address === "string" ? [address.toLowerCase()] : [];
  });
}

async function mailpitMessages(page: import("@playwright/test").Page) {
  const response = await page.request.get(`${mailpitBaseURL}/api/v1/messages`);
  if (response.status() !== 200) throw new Error("Mailpit list unavailable");
  const payload = (await response.json()) as { messages?: MailpitMessage[] };
  return Array.isArray(payload.messages) ? payload.messages : [];
}

async function verificationCredential(
  page: import("@playwright/test").Page,
  email: string,
  priorIds: Set<string>,
): Promise<{ token_id: string; token: string }> {
  for (let attempt = 0; attempt < 30; attempt += 1) {
    const messages = await mailpitMessages(page);
    const message = messages.find((item) => {
      const id = mailpitId(item);
      return (
        id !== null &&
        !priorIds.has(id) &&
        mailpitRecipients(item).includes(email.toLowerCase())
      );
    });
    const id = message && mailpitId(message);
    if (id) {
      const response = await page.request.get(
        `${mailpitBaseURL}/api/v1/message/${encodeURIComponent(id)}`,
      );
      if (response.status() !== 200)
        throw new Error("Mailpit message unavailable");
      const detail: unknown = await response.json();
      const record = detail as Record<string, unknown>;
      const body = typeof record.Text === "string" ? record.Text : "";
      const tokenId = /^Token ID:\s*(\S+)\s*$/m.exec(body)?.[1];
      const token = /^Token:\s*(\S+)\s*$/m.exec(body)?.[1];
      if (tokenId && token) return { token_id: tokenId, token };
    }
    await page.waitForTimeout(500);
  }
  throw new Error("No synthetic verification credential arrived in Mailpit");
}

async function logoutThroughUi(page: import("@playwright/test").Page) {
  await expect(page).toHaveURL(/\/mi-cuenta$/);
  const logoutButton = page.getByRole("button", { name: "Cerrar sesión" });
  await expect(logoutButton).toBeVisible();
  const logoutPromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/backend/auth/logout" &&
      response.request().method() === "POST",
    { timeout: 15_000 },
  );
  await logoutButton.click({ noWaitAfter: true });
  const logout = await logoutPromise;
  expect(logout.status()).toBe(204);

  const sessionPage = await page.context().newPage();
  try {
    await sessionPage.goto(`${nativeBaseURL}/api/health`, {
      waitUntil: "domcontentloaded",
      timeout: 10_000,
    });
    const sessionStatus = await sessionPage.evaluate(async () => {
      const response = await fetch("/api/backend/auth/session", {
        headers: { Accept: "application/json" },
      });
      return response.status;
    });
    expect(sessionStatus).toBe(401);
  } finally {
    await sessionPage.close();
  }
}

test("@phase7 @native bootstraps only the authorized synthetic accounts through normal registration and verification", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const policy = JSON.parse(
    await readFile(privatePath("test-account-policy.json"), "utf8"),
  ) as {
    account_policy?: {
      max_new_accounts?: number;
      normal_registration_and_explicit_verification_only?: boolean;
      never_print_credentials_or_tokens?: boolean;
      synthetic_email_domain?: string;
    };
  };
  const inventory = JSON.parse(
    await readFile(privatePath("phase3-account-inventory.json"), "utf8"),
  ) as { accounts?: PrivateAccount[] };
  const accountPolicy = policy.account_policy;
  const accounts = inventory.accounts ?? [];
  expect(
    accountPolicy?.normal_registration_and_explicit_verification_only,
  ).toBe(true);
  expect(accountPolicy?.never_print_credentials_or_tokens).toBe(true);
  expect(accountPolicy?.synthetic_email_domain).toBe("example.invalid");
  expect(accountPolicy?.max_new_accounts).toBeGreaterThanOrEqual(
    accounts.length,
  );
  expect(accounts).toHaveLength(2);

  for (const account of accounts) {
    if (
      typeof account.email !== "string" ||
      typeof account.password !== "string" ||
      account.status !== "verified" ||
      !account.email.endsWith("@example.invalid")
    ) {
      throw new Error("Private synthetic account inventory is not authorized");
    }

    await page.goto("/acceso");
    const loginEmail = page.getByLabel("Correo electrónico");
    const loginPassword = page.getByLabel("Contraseña", { exact: true });
    await loginEmail.fill(account.email);
    await loginPassword.fill(account.password);
    const loginPromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/auth/login" &&
        response.request().method() === "POST",
    );
    const [, login] = await Promise.all([
      page
        .getByRole("button", { name: "Iniciar sesión" })
        .click({ noWaitAfter: true }),
      loginPromise,
    ]);
    await loginEmail.fill("", { timeout: 500 }).catch(() => undefined);
    await loginPassword.fill("", { timeout: 500 }).catch(() => undefined);
    if (login.status() === 200) {
      await logoutThroughUi(page);
      continue;
    }
    const loginPayload = (await login.json().catch(() => null)) as {
      error?: { code?: unknown };
    } | null;
    const loginCode =
      typeof loginPayload?.error?.code === "string" &&
      /^[a-z0-9_]{1,48}$/.test(loginPayload.error.code)
        ? loginPayload.error.code
        : "";
    if (
      login.status() !== 401 &&
      !(login.status() === 403 && loginCode === "email_unverified")
    ) {
      throw new Error("Synthetic account login was not eligible for bootstrap");
    }

    const previousMessages = await mailpitMessages(page);
    const previousIds = new Set(
      previousMessages.flatMap((message) => {
        const id = mailpitId(message);
        return id ? [id] : [];
      }),
    );

    const existingVerification = previousMessages.some((message) =>
      mailpitRecipients(message).includes(account.email!.toLowerCase()),
    );
    if (loginCode === "email_unverified" && !existingVerification)
      throw new Error(
        "Pending synthetic account has no local verification message",
      );
    if (login.status() === 401 && existingVerification)
      throw new Error("Synthetic account credentials need owner review");
    if (!existingVerification) {
      await page.goto("/registro");
      await page.getByLabel(/Términos y condiciones/i).check();
      await page.getByLabel(/Política de privacidad/i).check();
      await page.getByLabel("Correo electrónico").fill(account.email);
      await page
        .getByLabel("Contraseña", { exact: true })
        .fill(account.password);
      const registrationPromise = page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname === "/api/backend/auth/register" &&
          response.request().method() === "POST",
      );
      const [, registration] = await Promise.all([
        page.getByRole("button", { name: "Crear cuenta" }).click(),
        registrationPromise,
      ]);
      await page
        .getByLabel("Correo electrónico")
        .fill("")
        .catch(() => undefined);
      await page
        .getByLabel("Contraseña", { exact: true })
        .fill("")
        .catch(() => undefined);
      expect(registration.status()).toBe(201);
      await expect(page).toHaveURL(/\/registro-exitoso$/);
      await expect(
        page.getByRole("heading", { name: /creada con éxito/i }),
      ).toBeVisible();
    }

    const credential = await verificationCredential(
      page,
      account.email,
      existingVerification ? new Set() : previousIds,
    );
    const verificationStatus = await page.evaluate(async (body) => {
      const response = await fetch("/api/backend/auth/verification", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      return response.status;
    }, credential);
    credential.token_id = "";
    credential.token = "";
    expect(verificationStatus).toBe(204);

    await page.goto("/acceso");
    await page.getByLabel("Correo electrónico").fill(account.email);
    await page.getByLabel("Contraseña", { exact: true }).fill(account.password);
    const verifiedLoginPromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/auth/login" &&
        response.request().method() === "POST",
    );
    const [, verifiedLogin] = await Promise.all([
      page
        .getByRole("button", { name: "Iniciar sesión" })
        .click({ noWaitAfter: true }),
      verifiedLoginPromise,
    ]);
    await page
      .getByLabel("Correo electrónico")
      .fill("", { timeout: 500 })
      .catch(() => undefined);
    await page
      .getByLabel("Contraseña", { exact: true })
      .fill("", { timeout: 500 })
      .catch(() => undefined);
    expect(verifiedLogin.status()).toBe(200);
    await logoutThroughUi(page);
  }
});
