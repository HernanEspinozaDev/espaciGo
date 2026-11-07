import { expect, test, type Page } from "@playwright/test";
import { chmod, open, readFile, rename, stat } from "node:fs/promises";
import { randomUUID } from "node:crypto";
import path from "node:path";

const enabled = process.env.ESPACIGO_WIZARD_NATIVE === "1";
const baseURL = "https://127.0.0.1:4206";
const privateDir = path.resolve(
  process.cwd(),
  "../.local/testing/wizard-improvements",
);
const inventoryPath = path.join(
  privateDir,
  "wizard-improvements-account-inventory.json",
);
const intentPath = path.join(
  privateDir,
  "native-wizard-editor-schedule-intent-v2.json",
);
const spaceId = process.env.ESPACIGO_WIZARD_EDITOR_SPACE_ID;
const originalClose = "18:00";
const changedClose = "17:30";

test.skip(
  !enabled,
  "Opt-in native local existing-draft schedule CAS trial only",
);
test.setTimeout(90_000);
test.use({ trace: "off", screenshot: "off", video: "off" });

type Account = { email?: unknown; password?: unknown };
type Period = { open: string; close: string };
type Day = { weekday: number; periods: Period[] };
type WeeklyHours = { enabled: boolean; time_zone: string; days: Day[] };
type WeeklyHoursPutBody = Pick<WeeklyHours, "enabled" | "days">;

async function login(page: Page, account: Account): Promise<void> {
  await page.goto("/acceso");
  try {
    const responsePromise = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === "/api/backend/auth/login" &&
        response.request().method() === "POST",
      { timeout: 15_000 },
    );
    await page.getByLabel("Correo electrónico").fill(account.email as string);
    await page
      .getByLabel("Contraseña", { exact: true })
      .fill(account.password as string);
    const [response] = await Promise.all([
      responsePromise,
      page
        .getByRole("button", { name: "Iniciar sesión" })
        .click({ noWaitAfter: true }),
    ]);
    if (response.status() !== 200) throw new Error("login status rejected");
  } catch {
    await page
      .getByLabel("Correo electrónico")
      .fill("")
      .catch(() => undefined);
    await page
      .getByLabel("Contraseña", { exact: true })
      .fill("")
      .catch(() => undefined);
    throw new Error("Native editor login failed; private form values cleared");
  }
}

