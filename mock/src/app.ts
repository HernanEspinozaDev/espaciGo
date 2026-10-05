interface MockConfig { apiReadyURL: string; }
interface APIError { error?: { code: string; message: string; request_id: string }; }
const statusElement = document.querySelector<HTMLElement>("#api-status")!;
const resultElement = document.querySelector<HTMLElement>("#result")!;
let apiBase = "";
let sessionToken = "";
let termIDs: string[] = [];

async function request(path: string, method = "GET", body?: unknown, authenticated = false, idempotencyKey?: string): Promise<Record<string, unknown>> {
  if (!apiBase) throw new Error("API local aún no disponible.");
  const headers: Record<string,string> = {Accept: "application/json"};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (authenticated) {
    if (!sessionToken) throw new Error("Primero inicia sesión.");
    headers.Authorization = `Bearer ${sessionToken}`;
  }
  if (idempotencyKey) headers["Idempotency-Key"] = idempotencyKey;
  const endpoint = path.startsWith("/") ? `${apiBase}${path}` : `${apiBase}/api/v1/auth/${path}`;
  const response = await fetch(endpoint, {method, headers, body: body === undefined ? undefined : JSON.stringify(body), mode: "cors", cache: "no-store", credentials: "omit"});
  const data = response.status === 204 ? {} : await response.json() as Record<string,unknown> & APIError;
  if (!response.ok) {
    if (authenticated && (response.status === 401 || path === "password/change" && response.status === 503)) sessionToken = "";
    const error = data as APIError;
    throw new Error(`${error.error?.message ?? "Error de API"} (HTTP ${response.status}, ${error.error?.code ?? "unknown"})`);
  }
  return data;
}
async function action(work: () => Promise<void>): Promise<void> {
  const buttons = document.querySelectorAll<HTMLButtonElement>("button");
  buttons.forEach(button => button.disabled = true);
  try { await work(); } catch (error) { resultElement.textContent = error instanceof Error ? error.message : "No se pudo conectar con la API."; }
  finally { buttons.forEach(button => button.disabled = false); }
}
function form(id: string, work: (data: FormData, element: HTMLFormElement) => Promise<void>): void {
  const element = document.querySelector<HTMLFormElement>(`#${id}`)!;
  element.addEventListener("submit", event => { event.preventDefault(); void action(() => work(new FormData(element), element)); });
}
form("register-form", async (data, element) => {
  if (!termIDs.length || !data.get("terms")) throw new Error("Debes aceptar los términos de prueba.");
  const response = await request("register", "POST", {email: data.get("email"), password: data.get("password"), terms_version_ids: termIDs});
  element.querySelector<HTMLInputElement>('[name="password"]')!.value = "";
  resultElement.textContent = response.status === "verification_pending" ? String(response.message) : "Cuenta creada. Abre el buzón de desarrollo para verificar el correo.";
});
form("verify-form", async (data, element) => {
  await request("verification", "POST", {token_id: data.get("token_id"), token: data.get("token")});
  element.reset(); resultElement.textContent = "Correo verificado. Ya puedes iniciar sesión.";
});
form("reissue-form", async data => { await request("verification/reissue", "POST", {email: data.get("email")}); resultElement.textContent = "Verificación reenviada al buzón local; el token anterior queda invalidado."; });
form("login-form", async (data, element) => {
  const response = await request("login", "POST", {email: data.get("email"), password: data.get("password")});
  sessionToken = String(response.access_token); element.querySelector<HTMLInputElement>('[name="password"]')!.value = "";
  await loadSpaceCategories();
  resultElement.textContent = "Sesión iniciada. Puedes consultarla o cerrarla.";
});
form("recovery-request-form", async (data, element) => {
  await request("password/recovery", "POST", {email:data.get("email")});
  element.reset(); resultElement.textContent = "Si la cuenta es elegible, recibirás instrucciones en el buzón local.";
});
form("recovery-consume-form", async (data, element) => {
  await request("password/recovery/consume", "POST", {token_id:data.get("token_id"), token:data.get("token"), new_password:data.get("new_password"), confirm_password:data.get("confirm_password")});
  sessionToken = ""; element.reset(); resultElement.textContent = "Contraseña actualizada y sesiones cerradas. Inicia sesión con la nueva contraseña.";
});
form("password-change-form", async (data, element) => {
  await request("password/change", "POST", {current_password:data.get("current_password"), new_password:data.get("new_password"), confirm_password:data.get("confirm_password")}, true);
  sessionToken = ""; element.reset(); document.querySelector("#session-output")!.textContent = "Sesión revocada por cambio de contraseña.";
  resultElement.textContent = "Contraseña actualizada. Inicia sesión otra vez; se notificó al buzón local.";
});
document.querySelector("#session-button")!.addEventListener("click", () => void action(async () => {
  const response = await request("session", "GET", undefined, true);
  document.querySelector("#session-output")!.textContent = JSON.stringify(response, null, 2);
  resultElement.textContent = "Sesión válida; estado y roles comprobados por la API.";
}));
document.querySelector("#logout-button")!.addEventListener("click", () => void action(async () => {
  await request("logout", "POST", undefined, true); sessionToken = "";
  document.querySelector("#session-output")!.textContent = "Sesión cerrada."; resultElement.textContent = "Logout completado. La credencial anterior queda revocada.";
}));
document.querySelector("#profile-load")!.addEventListener("click", () => void action(async () => {
  const profile = await request("/api/v1/profile", "GET", undefined, true);
  const form = document.querySelector<HTMLFormElement>("#profile-form")!;
  form.querySelector<HTMLInputElement>('[name="display_name"]')!.value = String(profile.display_name ?? "");
  form.querySelector<HTMLInputElement>('[name="phone"]')!.value = String(profile.phone ?? "");
  document.querySelector<HTMLElement>("#privacy-output")!.textContent = JSON.stringify(profile, null, 2);
}));
form("profile-form", async (data) => {
  const profile = await request("/api/v1/profile", "PUT", {display_name:data.get("display_name"), phone:data.get("phone")}, true);
  document.querySelector<HTMLElement>("#privacy-output")!.textContent = JSON.stringify(profile, null, 2);
  resultElement.textContent = "Perfil guardado para la cuenta de la sesión actual.";
});
form("rights-form", async (data, element) => {
  const item = await request("/api/v1/rights-requests", "POST", {type:data.get("type")}, true);
  element.reset();
  document.querySelector<HTMLElement>("#privacy-output")!.textContent = JSON.stringify(item, null, 2);
  resultElement.textContent = "Solicitud registrada para revisión; no se han borrado datos.";
});
document.querySelector("#rights-load")!.addEventListener("click", () => void action(async () => {
  const items = await request("/api/v1/rights-requests", "GET", undefined, true);
  document.querySelector<HTMLElement>("#privacy-output")!.textContent = JSON.stringify(items, null, 2);
}));
form("verification-form", async (data) => {
  const item = await request("/api/v1/verifications", "POST", {type:data.get("type")}, true, crypto.randomUUID());
  document.querySelector<HTMLElement>("#verification-output")!.textContent = JSON.stringify(item, null, 2);
  resultElement.textContent = "Solicitud sintética creada; requiere revisión autorizada. No acredita identidad.";
});
document.querySelector("#verification-load")!.addEventListener("click", () => void action(async () => {
  const items = await request("/api/v1/verifications", "GET", undefined, true);
  document.querySelector<HTMLElement>("#verification-output")!.textContent = JSON.stringify(items, null, 2);
}));
form("verification-retry-form", async (data, element) => {
  const id = String(data.get("id"));
  const item = await request(`/api/v1/verifications/${encodeURIComponent(id)}/retry`, "POST", {corrected:data.get("corrected") === "on"}, true, crypto.randomUUID());
  document.querySelector<HTMLElement>("#verification-output")!.textContent = JSON.stringify(item, null, 2);
  element.reset(); resultElement.textContent = "Reintento fixture creado y en revisión.";
});
document.querySelector("#review-load")!.addEventListener("click", () => void action(async () => {
  const items = await request("/api/v1/admin/verifications", "GET", undefined, true);
  document.querySelector<HTMLElement>("#review-output")!.textContent = JSON.stringify(items, null, 2);
}));
form("review-form", async (data, element) => {
  const id = String(data.get("id")), decision = String(data.get("decision"));
  const item = await request(`/api/v1/admin/verifications/${encodeURIComponent(id)}/review`, "POST", {decision, reason_code:data.get("reason_code")}, true);
  document.querySelector<HTMLElement>("#review-output")!.textContent = JSON.stringify(item, null, 2);
  element.reset(); resultElement.textContent = "Revisión fixture registrada.";
});
const spacesOutput = document.querySelector<HTMLElement>("#space-output")!;
const spaceForm = document.querySelector<HTMLFormElement>("#space-form")!;
const spacesList = document.querySelector<HTMLElement>("#spaces-list")!;
let currentDraftID = "";
async function loadSpaceCategories(): Promise<void> {
  const categoryResult = await request("/api/v1/spaces/categories", "GET", undefined, true);
  const categories = categoryResult.items as Array<{code:string;name:string}>;
  const categorySelect = document.querySelector<HTMLSelectElement>("#space-category")!;
  categorySelect.replaceChildren(new Option("Selecciona categoría", ""));
  for (const category of categories) categorySelect.add(new Option(category.name, category.code));
}
function spaceInput(data: FormData): Record<string, unknown> {
  return {title:data.get("title"), description:data.get("description"), area_m2:Number(data.get("area_m2")), category_code:data.get("category_code"), capacity:Number(data.get("capacity")), usage_rules:data.get("usage_rules"), rate_unit:data.get("rate_unit"), base_price_clp:Number(data.get("base_price_clp")), address:data.get("address")};
}
async function loadSpaces(): Promise<void> {
  const result = await request("/api/v1/spaces", "GET", undefined, true);
  const items = result.items as Array<Record<string, unknown>>;
  spacesList.replaceChildren();
  for (const item of items) {
    const li = document.createElement("li"), button = document.createElement("button");
    button.type = "button"; button.textContent = `${String(item.title)} · ${String(item.state)}`;
    button.addEventListener("click", () => void action(async () => {
      const draft = await request(`/api/v1/spaces/${encodeURIComponent(String(item.id))}`, "GET", undefined, true);
      currentDraftID = String(draft.id); spaceForm.querySelector<HTMLInputElement>('[name="draft_id"]')!.value = currentDraftID;
      for (const key of ["title","description","area_m2","category_code","capacity","usage_rules","rate_unit","base_price_clp","address"]) {
        const field = spaceForm.elements.namedItem(key) as HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;
        field.value = String(draft[key] ?? "");
      }
      document.querySelector<HTMLButtonElement>("#space-save")!.textContent = "Guardar cambios";
      document.querySelector<HTMLButtonElement>("#space-cancel")!.hidden = false;
      spacesOutput.textContent = JSON.stringify(draft, null, 2);
    })); li.append(button); spacesList.append(li);
  }
  spacesOutput.textContent = JSON.stringify(result, null, 2);
}
document.querySelector("#spaces-load")!.addEventListener("click", () => void action(loadSpaces));
document.querySelector("#space-cancel")!.addEventListener("click", () => { spaceForm.reset(); currentDraftID = ""; document.querySelector<HTMLButtonElement>("#space-save")!.textContent = "Crear borrador"; document.querySelector<HTMLButtonElement>("#space-cancel")!.hidden = true; });
form("space-form", async (data, element) => {
  const id = String(data.get("draft_id") ?? "");
  const draft = await request(id ? `/api/v1/spaces/${encodeURIComponent(id)}` : "/api/v1/spaces", id ? "PUT" : "POST", spaceInput(data), true);
  spacesOutput.textContent = JSON.stringify(draft, null, 2); element.reset(); currentDraftID = "";
  document.querySelector<HTMLButtonElement>("#space-save")!.textContent = "Crear borrador"; document.querySelector<HTMLButtonElement>("#space-cancel")!.hidden = true;
  resultElement.textContent = id ? "Borrador guardado." : "Borrador privado creado."; await loadSpaces();
});
async function initialize(): Promise<void> {
  try {
    const config = await (await fetch("/config.json", {cache: "no-store"})).json() as MockConfig;
    const readyURL = new URL(config.apiReadyURL);
    if (readyURL.protocol !== "http:" && readyURL.protocol !== "https:") throw new Error("URL no admitida");
    apiBase = readyURL.origin;
    const ready = await fetch(readyURL, {cache: "no-store", mode: "cors"});
    if (!ready.ok) throw new Error("API no disponible");
    const terms = await request("terms");
    const items = terms.items as Array<{id:string;code:string;type:string}>;
    termIDs = items.filter(item => item.type === "terminos").map(item => item.id);
    document.querySelector("#terms-version")!.textContent = items.map(item => `${item.type}: ${item.code}`).join(" · ");
    statusElement.textContent = "API y PostgreSQL listos.";
  } catch { statusElement.textContent = "No se pudo comprobar la API local; revisa el entorno y recarga."; }
}
void initialize();
