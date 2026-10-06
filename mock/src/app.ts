import { ProfileRequestGate, profileMatchesSelection } from "./profile-request.js";
import { CalendarRequestState } from "./calendar-request.js";
import { BookingQuoteState } from "./booking-quote-state.js";
import { inboxActions } from "./booking-inbox-state.js";
import { showThenMarkConversationPage } from "./conversation-read-state.js";
import { CatalogPaginationState } from "./catalog-pagination-state.js";
import { actionWithButtonState } from "./action-button-state.js";
import { BookingAvailabilityState, type AvailabilityContext, type SelectedAvailability } from "./booking-availability-state.js";

interface MockConfig { apiReadyURL: string; }
interface APIError { error?: { code: string; message: string; request_id: string }; }
const statusElement = document.querySelector<HTMLElement>("#api-status")!;
const resultElement = document.querySelector<HTMLElement>("#result")!;
let apiBase = "";
let sessionToken = "";
let sessionAccountID = "";
let termIDs: string[] = [];
let evidenceObjectURL = "";

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
    if (authenticated && response.status === 401) {
      sessionToken = "";
      clearBookingInboxOnSessionLoss();
    }
    if (authenticated && path === "password/change" && response.status === 503) { sessionToken = ""; clearBookingInboxOnSessionLoss(); }
    const error = data as APIError;
    throw new Error(`${error.error?.message ?? "Error de API"} (HTTP ${response.status}, ${error.error?.code ?? "unknown"})`);
  }
  return data;
}
async function action(work: () => Promise<void>): Promise<void> {
  const buttons = [...document.querySelectorAll<HTMLButtonElement>("button")];
  try { await actionWithButtonState(buttons, work, () => { refreshCalendarControls(); refreshBookingActions(); refreshConversationControls(); refreshCatalogControls(); }); }
  catch (error) { resultElement.textContent = error instanceof Error ? error.message : "No se pudo conectar con la API."; }
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
  sessionToken = String(response.access_token); sessionAccountID = String(response.account_id); resetCatalogTraversal(); element.querySelector<HTMLInputElement>('[name="password"]')!.value = "";
  await loadSpaceCategories();
  await loadBookingInbox();
  resultElement.textContent = "Sesión iniciada. Puedes consultarla o cerrarla.";
});
form("recovery-request-form", async (data, element) => {
  await request("password/recovery", "POST", {email:data.get("email")});
  element.reset(); resultElement.textContent = "Si la cuenta es elegible, recibirás instrucciones en el buzón local.";
});
form("recovery-consume-form", async (data, element) => {
  await request("password/recovery/consume", "POST", {token_id:data.get("token_id"), token:data.get("token"), new_password:data.get("new_password"), confirm_password:data.get("confirm_password")});
  sessionToken = ""; resetCatalogTraversal(); element.reset(); resultElement.textContent = "Contraseña actualizada y sesiones cerradas. Inicia sesión con la nueva contraseña.";
});
form("password-change-form", async (data, element) => {
  await request("password/change", "POST", {current_password:data.get("current_password"), new_password:data.get("new_password"), confirm_password:data.get("confirm_password")}, true);
  sessionToken = ""; resetCatalogTraversal(); element.reset(); document.querySelector("#session-output")!.textContent = "Sesión revocada por cambio de contraseña.";
  resultElement.textContent = "Contraseña actualizada. Inicia sesión otra vez; se notificó al buzón local.";
});
document.querySelector("#session-button")!.addEventListener("click", () => void action(async () => {
  const response = await request("session", "GET", undefined, true);
  document.querySelector("#session-output")!.textContent = JSON.stringify(response, null, 2);
  resultElement.textContent = "Sesión válida; estado y roles comprobados por la API.";
}));
document.querySelector("#logout-button")!.addEventListener("click", () => void action(async () => {
  await request("logout", "POST", undefined, true); sessionToken = ""; resetCatalogTraversal();
  clearBookingInboxOnSessionLoss();
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
const evidenceCase = document.querySelector<HTMLInputElement>("#evidence-case-id")!;
const evidenceOutput = document.querySelector<HTMLElement>("#evidence-output")!;
const evidenceItems = document.querySelector<HTMLElement>("#evidence-items")!;
async function evidenceContent(caseID:string, evidenceID:string, admin=false):Promise<void> {
  if (!sessionToken) throw new Error("Primero inicia sesión.");
  const path = admin
    ? `/api/v1/admin/verifications/${encodeURIComponent(caseID)}/evidence/${encodeURIComponent(evidenceID)}`
    : `/api/v1/verifications/${encodeURIComponent(caseID)}/evidence/${encodeURIComponent(evidenceID)}`;
  const response = await fetch(`${apiBase}${path}`, {headers:{Authorization:`Bearer ${sessionToken}`, Accept:"image/png"}, mode:"cors", cache:"no-store", credentials:"omit"});
  if (!response.ok) {
    const data = await response.json() as APIError;
    throw new Error(`${data.error?.message ?? "Error de API"} (HTTP ${response.status}, ${data.error?.code ?? "unknown"})`);
  }
  const blob=await response.blob();
  if (evidenceObjectURL) URL.revokeObjectURL(evidenceObjectURL);
  evidenceObjectURL=URL.createObjectURL(blob);
  const preview=document.querySelector<HTMLImageElement>("#evidence-preview")!;
  preview.src=evidenceObjectURL; preview.hidden=false;
  evidenceOutput.textContent=`Fixture ${evidenceID} cargado (${blob.size} bytes, ${response.headers.get("ETag") ?? "sin hash"}).`;
}
document.querySelector("#evidence-upload")!.addEventListener("click", () => void action(async () => {
  const caseID=evidenceCase.value.trim();
  if (!caseID) throw new Error("Indica el ID de tu caso.");
  const item=await request(`/api/v1/verifications/${encodeURIComponent(caseID)}/evidence`,"POST",{fixture_code:"synthetic-png-v1"},true);
  evidenceOutput.textContent=`Fixture privado guardado. Metadatos: ${JSON.stringify(item,null,2)}`;
  await loadEvidence();
}));
document.querySelector("#evidence-invalid")!.addEventListener("click", () => void action(async () => {
  const caseID=evidenceCase.value.trim();
  if (!caseID) throw new Error("Indica el ID de tu caso.");
  await request(`/api/v1/verifications/${encodeURIComponent(caseID)}/evidence`,"POST",{fixture_code:"not-a-synthetic-fixture"},true);
}));
async function loadEvidence():Promise<void> {
  const caseID=evidenceCase.value.trim();
  if (!caseID) throw new Error("Indica el ID de tu caso.");
  const response=await request(`/api/v1/verifications/${encodeURIComponent(caseID)}/evidence`,"GET",undefined,true);
  const items=(response.items ?? []) as Array<{id:string;mime_type:string;size_bytes:number;sha256:string;created_at:string}>;
  evidenceItems.replaceChildren();
  for (const item of items) {
    const row=document.createElement("p"), view=document.createElement("button"), remove=document.createElement("button");
    row.append(document.createTextNode(`${item.id} · ${item.size_bytes} bytes · ${item.created_at} `));
    view.type="button"; view.textContent="Consultar PNG"; view.addEventListener("click",()=>void action(()=>evidenceContent(caseID,item.id)));
    remove.type="button"; remove.textContent="Eliminar fixture"; remove.addEventListener("click",()=>void action(async()=>{
      await request(`/api/v1/verifications/${encodeURIComponent(caseID)}/evidence/${encodeURIComponent(item.id)}`,"DELETE",undefined,true);
      evidenceOutput.textContent=`Fixture ${item.id} eliminado explícitamente.`; await loadEvidence();
    }));
    row.append(view,remove); evidenceItems.append(row);
  }
  evidenceOutput.textContent=`${items.length} fixture(s) privado(s): ${JSON.stringify(items,null,2)}`;
}
document.querySelector("#evidence-load")!.addEventListener("click",()=>void action(loadEvidence));
form("review-evidence-form",async(data)=>{
  const caseID=String(data.get("case_id"));
  const response=await request(`/api/v1/admin/verifications/${encodeURIComponent(caseID)}/evidence`,"GET",undefined,true);
  const items=(response.items ?? []) as Array<{id:string;size_bytes:number;created_at:string}>;
  const output=document.querySelector<HTMLElement>("#review-evidence-output")!;
  output.textContent=JSON.stringify(items,null,2);
  if(items[0]) await evidenceContent(caseID,items[0].id,true);
});
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
type AttributeDefinition = {code:string;label:string;order:number;description?:string;type:string;unit?:string;options?:string[];minimum?:number;maximum?:number;step?:number};
type AttributeProfile = {category_code:string;schema_version:number;attributes:AttributeDefinition[]};
let currentProfile: AttributeProfile | null = null;
const profileRequestGate = new ProfileRequestGate();
async function loadSpaceCategories(): Promise<void> {
  const categoryResult = await request("/api/v1/spaces/categories", "GET", undefined, true);
  const categories = categoryResult.items as Array<{code:string;name:string}>;
  const categorySelect = document.querySelector<HTMLSelectElement>("#space-category")!;
  categorySelect.replaceChildren(new Option("Selecciona categoría", ""));
  for (const category of categories) categorySelect.add(new Option(category.name, category.code));
  categorySelect.addEventListener("change", () => void action(async () => { await loadAttributeProfile(categorySelect.value); }));
  if (categorySelect.value) await loadAttributeProfile(categorySelect.value);
}
async function loadAttributeProfile(category:string, version?:number): Promise<boolean> {
  const requestID=profileRequestGate.begin();
  currentProfile=null;
  const root=document.querySelector<HTMLElement>("#space-attributes")!; root.replaceChildren();
  if (!category) return false;
  const versionPath=version===undefined?"":`/${version}`;
  const profile=await request(`/api/v1/spaces/categories/${encodeURIComponent(category)}/attributes${versionPath}`,"GET",undefined,true) as AttributeProfile;
  const selectedCategory=document.querySelector<HTMLSelectElement>("#space-category")!.value;
  if (!profileRequestGate.accepts(requestID,category,selectedCategory) || profile.category_code!==category || profile.schema_version!==(version??profile.schema_version)) return false;
  currentProfile=profile;
  for (const definition of [...profile.attributes].sort((a,b)=>a.order-b.order)) {
    const label=document.createElement("label"); label.textContent=`${definition.label}${definition.unit?` (${definition.unit})`:""}`;
    let control: HTMLInputElement|HTMLSelectElement;
    if (definition.type==="boolean") {
      const select=document.createElement("select"); select.add(new Option("No declarar","")); select.add(new Option("Sí","true")); select.add(new Option("No","false")); control=select;
    } else if (definition.type==="enum" || definition.type==="enum_list") {
      const select=document.createElement("select"); select.add(new Option("No declarar","")); if(definition.type==="enum_list") select.multiple=true;
      for (const option of definition.options??[]) select.add(new Option(option,option)); control=select;
    } else {
      const input=document.createElement("input"); input.type="number"; input.step=definition.type==="integer"?"1":String(definition.step??"any");
      if(definition.minimum!==undefined) input.min=String(definition.minimum); if(definition.maximum!==undefined) input.max=String(definition.maximum); control=input;
    }
    control.name=`attribute:${definition.code}`; if(definition.description) control.title=definition.description; label.append(control); root.append(label);
  }
  return true;
}
function spaceInput(data: FormData): Record<string, unknown> {
  const selectedCategory=String(data.get("category_code")??"");
  if (!currentProfile || !profileMatchesSelection(currentProfile.category_code,selectedCategory)) throw new Error("Espera a que cargue el perfil de la categoría seleccionada.");
  const attributes:Record<string,unknown>={};
  for(const definition of currentProfile?.attributes??[]) {
    const key=`attribute:${definition.code}`, raw=data.getAll(key);
    if(definition.type==="enum_list") { const values=raw.map(String).filter(Boolean); if(values.length) attributes[definition.code]=values; continue; }
    const value=String(raw[0]??""); if(value==="") continue;
    if(definition.type==="boolean") attributes[definition.code]=value==="true";
    else if(definition.type==="integer") attributes[definition.code]=Number.parseInt(value,10);
    else if(definition.type==="number") attributes[definition.code]=Number(value);
    else attributes[definition.code]=value;
  }
  return {title:data.get("title"), description:data.get("description"), area_m2:Number(data.get("area_m2")), category_code:selectedCategory, capacity:Number(data.get("capacity")), usage_rules:data.get("usage_rules"), rate_unit:data.get("rate_unit"), base_price_clp:Number(data.get("base_price_clp")), address:data.get("address"), attribute_schema_version:currentProfile.schema_version, attributes};
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
      const profileLoaded=await loadAttributeProfile(String(draft.category_code),Number(draft.attribute_schema_version));
      if (!profileLoaded) throw new Error("No se pudo cargar el perfil guardado del borrador.");
      for(const definition of currentProfile?.attributes??[]) {
        const control=spaceForm.elements.namedItem(`attribute:${definition.code}`) as HTMLInputElement|HTMLSelectElement|null;
        const value=(draft.attributes as Record<string,unknown>)?.[definition.code]; if(!control || value===undefined) continue;
        if(definition.type==="enum_list" && control instanceof HTMLSelectElement) { const selected=new Set(value as string[]); for(const option of control.options) option.selected=selected.has(option.value); }
        else control.value=definition.type==="boolean"?(value?"true":"false"):String(value);
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
const calendarSpace = document.querySelector<HTMLSelectElement>("#calendar-space")!;
const calendarOutput = document.querySelector<HTMLElement>("#calendar-output")!;
const calendarBlocks = document.querySelector<HTMLElement>("#calendar-blocks")!;
const calendarRequestState = new CalendarRequestState();
const calendarZoneInput = document.querySelector<HTMLInputElement>('#calendar-config-form [name="time_zone"]')!;
const calendarQueryForm = document.querySelector<HTMLFormElement>("#calendar-query-form")!;
const calendarBlockForm = document.querySelector<HTMLFormElement>("#calendar-block-form")!;
const priceRateForm = document.querySelector<HTMLFormElement>("#price-rate-form")!;
const priceRateOutput = document.querySelector<HTMLElement>("#price-rate-output")!;
const priceSimulationOutput = document.querySelector<HTMLElement>("#price-simulation-output")!;
function pricePath(suffix:string):string { if(!calendarSpace.value) throw new Error("Selecciona uno de tus borradores."); return `/api/v1/spaces/${encodeURIComponent(calendarSpace.value)}/${suffix}`; }
document.querySelector<HTMLButtonElement>("#price-load")!.addEventListener("click",()=>void action(async()=>{
  const spaceID=calendarSpace.value, selection=calendarRequestState.snapshot();
  const [rate,history]=await Promise.all([request(pricePath("tariff"),"GET",undefined,true),request(pricePath("tariffs"),"GET",undefined,true)]);
  if(!calendarRequestState.accepts(selection,spaceID,calendarSpace.value)) return;
  priceRateForm.querySelector<HTMLSelectElement>('[name="rate_unit"]')!.value=String(rate.rate_unit);
  priceRateForm.querySelector<HTMLInputElement>('[name="base_price"]')!.value=String(rate.base_price);
  priceRateOutput.textContent=JSON.stringify({current:rate,history:(history.items??[]),private:true},null,2);
}));
form("price-rate-form",async data=>{
  const spaceID=calendarSpace.value,selection=calendarRequestState.snapshot();
  const rate=await request(pricePath("tariff"),"PUT",{rate_unit:data.get("rate_unit"),base_price:Number(data.get("base_price"))},true);
  if(!calendarRequestState.accepts(selection,spaceID,calendarSpace.value)) return;
  const history=await request(pricePath("tariffs"),"GET",undefined,true);
  if(!calendarRequestState.accepts(selection,spaceID,calendarSpace.value)) return;
  priceRateOutput.textContent=JSON.stringify({current:rate,history:(history.items??[]),private:true},null,2);
  resultElement.textContent="Tarifa actualizada como nueva versión; las simulaciones guardadas conservan su snapshot.";
});
form("price-simulation-form",async data=>{
  const spaceID=calendarSpace.value,selection=calendarRequestState.snapshot();
  const zone=calendarRequestState.zoneFor(spaceID,calendarZoneInput.value);
  const simulation=await request(pricePath("price-simulations"),"POST",{start_at:localTimeAsUTC(String(data.get("start_at")),zone),end_at:localTimeAsUTC(String(data.get("end_at")),zone)},true);
  if(!calendarRequestState.accepts(selection,spaceID,calendarSpace.value)) return;
  priceSimulationOutput.textContent=JSON.stringify({...simulation,label:"SIMULACIÓN PRIVADA"},null,2);
  resultElement.textContent="Simulación privada guardada con su versión de tarifa. No se modificó la ocupación.";
});
form("price-simulation-load-form",async data=>{
  const spaceID=calendarSpace.value,selection=calendarRequestState.snapshot();
  const simulation=await request(pricePath(`price-simulations/${encodeURIComponent(String(data.get("id")))}`),"GET",undefined,true);
  if(!calendarRequestState.accepts(selection,spaceID,calendarSpace.value)) return;
  priceSimulationOutput.textContent=JSON.stringify({...simulation,label:"SIMULACIÓN PRIVADA · SNAPSHOT ORIGINAL"},null,2);
});
function refreshCalendarControls(): void {
  const dirty = !calendarSpace.value || (() => {
    try { calendarRequestState.zoneFor(calendarSpace.value, calendarZoneInput.value); return false; }
    catch { return true; }
  })();
  calendarQueryForm.querySelectorAll<HTMLButtonElement>("button").forEach(button => button.disabled = dirty);
  calendarBlockForm.querySelectorAll<HTMLButtonElement>("button").forEach(button => button.disabled = dirty);
  document.querySelector<HTMLFormElement>("#price-simulation-form")!.querySelectorAll<HTMLButtonElement>("button").forEach(button => button.disabled = dirty);
}
async function loadCalendarSpaces(): Promise<void> {
  const result = await request("/api/v1/spaces", "GET", undefined, true);
  const items = result.items as Array<Record<string, unknown>>;
  const selected = calendarSpace.value;
  calendarSpace.replaceChildren(new Option("Selecciona un borrador", ""));
  for (const item of items) calendarSpace.add(new Option(String(item.title), String(item.id)));
  if ([...calendarSpace.options].some(option => option.value === selected)) calendarSpace.value = selected;
}
function calendarPath(spaceID: string, suffix = ""): string {
  if (!spaceID) throw new Error("Selecciona uno de tus borradores.");
  return `/api/v1/spaces/${encodeURIComponent(spaceID)}/availability${suffix}`;
}
function localTimeAsUTC(value: string, zone: string): string {
  if (!value || !zone) throw new Error("Selecciona fechas y configura una zona horaria IANA.");
  const normalized = value.length === 16 ? `${value}:00` : value;
  const target = Date.parse(`${normalized}Z`);
  if (!Number.isFinite(target)) throw new Error("La fecha local no es válida.");
  const formatter = new Intl.DateTimeFormat("en-CA", {timeZone:zone, year:"numeric", month:"2-digit", day:"2-digit", hour:"2-digit", minute:"2-digit", second:"2-digit", hourCycle:"h23"});
  const fieldsAt = (instant: number): Record<string,number> => Object.fromEntries(formatter.formatToParts(new Date(instant)).filter(part => part.type !== "literal").map(part => [part.type, Number(part.value)])) as Record<string,number>;
  let guess = target;
  for (let attempt=0; attempt<4; attempt++) {
    const fields=fieldsAt(guess);
    const represented=Date.UTC(fields.year,fields.month-1,fields.day,fields.hour,fields.minute,fields.second);
    const delta=target-represented;
    if(delta===0) return new Date(guess).toISOString();
    guess+=delta;
  }
  throw new Error("La hora local no existe en esa zona por un cambio horario. Elige otra hora.");
}
function renderCalendarBlocks(items: Array<Record<string,unknown>>, spaceID: string, selection: number): void {
  if (!calendarRequestState.accepts(selection, spaceID, calendarSpace.value)) return;
  calendarBlocks.replaceChildren();
  for (const block of items) {
    const li=document.createElement("li"), summary=document.createElement("span");
    const zone=String(block.time_zone), formatter=new Intl.DateTimeFormat("es-CL", {timeZone:zone,dateStyle:"medium",timeStyle:"short"});
    summary.textContent=`${formatter.format(new Date(String(block.start_at)))} – ${formatter.format(new Date(String(block.end_at)))} (${zone}): ${String(block.reason)} `;
    const remove=document.createElement("button"); remove.type="button"; remove.textContent="Quitar bloqueo";
    remove.addEventListener("click", () => void action(async () => {
      const capturedSpaceID = spaceID;
      const capturedSelection = calendarRequestState.snapshot();
      await request(`${calendarPath(capturedSpaceID,"/blocks")}/${encodeURIComponent(String(block.id))}`,"DELETE",undefined,true);
      if (!calendarRequestState.accepts(capturedSelection,capturedSpaceID,calendarSpace.value)) return;
      calendarOutput.textContent="Bloqueo retirado.";
      calendarQueryForm.requestSubmit();
    }));
    li.append(summary,remove); calendarBlocks.append(li);
  }
}
document.querySelector<HTMLButtonElement>("#calendar-spaces-load")!.addEventListener("click", () => void action(loadCalendarSpaces));
calendarSpace.addEventListener("change", () => void action(async () => {
  const spaceID=calendarSpace.value;
  const selection=calendarRequestState.beginSelection();
  calendarZoneInput.value=""; calendarBlocks.replaceChildren(); calendarOutput.textContent=""; refreshCalendarControls();
  if(!spaceID) return;
  const result=await request(calendarPath(spaceID,"/timezone"),"GET",undefined,true);
  if(!calendarRequestState.confirm(spaceID,String(result.time_zone),selection,calendarSpace.value)) return;
  calendarZoneInput.value=String(result.time_zone); calendarOutput.textContent=JSON.stringify(result,null,2); refreshCalendarControls();
}));
form("calendar-config-form", async data => {
  const spaceID=calendarSpace.value, selection=calendarRequestState.snapshot();
  const result=await request(calendarPath(spaceID),"PUT",{time_zone:data.get("time_zone")},true);
  if(!calendarRequestState.confirm(spaceID,String(result.time_zone),selection,calendarSpace.value)) return;
  calendarOutput.textContent=JSON.stringify(result,null,2);
  calendarZoneInput.value=String(result.time_zone); refreshCalendarControls();
});
form("calendar-query-form", async data => {
  const spaceID=calendarSpace.value, selection=calendarRequestState.snapshot();
  const zone=calendarRequestState.zoneFor(spaceID,calendarZoneInput.value);
  const query=new URLSearchParams({from:localTimeAsUTC(String(data.get("from")),zone),to:localTimeAsUTC(String(data.get("to")),zone)});
  const availability=await request(`${calendarPath(spaceID)}?${query}`,"GET",undefined,true);
  if(!calendarRequestState.accepts(selection,spaceID,calendarSpace.value)) return;
  const blocks=await request(`${calendarPath(spaceID,"/blocks")}?${query}`,"GET",undefined,true);
  if(!calendarRequestState.accepts(selection,spaceID,calendarSpace.value)) return;
  renderCalendarBlocks((blocks.items??[]) as Array<Record<string,unknown>>,spaceID,selection);
  calendarOutput.textContent=JSON.stringify({availability,blocks},null,2);
});
form("calendar-block-form", async (data, element) => {
  const spaceID=calendarSpace.value, selection=calendarRequestState.snapshot();
  const zone=calendarRequestState.zoneFor(spaceID,calendarZoneInput.value);
  const result=await request(calendarPath(spaceID,"/blocks"),"POST",{start_at:localTimeAsUTC(String(data.get("start_at")),zone),end_at:localTimeAsUTC(String(data.get("end_at")),zone),reason:data.get("reason")},true);
  if(!calendarRequestState.accepts(selection,spaceID,calendarSpace.value)) return;
  calendarOutput.textContent=JSON.stringify(result,null,2); element.reset();
});
calendarZoneInput.addEventListener("input",refreshCalendarControls);
refreshCalendarControls();
type BookingFixture = {space_id:string; title:string; category_code:string; category_name:string; description:string; time_zone:string; rate_unit:string; base_price_clp:number; currency:string; profile_version:number; profile:Record<string,unknown>; attributes:Record<string,unknown>};
type CatalogItem = BookingFixture & {available?:boolean};
type CatalogSearchItem = CatalogItem & {estimated_total_clp?:number;distance_km?:number;distance_kind?:"direct"};
let bookingFixture:BookingFixture|null=null;
let bookingCatalogRequest=0;
let catalogNextCursor="";
let catalogRequestCursor="";
let catalogLoading=false;
const catalogPagination=new CatalogPaginationState();
let catalogProfileRequest=0;
let catalogFilterProfile:AttributeProfile|null=null;
const bookingQuoteState=new BookingQuoteState();
const bookingAvailabilityState=new BookingAvailabilityState();
let selectedAvailabilityContext:AvailabilityContext|null=null;
let reservationKey=crypto.randomUUID();
const paymentKeys=new Map<string,string>();
const bookingBase="/api/v1/local/booking-trial";
const catalogNextButton=document.querySelector<HTMLButtonElement>("#booking-catalog-next")!;
function refreshCatalogControls():void { const button=document.querySelector<HTMLButtonElement>("#booking-catalog-next");if(button)button.disabled=catalogLoading||!catalogPagination.canNext; }
function clearCatalogResultsAndSelection(message:string):void {
  catalogProfileRequest++;
  clearWeeklyHoursEditor();
  bookingQuoteState.beginSearch();bookingAvailabilityState.invalidate();bookingFixture=null;
  resetAvailabilityPicker(message);
  const results=document.querySelector<HTMLElement>("#booking-catalog-results");if(results){results.replaceChildren();results.textContent=message;}
  const detail=document.querySelector<HTMLElement>("#booking-fixture-output");if(detail)detail.textContent=message;
  const quote=document.querySelector<HTMLElement>("#booking-quote-output");if(quote)quote.textContent="La cotización se invalidó al cambiar la sesión.";
  const space=document.querySelector<HTMLInputElement>('#booking-quote-form [name="space_id"]');if(space)space.value="";
  const quoteID=document.querySelector<HTMLInputElement>('#booking-request-form [name="quote_id"]');if(quoteID)quoteID.value="";
}
function resetCatalogTraversal():void { bookingCatalogRequest++;catalogPagination.invalidate();catalogNextCursor="";catalogRequestCursor="";catalogLoading=false;clearCatalogResultsAndSelection("Inicia sesión y busca fixtures sintéticos autorizados para esta cuenta.");refreshCatalogControls(); }
function invalidateCatalogSelection(message:string):void { bookingQuoteState.beginSearch();bookingAvailabilityState.invalidate();bookingFixture=null;clearWeeklyHoursEditor();resetAvailabilityPicker(message);(document.querySelector<HTMLInputElement>('#booking-quote-form [name="space_id"]')!).value="";(document.querySelector<HTMLInputElement>('#booking-request-form [name="quote_id"]')!).value="";bookingQuoteOutput.textContent=message; }
const bookingFixtureOutput=document.querySelector<HTMLElement>("#booking-fixture-output")!;
const bookingQuoteOutput=document.querySelector<HTMLElement>("#booking-quote-output")!;
const bookingAvailabilityPicker=document.querySelector<HTMLElement>("#booking-availability-picker")!;
const bookingWeeklyHoursEditor=document.querySelector<HTMLElement>("#booking-weekly-hours-editor")!;
const bookingHistoryOutput=document.querySelector<HTMLElement>("#booking-history-output")!;

type WeeklyPeriod={open:string;close:string};
type WeeklyDay={weekday:number;periods:WeeklyPeriod[]};
type WeeklyHours={space_id:string;enabled:boolean;time_zone:string;days:WeeklyDay[]};
let weeklyHoursRequest=0;
function clearWeeklyHoursEditor(message=""):void {
  weeklyHoursRequest++;
  bookingWeeklyHoursEditor.replaceChildren();
  if(message){const p=document.createElement("p");p.textContent=message;bookingWeeklyHoursEditor.append(p);}
}
async function renderWeeklyHoursEditor(spaceID:string):Promise<void> {
  const token=++weeklyHoursRequest, account=sessionAccountID, session=sessionToken;
  bookingWeeklyHoursEditor.replaceChildren();
  if(!session||!account||bookingFixture?.space_id!==spaceID||bookingFixture.rate_unit!=="hora")return;
  try {
    const response=await request(`${bookingBase}/catalog/${encodeURIComponent(spaceID)}/weekly-hours`,"GET",undefined,true);
    if(token!==weeklyHoursRequest||session!==sessionToken||account!==sessionAccountID||bookingFixture?.space_id!==spaceID)return;
    const schedule=bookingData<WeeklyHours>(response), formElement=document.createElement("form"), title=document.createElement("h4"), enabledLabel=document.createElement("label"), enabled=document.createElement("input"), daysContainer=document.createElement("div"), status=document.createElement("p"), save=document.createElement("button");
    formElement.id="booking-weekly-hours-form";title.textContent=`Horario semanal (${schedule.time_zone})`;
    enabled.type="checkbox";enabled.name="enabled";enabled.checked=schedule.enabled;enabledLabel.append(enabled,document.createTextNode(" Aplicar horario semanal; días sin tramos quedan cerrados"));
    const dayNames=["Lunes","Martes","Miércoles","Jueves","Viernes","Sábado","Domingo"];
    const controls:Array<{weekday:number;opens:HTMLInputElement[];closes:HTMLInputElement[]}>=[];
    for(let weekday=1;weekday<=7;weekday++){
      const saved=schedule.days.find(day=>day.weekday===weekday)?.periods??[], fieldset=document.createElement("fieldset"), legend=document.createElement("legend");legend.textContent=dayNames[weekday-1];fieldset.append(legend);
      const opens:HTMLInputElement[]=[],closes:HTMLInputElement[]=[];
      for(let slot=0;slot<2;slot++){
        const row=document.createElement("div"), open=document.createElement("input"), close=document.createElement("input");
        open.type="text";open.inputMode="numeric";open.placeholder="09:00";open.maxLength=5;open.setAttribute("aria-label",`${dayNames[weekday-1]} tramo ${slot+1} apertura`);open.value=saved[slot]?.open??"";
        close.type="text";close.inputMode="numeric";close.placeholder="17:00 o 24:00";close.maxLength=5;close.setAttribute("aria-label",`${dayNames[weekday-1]} tramo ${slot+1} cierre`);close.value=saved[slot]?.close??"";
        row.append(document.createTextNode(`Tramo ${slot+1}: `),open,document.createTextNode(" a "),close);fieldset.append(row);opens.push(open);closes.push(close);
      }
      controls.push({weekday,opens,closes});daysContainer.append(fieldset);
    }
    status.textContent="Sin configuración activa se conserva la disponibilidad actual. Cierre 24:00 significa fin del día; no cruza medianoche.";
    save.type="submit";save.textContent="Guardar horario semanal";formElement.append(title,enabledLabel,daysContainer,status,save);bookingWeeklyHoursEditor.replaceChildren(formElement);
    formElement.addEventListener("submit",event=>{event.preventDefault();void action(async()=>{
      if(session!==sessionToken||account!==sessionAccountID||bookingFixture?.space_id!==spaceID)throw new Error("Cambió la sesión o el espacio; vuelve a cargar el detalle.");
      const days=controls.map(({weekday,opens,closes})=>({weekday,periods:opens.flatMap((open,index)=>{const a=open.value.trim(),b=closes[index].value.trim();if(!a&&!b)return [];return [{open:a,close:b}];})}));
      const result=await request(`${bookingBase}/catalog/${encodeURIComponent(spaceID)}/weekly-hours`,"PUT",{enabled:enabled.checked,days},true);
      if(token!==weeklyHoursRequest||session!==sessionToken||account!==sessionAccountID||bookingFixture?.space_id!==spaceID)return;
      const saved=bookingData<WeeklyHours>(result);
      bookingAvailabilityState.invalidate();selectedAvailabilityContext=null;
      bookingQuoteState.beginSelection(spaceID);
      (document.querySelector<HTMLInputElement>('#booking-request-form [name="quote_id"]')!).value="";
      status.textContent=saved.enabled?"Horario guardado. Las nuevas consultas, cotizaciones y reservas se validan con esta regla; las reservas existentes no cambian.":"Horario desactivado. Se conserva el comportamiento anterior; las reservas existentes no cambian.";
      bookingQuoteOutput.textContent="Cambió la configuración horaria. Descarta la cotización previa y vuelve a consultar y cotizar.";
      void renderAvailabilityPicker();
    });});
  } catch(error) {
    if(token!==weeklyHoursRequest||session!==sessionToken||account!==sessionAccountID||bookingFixture?.space_id!==spaceID)return;
    const message=error instanceof Error?error.message:"No se pudo cargar el horario.";
    if(message.includes("HTTP 404"))return; // Participants can book, only the owner edits.
    clearWeeklyHoursEditor(message);
  }
}

type AvailabilityOptionsPayload={space_id:string;time_zone:string;rate_unit:string;items:Array<{start_at:string;end_at:string}>};
function resetAvailabilityPicker(message:string):void {
  bookingAvailabilityState.invalidate();
  selectedAvailabilityContext=null;
  bookingAvailabilityPicker.replaceChildren();
  if(message){const p=document.createElement("p");p.textContent=message;bookingAvailabilityPicker.append(p);}
}
function localDateText(instant:Date,zone:string):string {
  const parts=new Intl.DateTimeFormat("en-CA",{timeZone:zone,year:"numeric",month:"2-digit",day:"2-digit"}).formatToParts(instant);
  const fields=Object.fromEntries(parts.filter(p=>p.type!=="literal").map(p=>[p.type,p.value]));
  return `${fields.year}-${fields.month}-${fields.day}`;
}
function addDateDays(date:string,days:number):string {
  const [year,month,day]=date.split("-").map(Number);
  const value=new Date(Date.UTC(year,month-1,day+days));
  return `${value.getUTCFullYear()}-${String(value.getUTCMonth()+1).padStart(2,"0")}-${String(value.getUTCDate()).padStart(2,"0")}`;
}
function localDateTimeText(instant:string,zone:string):string {
  const parts=new Intl.DateTimeFormat("en-CA",{timeZone:zone,year:"numeric",month:"2-digit",day:"2-digit",hour:"2-digit",minute:"2-digit",hourCycle:"h23"}).formatToParts(new Date(instant));
  const fields=Object.fromEntries(parts.filter(p=>p.type!=="literal").map(p=>[p.type,p.value]));
  return `${fields.year}-${fields.month}-${fields.day}T${fields.hour}:${fields.minute}`;
}
function formatAvailabilityInstant(instant:string,zone:string):string {
  return new Intl.DateTimeFormat("es-CL",{timeZone:zone,year:"numeric",month:"2-digit",day:"2-digit",hour:"2-digit",minute:"2-digit",hourCycle:"h23",timeZoneName:"shortOffset"}).format(new Date(instant));
}
function currentAvailabilityContext(date:string,duration:string):AvailabilityContext|null {
  if(!bookingFixture||!sessionToken||!sessionAccountID)return null;
  return {spaceID:bookingFixture.space_id,date,duration,accountID:sessionAccountID,sessionToken};
}
function invalidateSelectedInterval(message:string):void {
  bookingAvailabilityState.invalidate();
  selectedAvailabilityContext=null;
  if(bookingFixture)bookingQuoteState.beginSelection(bookingFixture.space_id);
  const quoteID=document.querySelector<HTMLInputElement>('#booking-request-form [name="quote_id"]');if(quoteID)quoteID.value="";
  bookingQuoteOutput.textContent=message;
}
function renderAvailabilityPicker():void {
  if(!bookingFixture){resetAvailabilityPicker("Selecciona un espacio para consultar intervalos.");return;}
  bookingAvailabilityState.invalidate();
  bookingAvailabilityPicker.replaceChildren();
  const zone=bookingFixture.time_zone, now=new Date(), today=localDateText(now,zone), maxDate=addDateDays(today,90);
  const container=document.createElement("section"), title=document.createElement("h4");title.textContent="Horarios candidatos";container.append(title);
  const dateLabel=document.createElement("label");dateLabel.textContent=`Fecha (${zone}) `;
  const dateInput=document.createElement("input");dateInput.type="date";dateInput.min=today;dateInput.max=maxDate;dateInput.value=today;dateInput.required=true;dateLabel.append(dateInput);container.append(dateLabel);
  const shortcuts=document.createElement("div"), shortcutsLabel=document.createElement("p");shortcutsLabel.textContent="Próximos siete días";shortcuts.append(shortcutsLabel);
  for(let offset=0;offset<7;offset++){
    const date=addDateDays(today,offset), button=document.createElement("button");button.type="button";
    button.textContent=offset===0?`Hoy · ${date}`:offset===1?`Mañana · ${date}`:date;
    button.addEventListener("click",()=>{dateInput.value=date;void queryOptions();});shortcuts.append(button);
  }
  container.append(shortcuts);
  const durationLabel=document.createElement("label");durationLabel.textContent="Duración ";
  const duration=document.createElement("select");
  const choices=bookingFixture.rate_unit==="hora"?[[1,"1 hora transcurrida"],[2,"2 horas transcurridas"],[4,"4 horas transcurridas"]] as const:bookingFixture.rate_unit==="dia"?[[1,"1 día calendario"],[2,"2 días calendario"],[3,"3 días calendario"]] as const:[[1,"1 mes calendario"]] as const;
  for(const [value,label] of choices)duration.add(new Option(label,String(value)));
  durationLabel.append(duration);container.append(durationLabel);
  const consult=document.createElement("button");consult.type="button";consult.textContent="Consultar horarios";container.append(consult);
  const notice=document.createElement("p");notice.textContent="Disponibilidad consultada; se confirmará al cotizar y reservar.";container.append(notice);
  const status=document.createElement("p");status.textContent="Elige una fecha y consulta candidatos; solo se consulta una fecha por solicitud.";container.append(status);
  const options=document.createElement("ul");container.append(options);bookingAvailabilityPicker.append(container);
  const clearCurrent=()=>{invalidateSelectedInterval("La fecha o duración cambió; vuelve a consultar y cotizar.");options.replaceChildren();status.textContent="La consulta anterior quedó invalidada.";};
  dateInput.addEventListener("input",clearCurrent);dateInput.addEventListener("change",clearCurrent);
  duration.addEventListener("input",clearCurrent);duration.addEventListener("change",clearCurrent);
  const queryOptions=async()=>{
    if(!dateInput.value||dateInput.value<today||dateInput.value>maxDate){throw new Error("Elige una fecha local entre hoy y los próximos 90 días.");}
    const context=currentAvailabilityContext(dateInput.value,duration.value);if(!context)throw new Error("La sesión o el espacio seleccionado cambiaron.");
    const token=bookingAvailabilityState.beginRequest(context);options.replaceChildren();status.textContent="Consultando intervalos libres…";
    const query=new URLSearchParams({date:context.date,duration:context.duration});
    const response=await request(`${bookingBase}/catalog/${encodeURIComponent(context.spaceID)}/availability-options?${query}`,"GET",undefined,true);
    const current=currentAvailabilityContext(dateInput.value,duration.value);
    if(!current||!bookingAvailabilityState.accepts(token,current)||bookingFixture?.space_id!==context.spaceID)return;
    const payload=bookingData<AvailabilityOptionsPayload>(response);
    if(payload.space_id!==context.spaceID||payload.time_zone!==bookingFixture.time_zone||payload.rate_unit!==bookingFixture.rate_unit){status.textContent="La ficha del espacio cambió; vuelve a seleccionarla.";return;}
    status.textContent=payload.items.length?"Disponibilidad consultada; se confirmará al cotizar y reservar.":"No hay intervalos libres para esa fecha y duración.";
    for(const interval of payload.items){
      const startLocal=localDateTimeText(interval.start_at,payload.time_zone),endLocal=localDateTimeText(interval.end_at,payload.time_zone);
      const li=document.createElement("li"),select=document.createElement("button");select.type="button";
      select.textContent=`Usar ${formatAvailabilityInstant(interval.start_at,payload.time_zone)} – ${formatAvailabilityInstant(interval.end_at,payload.time_zone)} (${payload.time_zone})`;
      select.addEventListener("click",()=>{
        const currentSelection=currentAvailabilityContext(dateInput.value,duration.value);
        const selected:SelectedAvailability={startAt:interval.start_at,endAt:interval.end_at,startLocal,endLocal};
        if(!currentSelection||!bookingAvailabilityState.select(token,currentSelection,selected))return;
        selectedAvailabilityContext={...currentSelection};
        (document.querySelector<HTMLInputElement>('#booking-quote-form [name="start_at"]')!).value=startLocal;
        (document.querySelector<HTMLInputElement>('#booking-quote-form [name="end_at"]')!).value=endLocal;
        (document.querySelector<HTMLInputElement>('#booking-request-form [name="quote_id"]')!).value="";
        bookingQuoteState.beginSelection(context.spaceID);
        bookingQuoteOutput.textContent="Intervalo seleccionado. Cotiza para volver a validar la disponibilidad.";
        resultElement.textContent="Intervalo cargado en el formulario de cotización.";
      });
      li.append(select);options.append(li);
    }
  };
  consult.addEventListener("click",()=>void action(queryOptions));
}
const bookingCatalogCategory=document.querySelector<HTMLSelectElement>("#booking-catalog-category")!;
const bookingCatalogProfileFilters=document.querySelector<HTMLElement>("#booking-catalog-profile-filters")!;
const bookingCatalogNearbyEnabled=document.querySelector<HTMLInputElement>("#booking-catalog-nearby-enabled")!;
const bookingCatalogLatitude=document.querySelector<HTMLInputElement>("#booking-catalog-latitude")!;
const bookingCatalogLongitude=document.querySelector<HTMLInputElement>("#booking-catalog-longitude")!;
const bookingCatalogRadius=document.querySelector<HTMLSelectElement>("#booking-catalog-radius")!;
const bookingCatalogCenterSample=document.querySelector<HTMLButtonElement>("#booking-catalog-center-sample")!;
type TrialReservation={id:string;quote_id:string;space_id:string;host_id:string;renter_id:string;state:string;rate_unit:string;unit_price_clp:number;currency:string;units:number;subtotal_clp:number;start_at:string;end_at:string;time_zone:string;cancellation_policy_version:string;pay_expires_at:string;host_expires_at?:string|null;refund_id?:string;refund_operation_id?:string;refund_amount_clp?:number;refund_state?:string;refund_last_result?:string;refund_updated_at?:string;updated_at:string;unread_count:number};
type TrialTransition={sequence:number;to:string;reason:string;at:string};
type TrialDetail=TrialReservation&{history:TrialTransition[]};
type CancellationPreview={reservation_id:string;policy_version:string;eligible:boolean;deadline:string;amount_clp:number;currency:string;refund_label:string;reason_code?:string};
type ConversationMessage={id:string;reservation_id:string;author_id:string;sequence:number;body:string;created_at:string};
type ConversationPage={items:ConversationMessage[];older_cursor:number|null};
let selectedReservationID="";
let selectedReservation:TrialDetail|null=null;
let bookingInboxRevision=0;
let conversationRevision=0;
let conversationOlderCursor:number|null=null;
let conversationMessages:ConversationMessage[]=[];
let pendingMessageKey="";
let pendingMessageBody="";
let cancellationPreview:CancellationPreview|null=null;
let cancellationPreviewReservationID="";
const cancellationKeys=new Map<string,string>();
const renterInbox=document.querySelector<HTMLElement>("#booking-inbox-renter")!;
const hostInbox=document.querySelector<HTMLElement>("#booking-inbox-host")!;
const conversationOutput=document.querySelector<HTMLElement>("#booking-conversation-messages")!;
const conversationStatus=document.querySelector<HTMLElement>("#booking-conversation-status")!;
function bookingData<T>(result:Record<string,unknown>):T{return result.data as T}
const catalogResults=document.querySelector<HTMLElement>("#booking-catalog-results")!;
async function loadCatalogFilterProfile(category:string):Promise<void>{
  const token=++catalogProfileRequest;
  catalogFilterProfile=null;
  bookingCatalogProfileFilters.replaceChildren();
  bookingCatalogProfileFilters.textContent=category?"Cargando filtros de esta categoría…":"Selecciona una categoría para cargar sus filtros tipados.";
  if(!category)return;
  const profile=await request(`/api/v1/spaces/categories/${encodeURIComponent(category)}/attributes`,"GET",undefined,true) as AttributeProfile;
  if(token!==catalogProfileRequest||bookingCatalogCategory.value!==category||profile.category_code!==category)return;
  catalogFilterProfile=profile;
  bookingCatalogProfileFilters.replaceChildren();
  for(const definition of [...profile.attributes].sort((a,b)=>a.order-b.order)){
    const label=document.createElement("label");label.textContent=`${definition.label}${definition.unit?` (${definition.unit})`:""}`;
    let control:HTMLInputElement|HTMLSelectElement;
    if(definition.type==="boolean"){
      const select=document.createElement("select");select.add(new Option("Cualquiera",""));select.add(new Option("Sí","true"));select.add(new Option("No","false"));control=select;
    }else if(definition.type==="enum"||definition.type==="enum_list"){
      const select=document.createElement("select");select.add(new Option(definition.type==="enum_list"?"Cualquiera (selecciona una o más)":"Cualquiera",""));if(definition.type==="enum_list")select.multiple=true;
      for(const option of definition.options??[])select.add(new Option(option,option));control=select;
    }else{
      const input=document.createElement("input");input.type="number";input.step=definition.type==="integer"?"1":String(definition.step??"any");
      if(definition.minimum!==undefined)input.min=String(definition.minimum);if(definition.maximum!==undefined)input.max=String(definition.maximum);control=input;
    }
    control.name=`catalog_attribute:${definition.code}`;if(definition.description)control.title=definition.description;label.append(control);bookingCatalogProfileFilters.append(label);
  }
}
bookingCatalogCategory.addEventListener("change",()=>void action(async()=>loadCatalogFilterProfile(bookingCatalogCategory.value)));
bookingCatalogNearbyEnabled.addEventListener("change",()=>{
  const enabled=bookingCatalogNearbyEnabled.checked;
  bookingCatalogLatitude.disabled=!enabled;bookingCatalogLongitude.disabled=!enabled;bookingCatalogRadius.disabled=!enabled;
  bookingCatalogLatitude.required=enabled;bookingCatalogLongitude.required=enabled;bookingCatalogRadius.required=enabled;
  if(enabled&&!bookingCatalogRadius.value)bookingCatalogRadius.value="5";
});
bookingCatalogCenterSample.addEventListener("click",()=>{
  bookingCatalogLatitude.value="-33.4560";bookingCatalogLongitude.value="-70.6693";
  bookingCatalogLatitude.dispatchEvent(new Event("input",{bubbles:true}));
  bookingCatalogLongitude.dispatchEvent(new Event("input",{bubbles:true}));
});
form("booking-catalog-form",async data=>{
  const token=++bookingCatalogRequest;
  const requestSession=sessionToken,requestAccount=sessionAccountID;
  const requestedCursor=catalogRequestCursor;
  catalogRequestCursor="";
  catalogPagination.invalidate();
  catalogNextCursor="";catalogLoading=false;refreshCatalogControls();
  const query=new URLSearchParams();
  const category=String(data.get("category_code")??"");
  if(category)query.set("category_code",category);
  const localStart=String(data.get("start_at")??""),localEnd=String(data.get("end_at")??"");
  if(Boolean(localStart)!==Boolean(localEnd))throw new Error("Para filtrar disponibilidad indica inicio y término.");
  const minTotal=String(data.get("min_total_clp")??""),maxTotal=String(data.get("max_total_clp")??"");
  if((minTotal||maxTotal)&&!localStart)throw new Error("Para filtrar el precio total estimado indica inicio y término.");
  if(minTotal)query.set("min_total_clp",minTotal);
  if(maxTotal)query.set("max_total_clp",maxTotal);
  if(bookingCatalogNearbyEnabled.checked){
    const latitude=String(data.get("latitude")??""),longitude=String(data.get("longitude")??""),radius=String(data.get("radius_km")??"");
    if(!latitude||!longitude||!radius)throw new Error("Para filtrar por cercanía indica latitud, longitud y radio.");
    query.set("latitude",latitude);query.set("longitude",longitude);query.set("radius_km",radius);
  }
  const searchZone=String(data.get("time_zone")??"").trim();
  if(localStart){
    if(!searchZone)throw new Error("Indica la zona horaria para interpretar el intervalo de búsqueda.");
    query.set("start_at",localTimeAsUTC(localStart,searchZone));
    query.set("end_at",localTimeAsUTC(localEnd,searchZone));
  }
  const attributes:Record<string,unknown>={};
  const selectedCategory=bookingCatalogCategory.value;
  const definitions=catalogFilterProfile?.attributes??[];
  for(const definition of definitions){
    const key=`catalog_attribute:${definition.code}`,raw=data.getAll(key).map(String);
    if(definition.type==="enum_list"){
      const values=raw.filter(Boolean);if(values.length)attributes[definition.code]=values;
      continue;
    }
    const value=raw[0]??"";if(value==="")continue;
    if(definition.type==="boolean")attributes[definition.code]=value==="true";
    else if(definition.type==="integer")attributes[definition.code]=Number.parseInt(value,10);
    else if(definition.type==="number")attributes[definition.code]=Number(value);
    else attributes[definition.code]=value;
  }
  if(Object.keys(attributes).length){
    if(!catalogFilterProfile||catalogFilterProfile.category_code!==selectedCategory)throw new Error("Espera a que cargue el perfil de la categoría antes de buscar.");
    query.set("profile_version",String(catalogFilterProfile.schema_version));
    query.set("attributes",JSON.stringify(attributes));
  }
  const pageSize=Number(data.get("page_size")??5);
  if(!Number.isInteger(pageSize)||pageSize<1||pageSize>25)throw new Error("El tamaño de página debe estar entre 1 y 25.");
  query.set("page_size",String(pageSize));
  if(requestedCursor)query.set("cursor",requestedCursor);
  const pageToken=catalogPagination.beginRequest();
  catalogLoading=true;refreshCatalogControls();
  bookingQuoteState.beginSearch();bookingAvailabilityState.invalidate();selectedAvailabilityContext=null;
  bookingFixture=null;
  (document.querySelector<HTMLInputElement>('#booking-quote-form [name="space_id"]')!).value="";
  (document.querySelector<HTMLInputElement>('#booking-request-form [name="quote_id"]')!).value="";
  bookingQuoteOutput.textContent="La cotización queda invalidada al iniciar otra búsqueda.";
  bookingFixtureOutput.textContent="Selecciona un resultado para consultar su detalle.";
  resetAvailabilityPicker("La búsqueda nueva invalidó los intervalos anteriores.");
  const suffix=query.size?`?${query.toString()}`:"";
  let response:Record<string,unknown>;
  try { response=await request(`${bookingBase}/catalog${suffix}`,"GET",undefined,true); }
  finally { if(token===bookingCatalogRequest){catalogLoading=false;catalogPagination.finish(pageToken,catalogNextCursor);refreshCatalogControls();} }
  if(token!==bookingCatalogRequest||!catalogPagination.accepts(pageToken,requestAccount,sessionAccountID)||requestSession!==sessionToken)return;
  const payload=bookingData<{items:CatalogSearchItem[];next_cursor?:string}>(response);
  catalogNextCursor=payload.next_cursor??"";
  catalogPagination.finish(pageToken,catalogNextCursor);refreshCatalogControls();
  catalogResults.replaceChildren();
  if(!payload.items.length){catalogResults.textContent="No hay espacios sintéticos habilitados para estos filtros.";return;}
  for(const item of payload.items){
    const card=document.createElement("article");
    const title=document.createElement("h3");title.textContent=`${item.title} · ${item.category_name}`;
    const estimated=item.estimated_total_clp===undefined?"":` · Estimación total ${item.estimated_total_clp.toLocaleString("es-CL")} ${item.currency}`;
    const distance=item.distance_km===undefined?"":` · Distancia directa aprox. ${item.distance_km.toFixed(1)} km (no ruta vial)`;
    const meta=document.createElement("p");meta.textContent=`Tarifa ${item.base_price_clp} ${item.currency}/${item.rate_unit} · ${item.time_zone}${estimated}${distance}${item.available===undefined?"":item.available?" · disponible":" · no disponible"}`;
    const details=document.createElement("button");details.type="button";details.textContent="Ver detalle y preparar cotización";
    details.addEventListener("click",()=>void action(async()=>{
      const detailToken=++bookingCatalogRequest;
      const selectionToken=bookingQuoteState.beginSelection(item.space_id);
      bookingFixture=null;clearWeeklyHoursEditor();resetAvailabilityPicker("Cambiando espacio; las opciones anteriores quedaron invalidadas.");
      (document.querySelector<HTMLInputElement>('#booking-quote-form [name="space_id"]')!).value="";
      (document.querySelector<HTMLInputElement>('#booking-request-form [name="quote_id"]')!).value="";
      (document.querySelector<HTMLInputElement>('#booking-quote-form [name="start_at"]')!).value="";
      (document.querySelector<HTMLInputElement>('#booking-quote-form [name="end_at"]')!).value="";
      bookingQuoteOutput.textContent="Selecciona el intervalo y prepara una nueva cotización para este espacio.";
      const detailResult=await request(`${bookingBase}/catalog/${encodeURIComponent(item.space_id)}`,"GET",undefined,true);
      if(detailToken!==bookingCatalogRequest||!bookingQuoteState.selectionIsCurrent(selectionToken,item.space_id))return;
      bookingFixture=bookingData<BookingFixture>(detailResult);
      (document.querySelector<HTMLInputElement>('#booking-quote-form [name="space_id"]')!).value=bookingFixture.space_id;
      bookingFixtureOutput.textContent=`${String(detailResult.safety_notice)}\n${JSON.stringify(bookingFixture,null,2)}`;
      void renderWeeklyHoursEditor(item.space_id);
      renderAvailabilityPicker();
    }));
    card.append(title,meta,details);catalogResults.append(card);
  }
});
document.querySelector<HTMLFormElement>("#booking-catalog-form")!.addEventListener("input",()=>{
  bookingCatalogRequest++;
  catalogPagination.invalidate();
  catalogNextCursor="";catalogRequestCursor="";catalogNextButton.disabled=true;
  bookingQuoteState.beginSearch();
  bookingAvailabilityState.invalidate();selectedAvailabilityContext=null;resetAvailabilityPicker("Los filtros cambiaron; los intervalos anteriores quedaron invalidados.");
  bookingFixture=null;
  (document.querySelector<HTMLInputElement>('#booking-quote-form [name="space_id"]')!).value="";
  (document.querySelector<HTMLInputElement>('#booking-request-form [name="quote_id"]')!).value="";
  bookingQuoteOutput.textContent="La cotización queda invalidada al cambiar los filtros.";
});
catalogNextButton.addEventListener("click",()=>{
  const cursor=catalogPagination.beginNext();if(!cursor)return;
  catalogRequestCursor=cursor;
  invalidateCatalogSelection("La selección y cotización quedan invalidadas al cambiar de página.");
  document.querySelector<HTMLFormElement>("#booking-catalog-form")!.requestSubmit();
});
document.querySelector<HTMLButtonElement>("#booking-catalog-restart")!.addEventListener("click",()=>{
  resetCatalogTraversal();invalidateCatalogSelection("Búsqueda reiniciada desde la primera página.");
  document.querySelector<HTMLFormElement>("#booking-catalog-form")!.requestSubmit();
});
form("booking-quote-form",async data=>{
  const selectedSpace=String(data.get("space_id")??"");
  if(!bookingFixture||selectedSpace!==bookingFixture.space_id)throw new Error("Selecciona el detalle de un espacio autorizado antes de cotizar.");
  const quoteToken=bookingQuoteState.beginQuote(selectedSpace);
  if(quoteToken===null)throw new Error("La selección cambió. Vuelve a consultar el detalle del espacio antes de cotizar.");
  const startLocal=String(data.get("start_at")),endLocal=String(data.get("end_at"));
  const selectedInterval=selectedAvailabilityContext?bookingAvailabilityState.selectedFor(selectedAvailabilityContext,startLocal,endLocal):null;
  let quoteResponse:Record<string,unknown>;
  try {
    quoteResponse=await request(`${bookingBase}/quotes`,"POST",{space_id:selectedSpace,start_at:selectedInterval?.startAt??localTimeAsUTC(startLocal,bookingFixture.time_zone),end_at:selectedInterval?.endAt??localTimeAsUTC(endLocal,bookingFixture.time_zone)},true);
  } catch(error) {
    if(error instanceof Error&&error.message.includes("HTTP 409")){
      resetAvailabilityPicker("La disponibilidad cambió. Consulta los horarios otra vez antes de cotizar.");
      if(bookingFixture)renderAvailabilityPicker();
    }
    throw error;
  }
  const quote=bookingData<Record<string,unknown>>(quoteResponse);
  if(quote.space_id!==selectedSpace||!bookingQuoteState.acceptQuote(quoteToken,selectedSpace,String(quote.id??"")))return;
  (document.querySelector<HTMLInputElement>('#booking-request-form [name="quote_id"]')!).value=String(quote.id);
  bookingQuoteOutput.textContent=`ENSAYO LOCAL — SIN COBRO REAL\n${JSON.stringify(quote,null,2)}`;
});
form("booking-request-form",async(data,element)=>{
  const quoteID=String(data.get("quote_id")??"");
  if(!bookingQuoteState.canRequest(quoteID,bookingFixture?.space_id??null))throw new Error("La cotización no corresponde al espacio seleccionado o quedó invalidada. Selecciona el espacio y cotiza nuevamente.");
  const result=await request(`${bookingBase}/reservations`,"POST",{quote_id:quoteID},true,reservationKey);
  const item=bookingData<TrialReservation>(result);reservationKey=crypto.randomUUID();element.reset();
  selectedReservationID=item.id;
  await loadBookingInbox();
  resultElement.textContent="Solicitud creada; quedó seleccionada en tu bandeja local.";
});
const reservationStates:Record<string,string>={pendiente_de_pago:"Pendiente de pago",pagada:"Pagada · espera decisión del anfitrión",aprobada_host:"Aprobada por anfitrión",cancelada_por_pago:"Cancelada por rechazo del pago simulado",rechazada_arrendador:"Rechazada por anfitrión",vencida_pago:"Vencida por falta de pago",vencida_host:"Vencida por falta de decisión del anfitrión",cancelada_arrendatario:"Cancelada por arrendatario"};
function reservationState(state:string):string{return reservationStates[state]??state}
function bookingDate(value:string,zone:string):string{
  try{return new Intl.DateTimeFormat("es-CL",{dateStyle:"medium",timeStyle:"short",timeZone:zone}).format(new Date(value))}catch{return value}
}
function reservationSummary(item:TrialReservation,role:"renter"|"host"):string{
  const when=`${bookingDate(item.start_at,item.time_zone)}–${bookingDate(item.end_at,item.time_zone)} (${item.time_zone})`;
  let attention="";
  if(role==="host"&&item.state==="pagada")attention=` · ${item.host_expires_at?`espera tu decisión hasta ${bookingDate(item.host_expires_at,item.time_zone)}`:"espera tu decisión"}`;
  else if(role==="renter"&&item.state==="pendiente_de_pago")attention=` · pago vence ${bookingDate(item.pay_expires_at,item.time_zone)}`;
  else if(item.state==="vencida_pago")attention=` · pago venció ${bookingDate(item.pay_expires_at,item.time_zone)}`;
  else if(item.state==="vencida_host"&&item.host_expires_at)attention=` · plazo del anfitrión venció ${bookingDate(item.host_expires_at,item.time_zone)}`;
  const unread=item.unread_count>0?` · ${item.unread_count} mensaje${item.unread_count===1?"":"s"} sin leer`:"";
  return `Espacio ${item.space_id.slice(0,8)} · ${reservationState(item.state)}${attention}${unread}\n${item.subtotal_clp.toLocaleString("es-CL")} ${item.currency} · ${when}`;
}
function renderReservationList(target:HTMLElement,items:TrialReservation[],role:"renter"|"host"):void{
  target.replaceChildren();
  if(!items.length){target.textContent="No tienes reservas en este rol.";return;}
  const ordered=role==="host"?[...items].sort((a,b)=>Number(b.state==="pagada")-Number(a.state==="pagada")):items;
  for(const item of ordered){
    const article=document.createElement("article");
    const summary=document.createElement("p");summary.textContent=reservationSummary(item,role);
    const choose=document.createElement("button");choose.type="button";choose.textContent="Consultar detalle e historial";
    choose.addEventListener("click",()=>void action(async()=>{await loadReservationDetail(item.id);resultElement.textContent="Detalle e historial de la reserva seleccionada.";}));
    article.append(summary,choose);target.append(article);
  }
}
function renderReservationDetail(item:TrialDetail):void{
  const deadline=["pendiente_de_pago","vencida_pago"].includes(item.state)?`Vencimiento de pago: ${bookingDate(item.pay_expires_at,item.time_zone)}${item.state==="vencida_pago"?" (vencido)":""}`:["pagada","vencida_host"].includes(item.state)&&item.host_expires_at?`Vencimiento de respuesta del anfitrión: ${bookingDate(item.host_expires_at,item.time_zone)}${item.state==="vencida_host"?" (vencido)":""}`:"Sin vencimiento pendiente.";
  const history=item.history.map(entry=>`${entry.sequence}. ${reservationState(entry.to)} · ${bookingDate(entry.at,item.time_zone)} · ${entry.reason}`).join("\n");
  const refund=item.refund_state?`\nDevolución simulada: ${item.refund_state} · ${(item.refund_amount_clp??0).toLocaleString("es-CL")} ${item.currency} · ${item.refund_last_result??"sin intento"} · operación ${item.refund_operation_id}`:"";
  bookingHistoryOutput.textContent=`ENSAYO LOCAL — SIN COBRO REAL\nEspacio: ${item.space_id}\nPrecio: ${item.subtotal_clp.toLocaleString("es-CL")} ${item.currency} (${item.units} × ${item.unit_price_clp.toLocaleString("es-CL")} por ${item.rate_unit})\nIntervalo: ${bookingDate(item.start_at,item.time_zone)}–${bookingDate(item.end_at,item.time_zone)} (${item.time_zone})\nPolítica snapshot: ${item.cancellation_policy_version}\nEstado: ${reservationState(item.state)}\n${deadline}${refund}\n\nHistorial:\n${history||"Sin transiciones."}`;
  refreshBookingActions();
}
function clearConversation(message:string):void{
  conversationRevision++;
  conversationOlderCursor=null;
  conversationMessages=[];
  pendingMessageKey="";pendingMessageBody="";
  conversationOutput.replaceChildren();
  conversationStatus.textContent=message;
  const older=document.querySelector<HTMLButtonElement>("#booking-conversation-older")!;
  older.hidden=true;older.disabled=true;
  refreshConversationControls();
}
function clearBookingInboxOnSessionLoss():void{
  sessionAccountID="";selectedReservationID="";selectedReservation=null;resetCatalogTraversal();
  bookingInboxRevision++;
  cancellationPreview=null;cancellationPreviewReservationID="";
  document.querySelector<HTMLElement>("#booking-inbox-cancel-preview-output")!.textContent="Inicia sesión para consultar la cancelación.";
  renterInbox.textContent="Inicia sesión y actualiza tu bandeja.";
  hostInbox.textContent="Inicia sesión y actualiza tu bandeja.";
  bookingHistoryOutput.textContent="Inicia sesión para consultar reservas propias.";
  clearConversation("La sesión terminó; inicia sesión para consultar conversaciones.");
  refreshBookingActions();
}
function refreshConversationControls():void{
  const body=document.querySelector<HTMLTextAreaElement>("#booking-conversation-body");
  const send=document.querySelector<HTMLButtonElement>("#booking-conversation-send");
  const older=document.querySelector<HTMLButtonElement>("#booking-conversation-older");
  if(!body||!send||!older)return;
  const participant=Boolean(selectedReservation&&sessionAccountID&&(selectedReservation.host_id===sessionAccountID||selectedReservation.renter_id===sessionAccountID));
  const writable=participant&&["pendiente_de_pago","pagada","aprobada_host"].includes(selectedReservation!.state);
  body.disabled=!writable;send.disabled=!writable;
  older.disabled=!participant||conversationOlderCursor===null;
}
function renderConversation():void{
  conversationOutput.replaceChildren();
  for(const item of conversationMessages){
    const article=document.createElement("article");
    const header=document.createElement("p");header.textContent=`${item.author_id===sessionAccountID?"Tú":"Participante"} · ${bookingDate(item.created_at,selectedReservation?.time_zone??"UTC")} · #${item.sequence}`;
    const body=document.createElement("p");body.textContent=item.body;
    article.append(header,body);conversationOutput.append(article);
  }
  const older=document.querySelector<HTMLButtonElement>("#booking-conversation-older")!;
  older.hidden=conversationOlderCursor===null;older.disabled=conversationOlderCursor===null;
}
async function loadConversationPage(id:string,before:number|null,prepend:boolean):Promise<void>{
  const token=++conversationRevision;
  const query=new URLSearchParams({limit:"30"});if(before!==null)query.set("before",String(before));
  const response=await request(`${bookingBase}/reservations/${encodeURIComponent(id)}/messages?${query}`,"GET",undefined,true);
  if(token!==conversationRevision||selectedReservationID!==id)return;
  const page=bookingData<ConversationPage>(response);
  if(prepend){
    const existing=new Set(conversationMessages.map(item=>item.sequence));
    conversationMessages=[...page.items.filter(item=>!existing.has(item.sequence)),...conversationMessages].sort((a,b)=>a.sequence-b.sequence);
    conversationOlderCursor=page.older_cursor;
    renderConversation();refreshConversationControls();
    return;
  }
  await showThenMarkConversationPage(
    async()=>page,
    loaded=>{
      if(token!==conversationRevision||selectedReservationID!==id)return false;
      conversationMessages=loaded.items;
      conversationOlderCursor=loaded.older_cursor;
      conversationStatus.textContent=conversationMessages.length?"Solo los dos participantes pueden ver este hilo. Los mensajes no se borran automáticamente en el prototipo local.":"Aún no hay mensajes. La conversación queda ligada a esta reserva.";
      renderConversation();refreshConversationControls();
      return true;
    },
    async through=>{
      await request(`${bookingBase}/reservations/${encodeURIComponent(id)}/messages/read`,"POST",{through_sequence:through},true);
      await loadBookingInbox(false);
    }
  );
}
function refreshBookingActions():void{
  const pay=document.querySelector<HTMLButtonElement>("#booking-inbox-pay");
  const cancel=document.querySelector<HTMLButtonElement>("#booking-inbox-cancel");
  const cancelPreview=document.querySelector<HTMLButtonElement>("#booking-inbox-cancel-preview");
  const refund=document.querySelector<HTMLButtonElement>("#booking-inbox-refund");
  const approve=document.querySelector<HTMLButtonElement>("#booking-inbox-approve");
  const reject=document.querySelector<HTMLButtonElement>("#booking-inbox-reject");
  if(!pay||!cancel||!cancelPreview||!refund||!approve||!reject)return;
  const now=Date.now();
  const allowed=selectedReservation?inboxActions(sessionAccountID,selectedReservation,now):null;
  pay.disabled=!allowed?.canPay;
  cancelPreview.disabled=!allowed?.canCancel;
  cancel.disabled=!allowed?.canCancel||cancellationPreviewReservationID!==selectedReservationID||!cancellationPreview?.eligible;
  refund.disabled=!selectedReservation||selectedReservation.renter_id!==sessionAccountID||selectedReservation.state!=="cancelada_arrendatario"||selectedReservation.refund_state!=="pendiente"||!selectedReservation.refund_operation_id;
  approve.disabled=!allowed?.canDecide;
  const rejectReason=(document.querySelector<HTMLInputElement>("#booking-inbox-reject-reason")?.value??"").trim();
  reject.disabled=!allowed?.canDecide||!rejectReason;
  const outcome=document.querySelector<HTMLSelectElement>("#booking-inbox-payment-outcome");if(outcome)outcome.disabled=!allowed?.canPay;
}
async function loadReservationDetail(id:string):Promise<void>{
  selectedReservationID=id;selectedReservation=null;refreshBookingActions();
  cancellationPreview=null;cancellationPreviewReservationID="";
  document.querySelector<HTMLElement>("#booking-inbox-cancel-preview-output")!.textContent="Consulta la opción de cancelación antes de confirmar.";
  clearConversation("Cargando mensajes de la reserva seleccionada…");
  const revision=++bookingInboxRevision;
  const result=await request(`${bookingBase}/reservations/${encodeURIComponent(id)}`,"GET",undefined,true);
  if(revision!==bookingInboxRevision||selectedReservationID!==id)return;
  selectedReservation=bookingData<TrialDetail>(result);renderReservationDetail(selectedReservation);
  await loadConversationPage(id,null,false);
  conversationStatus.textContent=`Conversación local · ${selectedReservation.state}. ${["pendiente_de_pago","pagada","aprobada_host"].includes(selectedReservation.state)?"Puedes enviar texto plano en este estado.":"Solo lectura: el estado de la reserva no permite enviar."}`;
  refreshConversationControls();
}
async function loadBookingInbox(reloadSelected=true):Promise<void>{
  const revision=++bookingInboxRevision;
  if(!sessionToken||!sessionAccountID){renterInbox.textContent="Inicia sesión y actualiza tu bandeja.";hostInbox.textContent="Inicia sesión y actualiza tu bandeja.";return;}
  const result=await request(`${bookingBase}/reservations`,"GET",undefined,true);
  if(revision!==bookingInboxRevision)return;
  const reservations=bookingData<{items:TrialReservation[]}>(result).items;
  const renterRows=reservations.filter(item=>item.renter_id===sessionAccountID);
  const hostRows=reservations.filter(item=>item.host_id===sessionAccountID);
  renderReservationList(renterInbox,renterRows,"renter");renderReservationList(hostInbox,hostRows,"host");
  const current=reservations.find(item=>item.id===selectedReservationID);
  if(current){if(reloadSelected)await loadReservationDetail(current.id);}
  else if(selectedReservationID&&reloadSelected){selectedReservationID="";selectedReservation=null;bookingHistoryOutput.textContent="La reserva seleccionada ya no está en tu bandeja.";refreshBookingActions();}
  else if(!selectedReservation)refreshBookingActions();
}
document.querySelector<HTMLButtonElement>("#booking-inbox-load")!.addEventListener("click",()=>void action(async()=>{
  await loadBookingInbox();resultElement.textContent="Bandeja local actualizada desde la API.";
}));
async function performSelectedBookingAction(path:string,method:string,body?:unknown,key?:string):Promise<void>{
  const id=selectedReservationID;
  if(!id||!selectedReservation)throw new Error("Selecciona una reserva de tu bandeja primero.");
  try{await request(`${bookingBase}/reservations/${encodeURIComponent(id)}${path}`,method,body,true,key);}
  catch(error){try{await loadBookingInbox();}catch{/* Preserve the original conflict/error for the user. */}throw error;}
  await loadBookingInbox();
  resultElement.textContent="Operación local completada; detalle e historial actualizados desde la API.";
}
document.querySelector<HTMLButtonElement>("#booking-inbox-pay")!.addEventListener("click",()=>void action(async()=>{
  const id=selectedReservationID;let key=paymentKeys.get(id);if(!key){key=crypto.randomUUID();paymentKeys.set(id,key);}
  const outcome=document.querySelector<HTMLSelectElement>("#booking-inbox-payment-outcome")!.value;
  await performSelectedBookingAction("/payment","POST",{outcome},key);
}));
document.querySelector<HTMLInputElement>("#booking-inbox-reject-reason")!.addEventListener("input",refreshBookingActions);
document.querySelector<HTMLButtonElement>("#booking-inbox-cancel-preview")!.addEventListener("click",()=>void action(async()=>{
  const id=selectedReservationID;if(!id||!selectedReservation)throw new Error("Selecciona una reserva primero.");
  cancellationPreview=null;cancellationPreviewReservationID="";refreshBookingActions();
  const response=await request(`${bookingBase}/reservations/${encodeURIComponent(id)}/cancellation-preview`,"GET",undefined,true);
  if(selectedReservationID!==id)return;
  cancellationPreview=bookingData<CancellationPreview>(response);cancellationPreviewReservationID=id;
  const preview=cancellationPreview;
  document.querySelector<HTMLElement>("#booking-inbox-cancel-preview-output")!.textContent=`ENSAYO LOCAL — SIN COBRO REAL\nPolítica: ${preview.policy_version}\nPlazo: ${bookingDate(preview.deadline,selectedReservation.time_zone)} (estrictamente antes)\nImporte: ${preview.amount_clp.toLocaleString("es-CL")} ${preview.currency}\n${preview.refund_label}\nElegible: ${preview.eligible?"Sí":"No"}${preview.reason_code?` · ${preview.reason_code}`:""}`;
  refreshBookingActions();
}));
document.querySelector<HTMLButtonElement>("#booking-inbox-cancel")!.addEventListener("click",()=>void action(async()=>{
  const id=selectedReservationID;if(!id||!selectedReservation||cancellationPreviewReservationID!==id||!cancellationPreview?.eligible)throw new Error("Consulta nuevamente las condiciones antes de confirmar la cancelación.");
  let key=cancellationKeys.get(id);if(!key){key=crypto.randomUUID();cancellationKeys.set(id,key);}
  const reason=document.querySelector<HTMLInputElement>("#booking-inbox-cancel-reason")!.value;
  await performSelectedBookingAction("/cancel","POST",{reason},key);
  cancellationPreview=null;cancellationPreviewReservationID="";
}));
document.querySelector<HTMLButtonElement>("#booking-inbox-refund")!.addEventListener("click",()=>void action(async()=>{
  const id=selectedReservationID,operationID=selectedReservation?.refund_operation_id;
  if(!id||!operationID)throw new Error("No hay una obligación de devolución pendiente para esta reserva.");
  const outcome=document.querySelector<HTMLSelectElement>("#booking-inbox-refund-outcome")!.value;
  await performSelectedBookingAction("/refund","POST",{outcome},operationID);
}));
document.querySelector<HTMLButtonElement>("#booking-inbox-approve")!.addEventListener("click",()=>void action(async()=>performSelectedBookingAction("/decision","POST",{decision:"aprobar"})));
document.querySelector<HTMLButtonElement>("#booking-inbox-reject")!.addEventListener("click",()=>void action(async()=>{
  const reason=document.querySelector<HTMLInputElement>("#booking-inbox-reject-reason")!.value.trim();
  if(!reason)throw new Error("Escribe el motivo de rechazo requerido.");
  await performSelectedBookingAction("/decision","POST",{decision:"rechazar",reason});
}));
document.querySelector<HTMLButtonElement>("#booking-conversation-older")!.addEventListener("click",()=>void action(async()=>{
  const id=selectedReservationID,cursor=conversationOlderCursor;
  if(!id||cursor===null)throw new Error("No hay mensajes anteriores para cargar.");
  await loadConversationPage(id,cursor,true);
}));
form("booking-conversation-form",async(data,element)=>{
  const id=selectedReservationID;
  if(!id||!selectedReservation)throw new Error("Selecciona una reserva de tu bandeja primero.");
  const body=String(data.get("body")??"");
  if(!["pendiente_de_pago","pagada","aprobada_host"].includes(selectedReservation.state))throw new Error("Este estado conserva lectura y no permite enviar mensajes.");
  if(!pendingMessageKey||pendingMessageBody!==body){pendingMessageKey=crypto.randomUUID();pendingMessageBody=body;}
  await request(`${bookingBase}/reservations/${encodeURIComponent(id)}/messages`,"POST",{body},true,pendingMessageKey);
  if(selectedReservationID!==id)return;
  pendingMessageKey="";pendingMessageBody="";element.reset();
  await loadConversationPage(id,null,false);
  resultElement.textContent="Mensaje sintético guardado en la conversación de la reserva.";
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