test("@native edits the existing hourly draft schedule with CAS and rejects a stale write", async ({
  page,
}) => {
  if (
    !spaceId ||
    !/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(
      spaceId,
    )
  ) {
    throw new Error(
      "Select the verified existing hourly draft via ESPACIGO_WIZARD_EDITOR_SPACE_ID",
    );
  }
  const directory = await stat(privateDir);
  if ((directory.mode & 0o777) !== 0o700)
    throw new Error("Private trial directory must be mode 0700");
  const inventoryStat = await stat(inventoryPath);
  if ((inventoryStat.mode & 0o777) !== 0o600)
    throw new Error("Private account inventory must be mode 0600");
  const accountList = JSON.parse(
    await readFile(inventoryPath, "utf8"),
  ) as Account[];
  const account = accountList[0];
  if (
    typeof account?.email !== "string" ||
    typeof account.password !== "string"
  ) {
    throw new Error("Private verified-account inventory is incomplete");
  }

  await login(page, account);
  await page.goto(`/anfitrion/espacios/${spaceId}/editar`);
  await expect(page.locator("#editor-title")).toBeVisible();
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect(
    page.getByRole("heading", { name: "Disponibilidad semanal" }),
  ).toBeVisible();

  const preHoursResponse = await page.request.get(
    `/api/backend/spaces/${spaceId}/weekly-hours`,
  );
  expect(preHoursResponse.status()).toBe(200);
  const preHours = (await preHoursResponse.json()).data as WeeklyHours;
  const preEtag = preHoursResponse.headers()["etag"];
  expect(preEtag).toBeTruthy();
  expect(preHours.enabled).toBe(true);
  expect(preHours.time_zone).toBe("America/Santiago");
  expect(preHours.days).toHaveLength(7);
  const monday = preHours.days.find((day) => day.weekday === 1);
  expect(monday?.periods).toEqual([{ open: "09:00", close: originalClose }]);
  const originalDays = structuredClone(preHours.days);
  const preDetailResponse = await page.request.get(
    `/api/backend/spaces/${spaceId}`,
  );
  expect(preDetailResponse.status()).toBe(200);
  const preDetailEtag = preDetailResponse.headers()["etag"];
  expect(preDetailEtag).toBeTruthy();

  const mutations: Array<{
    method: string;
    path: string;
    ifMatch: string | null;
    body: unknown;
  }> = [];
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (
      url.origin !== new URL(baseURL).origin ||
      url.pathname.startsWith("/api/backend/auth/")
    )
      return;
    if (
      url.pathname === `/api/backend/spaces/${spaceId}/weekly-hours` &&
      request.method() === "PUT"
    ) {
      mutations.push({
        method: request.method(),
        path: url.pathname,
        ifMatch: request.headers()["if-match"] ?? null,
        body: request.postDataJSON(),
      });
    }
  });

  await expect(
    page.getByLabel("Tramo 1: hora de cierre (24:00 permitido)"),
  ).toHaveValue(originalClose);
  await page
    .getByLabel("Tramo 1: hora de cierre (24:00 permitido)")
    .fill(changedClose);

  // Intent is created only after the authenticated read and UI preflight succeed.
  const handle = await open(intentPath, "wx", 0o600).catch(() => {
    throw new Error(
      "Editor schedule intent already exists; reconcile manually, never replay",
    );
  });
  try {
    await handle.writeFile(
      JSON.stringify({
        spaceId,
        intent: "edit-existing-weekly-hours",
        state: "prepared",
        expectedBefore: { monday: { open: "09:00", close: originalClose } },
        target: { monday: { open: "09:00", close: changedClose } },
      }),
    );
    await handle.sync();
  } finally {
    await handle.close();
  }
  await chmod(intentPath, 0o600);

  const timeZoneSave = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname ===
        `/api/backend/spaces/${spaceId}/availability` &&
      response.request().method() === "PUT",
  );
  const weeklySave = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname ===
        `/api/backend/spaces/${spaceId}/weekly-hours` &&
      response.request().method() === "PUT",
  );
  await page.getByRole("button", { name: "Guardar horarios" }).click();
  const [timeZoneResponse, saveResponse] = await Promise.all([
    timeZoneSave,
    weeklySave,
  ]);
  expect(timeZoneResponse.status()).toBe(200);
  const configuredEtag = timeZoneResponse.headers()["etag"];
  expect(configuredEtag).toBeTruthy();
  // Time-zone updates may be idempotent and preserve the detail ETag.

  expect(saveResponse.status()).toBe(200);
  expect(saveResponse.headers()["etag"]).not.toBe(configuredEtag);
  await expect(
    page.getByText("Horario semanal habitual y zona horaria guardados."),
  ).toBeVisible();
  expect(mutations).toHaveLength(1);
  expect(mutations[0]?.ifMatch).toBe(configuredEtag);
  expect(mutations[0]?.ifMatch).not.toBe(preEtag);

  const postHoursResponse = await page.request.get(
    `/api/backend/spaces/${spaceId}/weekly-hours`,
  );
  expect(postHoursResponse.status()).toBe(200);
  const postHours = (await postHoursResponse.json()).data as WeeklyHours;
  const expectedDays = structuredClone(originalDays);
  const expectedMonday = expectedDays.find((day) => day.weekday === 1);
  if (!expectedMonday)
    throw new Error("Monday missing from authoritative schedule");
  expectedMonday.periods = [{ open: "09:00", close: changedClose }];
  expect(postHours.days).toEqual(expectedDays);
  const postEtag = postHoursResponse.headers()["etag"];
  expect(postEtag).toBeTruthy();
  expect(postEtag).toBe(saveResponse.headers()["etag"]);
  expect(postEtag).not.toBe(preEtag);

  const detailResponse = await page.request.get(
    `/api/backend/spaces/${spaceId}`,
  );
  expect(detailResponse.status()).toBe(200);
  expect(detailResponse.headers()["etag"]).not.toBe(preDetailEtag);

  await page.reload();
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect(
    page.getByLabel("Tramo 1: hora de cierre (24:00 permitido)"),
  ).toHaveValue(changedClose);

  // Explicit stale CAS probe through the HTTPS browser origin; never auto-replay.
  const staleResponse = await page.request.put(
    `/api/backend/spaces/${spaceId}/weekly-hours`,
    {
      headers: { Origin: baseURL, "If-Match": preEtag! },
      data: {
        enabled: preHours.enabled,
        days: originalDays,
      } satisfies WeeklyHoursPutBody,
    },
  );
  expect(staleResponse.status()).toBe(412);
  expect(mutations).toHaveLength(1);
  const afterStaleResponse = await page.request.get(
    `/api/backend/spaces/${spaceId}/weekly-hours`,
  );
  expect(afterStaleResponse.status()).toBe(200);
  expect(((await afterStaleResponse.json()).data as WeeklyHours).days).toEqual(
    expectedDays,
  );
});

test("@native reconciles the saved hourly schedule and rejects stale CAS", async ({
  page,
}) => {
  const directory = await stat(privateDir);
  expect(directory.mode & 0o777).toBe(0o700);
  expect((await stat(intentPath)).mode & 0o777).toBe(0o600);
  const receipt = JSON.parse(await readFile(intentPath, "utf8")) as {
    spaceId: string;
    intent: string;
    state: string;
    observedAvailabilityETag: string;
    observedReadback: { monday: Period };
    flags?: Record<string, boolean>;
  };
  expect(["saved-needs-reconciliation", "verified"]).toContain(receipt.state);
  expect(receipt.intent).toBe("edit-existing-weekly-hours");
  expect(receipt.spaceId).toMatch(/^[0-9a-f-]{36}$/i);
  expect(receipt.observedAvailabilityETag).toMatch(/^"[^"\\]+"$/);
  expect(receipt.observedReadback.monday).toEqual({
    open: "09:00",
    close: changedClose,
  });
  const accounts = JSON.parse(
    await readFile(inventoryPath, "utf8"),
  ) as Account[];
  const account = accounts[0];
  if (
    typeof account?.email !== "string" ||
    typeof account.password !== "string"
  )
    throw new Error("Private verified-account inventory is incomplete");
  await login(page, account);
  const weeklyPath = `/api/backend/spaces/${receipt.spaceId}/weekly-hours`;
  const weeklyResponse = await page.request.get(weeklyPath);
  expect(weeklyResponse.status()).toBe(200);
  const weekly = (await weeklyResponse.json()).data as WeeklyHours;
  expect(weekly.enabled).toBe(true);
  expect(weekly.time_zone).toBe("America/Santiago");
  expect(weekly.days).toHaveLength(7);
  const expectedDays: Day[] = [1, 2, 3, 4, 5, 6, 7].map((weekday) => ({
    weekday,
    periods:
      weekday === 1 || (weekday >= 2 && weekday <= 5)
        ? [{ open: "09:00", close: weekday === 1 ? changedClose : "18:00" }]
        : [],
  }));
  expect(weekly.days).toEqual(expectedDays);
  const detailResponse = await page.request.get(
    `/api/backend/spaces/${receipt.spaceId}`,
  );
  expect(detailResponse.status()).toBe(200);
  const weeklyPuts: Array<{ ifMatch: string | null }> = [];
  page.on("request", (request) => {
    if (
      request.method() === "PUT" &&
      new URL(request.url()).pathname === weeklyPath
    )
      weeklyPuts.push({ ifMatch: request.headers()["if-match"] ?? null });
  });
  // APIRequestContext probe is separate from browser traffic; assert no UI replay.
  expect(weeklyPuts).toHaveLength(0);
  await page.goto(`/anfitrion/espacios/${receipt.spaceId}/editar`);
  await expect(page.locator("#editor-title")).toBeVisible();
  await page.getByRole("tab", { name: /horarios/i }).click();
  const closingInput = page.getByLabel(
    "Tramo 1: hora de cierre (24:00 permitido)",
  );
  await expect(closingInput).toHaveValue(changedClose);
  await page.reload();
  await page.getByRole("tab", { name: /horarios/i }).click();
  await expect(closingInput).toHaveValue(changedClose);

  const currentEtag = weeklyResponse.headers()["etag"];
  if (!currentEtag) throw new Error("Weekly schedule readback has no ETag");
  if (receipt.observedAvailabilityETag === currentEtag)
    throw new Error(
      "Observed prior availability ETag is not stale; CAS probe withheld",
    );
  const staleBody: WeeklyHoursPutBody = {
    enabled: weekly.enabled,
    days: expectedDays.map((day) => ({
      weekday: day.weekday,
      periods: day.periods.map((period) => ({ ...period })),
    })),
  };
  const monday = staleBody.days.find((day) => day.weekday === 1);
  if (!monday) throw new Error("Monday missing from current weekly schedule");
  monday.periods = [{ open: "09:00", close: originalClose }];
  const staleResponse = await page.request.put(weeklyPath, {
    headers: { Origin: baseURL, "If-Match": receipt.observedAvailabilityETag },
    data: staleBody,
  });
  expect(staleResponse.status()).toBe(412);
  expect(weeklyPuts).toHaveLength(0);
  const finalReadback = await page.request.get(weeklyPath);
  expect(finalReadback.status()).toBe(200);
  expect(((await finalReadback.json()).data as WeeklyHours).days).toEqual(
    expectedDays,
  );
  expect(weeklyPuts).toHaveLength(0);

  const updated = {
    ...receipt,
    state: "verified",
    flags: { readback: true, reload: true, stale412: true },
  };
  const temporaryPath = `${intentPath}.${randomUUID()}.tmp`;
  const handle = await open(temporaryPath, "wx", 0o600);
  try {
    await handle.writeFile(JSON.stringify(updated));
    await handle.sync();
  } finally {
    await handle.close();
  }
  await chmod(temporaryPath, 0o600);
  await rename(temporaryPath, intentPath);
  const dirHandle = await open(privateDir, "r");
  try {
    await dirHandle.sync();
  } finally {
    await dirHandle.close();
  }
});
