import { ProfileRequestGate, profileMatchesSelection } from "./profile-request.js";
import { CalendarRequestState } from "./calendar-request.js";
const statusElement = document.querySelector("#api-status");
const resultElement = document.querySelector("#result");
let apiBase = "";
let sessionToken = "";
let termIDs = [];
let evidenceObjectURL = "";
async function request(path, method = "GET", body, authenticated = false, idempotencyKey) {
    if (!apiBase)
        throw new Error("API local aún no disponible.");
    const headers = { Accept: "application/json" };
    if (body !== undefined)
        headers["Content-Type"] = "application/json";
    if (authenticated) {
        if (!sessionToken)
            throw new Error("Primero inicia sesión.");
        headers.Authorization = `Bearer ${sessionToken}`;
    }
    if (idempotencyKey)
        headers["Idempotency-Key"] = idempotencyKey;
    const endpoint = path.startsWith("/") ? `${apiBase}${path}` : `${apiBase}/api/v1/auth/${path}`;
    const response = await fetch(endpoint, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), mode: "cors", cache: "no-store", credentials: "omit" });
    const data = response.status === 204 ? {} : await response.json();
    if (!response.ok) {
        if (authenticated && (response.status === 401 || path === "password/change" && response.status === 503))
            sessionToken = "";
        const error = data;
        throw new Error(`${error.error?.message ?? "Error de API"} (HTTP ${response.status}, ${error.error?.code ?? "unknown"})`);
    }
    return data;
}
async function action(work) {
    const buttons = document.querySelectorAll("button");
    buttons.forEach(button => button.disabled = true);
    try {
        await work();
    }
    catch (error) {
        resultElement.textContent = error instanceof Error ? error.message : "No se pudo conectar con la API.";
    }
    finally {
        buttons.forEach(button => button.disabled = false);
        refreshCalendarControls();
    }
}
function form(id, work) {
    const element = document.querySelector(`#${id}`);
    element.addEventListener("submit", event => { event.preventDefault(); void action(() => work(new FormData(element), element)); });
}
form("register-form", async (data, element) => {
    if (!termIDs.length || !data.get("terms"))
        throw new Error("Debes aceptar los términos de prueba.");
    const response = await request("register", "POST", { email: data.get("email"), password: data.get("password"), terms_version_ids: termIDs });
    element.querySelector('[name="password"]').value = "";
    resultElement.textContent = response.status === "verification_pending" ? String(response.message) : "Cuenta creada. Abre el buzón de desarrollo para verificar el correo.";
});
form("verify-form", async (data, element) => {
    await request("verification", "POST", { token_id: data.get("token_id"), token: data.get("token") });
    element.reset();
    resultElement.textContent = "Correo verificado. Ya puedes iniciar sesión.";
});
form("reissue-form", async (data) => { await request("verification/reissue", "POST", { email: data.get("email") }); resultElement.textContent = "Verificación reenviada al buzón local; el token anterior queda invalidado."; });
form("login-form", async (data, element) => {
    const response = await request("login", "POST", { email: data.get("email"), password: data.get("password") });
    sessionToken = String(response.access_token);
    element.querySelector('[name="password"]').value = "";
    await loadSpaceCategories();
    resultElement.textContent = "Sesión iniciada. Puedes consultarla o cerrarla.";
});
form("recovery-request-form", async (data, element) => {
    await request("password/recovery", "POST", { email: data.get("email") });
    element.reset();
    resultElement.textContent = "Si la cuenta es elegible, recibirás instrucciones en el buzón local.";
});
form("recovery-consume-form", async (data, element) => {
    await request("password/recovery/consume", "POST", { token_id: data.get("token_id"), token: data.get("token"), new_password: data.get("new_password"), confirm_password: data.get("confirm_password") });
    sessionToken = "";
    element.reset();
    resultElement.textContent = "Contraseña actualizada y sesiones cerradas. Inicia sesión con la nueva contraseña.";
});
form("password-change-form", async (data, element) => {
    await request("password/change", "POST", { current_password: data.get("current_password"), new_password: data.get("new_password"), confirm_password: data.get("confirm_password") }, true);
    sessionToken = "";
    element.reset();
    document.querySelector("#session-output").textContent = "Sesión revocada por cambio de contraseña.";
    resultElement.textContent = "Contraseña actualizada. Inicia sesión otra vez; se notificó al buzón local.";
});
document.querySelector("#session-button").addEventListener("click", () => void action(async () => {
    const response = await request("session", "GET", undefined, true);
    document.querySelector("#session-output").textContent = JSON.stringify(response, null, 2);
    resultElement.textContent = "Sesión válida; estado y roles comprobados por la API.";
}));
document.querySelector("#logout-button").addEventListener("click", () => void action(async () => {
    await request("logout", "POST", undefined, true);
    sessionToken = "";
    document.querySelector("#session-output").textContent = "Sesión cerrada.";
    resultElement.textContent = "Logout completado. La credencial anterior queda revocada.";
}));
document.querySelector("#profile-load").addEventListener("click", () => void action(async () => {
    const profile = await request("/api/v1/profile", "GET", undefined, true);
    const form = document.querySelector("#profile-form");
    form.querySelector('[name="display_name"]').value = String(profile.display_name ?? "");
    form.querySelector('[name="phone"]').value = String(profile.phone ?? "");
    document.querySelector("#privacy-output").textContent = JSON.stringify(profile, null, 2);
}));
form("profile-form", async (data) => {
    const profile = await request("/api/v1/profile", "PUT", { display_name: data.get("display_name"), phone: data.get("phone") }, true);
    document.querySelector("#privacy-output").textContent = JSON.stringify(profile, null, 2);
    resultElement.textContent = "Perfil guardado para la cuenta de la sesión actual.";
});
form("rights-form", async (data, element) => {
    const item = await request("/api/v1/rights-requests", "POST", { type: data.get("type") }, true);
    element.reset();
    document.querySelector("#privacy-output").textContent = JSON.stringify(item, null, 2);
    resultElement.textContent = "Solicitud registrada para revisión; no se han borrado datos.";
});
document.querySelector("#rights-load").addEventListener("click", () => void action(async () => {
    const items = await request("/api/v1/rights-requests", "GET", undefined, true);
    document.querySelector("#privacy-output").textContent = JSON.stringify(items, null, 2);
}));
form("verification-form", async (data) => {
    const item = await request("/api/v1/verifications", "POST", { type: data.get("type") }, true, crypto.randomUUID());
    document.querySelector("#verification-output").textContent = JSON.stringify(item, null, 2);
    resultElement.textContent = "Solicitud sintética creada; requiere revisión autorizada. No acredita identidad.";
});
document.querySelector("#verification-load").addEventListener("click", () => void action(async () => {
    const items = await request("/api/v1/verifications", "GET", undefined, true);
    document.querySelector("#verification-output").textContent = JSON.stringify(items, null, 2);
}));
const evidenceCase = document.querySelector("#evidence-case-id");
const evidenceOutput = document.querySelector("#evidence-output");
const evidenceItems = document.querySelector("#evidence-items");
async function evidenceContent(caseID, evidenceID, admin = false) {
    if (!sessionToken)
        throw new Error("Primero inicia sesión.");
    const path = admin
        ? `/api/v1/admin/verifications/${encodeURIComponent(caseID)}/evidence/${encodeURIComponent(evidenceID)}`
        : `/api/v1/verifications/${encodeURIComponent(caseID)}/evidence/${encodeURIComponent(evidenceID)}`;
    const response = await fetch(`${apiBase}${path}`, { headers: { Authorization: `Bearer ${sessionToken}`, Accept: "image/png" }, mode: "cors", cache: "no-store", credentials: "omit" });
    if (!response.ok) {
        const data = await response.json();
        throw new Error(`${data.error?.message ?? "Error de API"} (HTTP ${response.status}, ${data.error?.code ?? "unknown"})`);
    }
    const blob = await response.blob();
    if (evidenceObjectURL)
        URL.revokeObjectURL(evidenceObjectURL);
    evidenceObjectURL = URL.createObjectURL(blob);
    const preview = document.querySelector("#evidence-preview");
    preview.src = evidenceObjectURL;
    preview.hidden = false;
    evidenceOutput.textContent = `Fixture ${evidenceID} cargado (${blob.size} bytes, ${response.headers.get("ETag") ?? "sin hash"}).`;
}
document.querySelector("#evidence-upload").addEventListener("click", () => void action(async () => {
    const caseID = evidenceCase.value.trim();
    if (!caseID)
        throw new Error("Indica el ID de tu caso.");
    const item = await request(`/api/v1/verifications/${encodeURIComponent(caseID)}/evidence`, "POST", { fixture_code: "synthetic-png-v1" }, true);
    evidenceOutput.textContent = `Fixture privado guardado. Metadatos: ${JSON.stringify(item, null, 2)}`;
    await loadEvidence();
}));
document.querySelector("#evidence-invalid").addEventListener("click", () => void action(async () => {
    const caseID = evidenceCase.value.trim();
    if (!caseID)
        throw new Error("Indica el ID de tu caso.");
    await request(`/api/v1/verifications/${encodeURIComponent(caseID)}/evidence`, "POST", { fixture_code: "not-a-synthetic-fixture" }, true);
}));
async function loadEvidence() {
    const caseID = evidenceCase.value.trim();
    if (!caseID)
        throw new Error("Indica el ID de tu caso.");
    const response = await request(`/api/v1/verifications/${encodeURIComponent(caseID)}/evidence`, "GET", undefined, true);
    const items = (response.items ?? []);
    evidenceItems.replaceChildren();
    for (const item of items) {
        const row = document.createElement("p"), view = document.createElement("button"), remove = document.createElement("button");
        row.append(document.createTextNode(`${item.id} · ${item.size_bytes} bytes · ${item.created_at} `));
        view.type = "button";
        view.textContent = "Consultar PNG";
        view.addEventListener("click", () => void action(() => evidenceContent(caseID, item.id)));
        remove.type = "button";
        remove.textContent = "Eliminar fixture";
        remove.addEventListener("click", () => void action(async () => {
            await request(`/api/v1/verifications/${encodeURIComponent(caseID)}/evidence/${encodeURIComponent(item.id)}`, "DELETE", undefined, true);
            evidenceOutput.textContent = `Fixture ${item.id} eliminado explícitamente.`;
            await loadEvidence();
        }));
        row.append(view, remove);
        evidenceItems.append(row);
    }
    evidenceOutput.textContent = `${items.length} fixture(s) privado(s): ${JSON.stringify(items, null, 2)}`;
}
document.querySelector("#evidence-load").addEventListener("click", () => void action(loadEvidence));
form("review-evidence-form", async (data) => {
    const caseID = String(data.get("case_id"));
    const response = await request(`/api/v1/admin/verifications/${encodeURIComponent(caseID)}/evidence`, "GET", undefined, true);
    const items = (response.items ?? []);
    const output = document.querySelector("#review-evidence-output");
    output.textContent = JSON.stringify(items, null, 2);
    if (items[0])
        await evidenceContent(caseID, items[0].id, true);
});
form("verification-retry-form", async (data, element) => {
    const id = String(data.get("id"));
    const item = await request(`/api/v1/verifications/${encodeURIComponent(id)}/retry`, "POST", { corrected: data.get("corrected") === "on" }, true, crypto.randomUUID());
    document.querySelector("#verification-output").textContent = JSON.stringify(item, null, 2);
    element.reset();
    resultElement.textContent = "Reintento fixture creado y en revisión.";
});
document.querySelector("#review-load").addEventListener("click", () => void action(async () => {
    const items = await request("/api/v1/admin/verifications", "GET", undefined, true);
    document.querySelector("#review-output").textContent = JSON.stringify(items, null, 2);
}));
form("review-form", async (data, element) => {
    const id = String(data.get("id")), decision = String(data.get("decision"));
    const item = await request(`/api/v1/admin/verifications/${encodeURIComponent(id)}/review`, "POST", { decision, reason_code: data.get("reason_code") }, true);
    document.querySelector("#review-output").textContent = JSON.stringify(item, null, 2);
    element.reset();
    resultElement.textContent = "Revisión fixture registrada.";
});
const spacesOutput = document.querySelector("#space-output");
const spaceForm = document.querySelector("#space-form");
const spacesList = document.querySelector("#spaces-list");
let currentDraftID = "";
let currentProfile = null;
const profileRequestGate = new ProfileRequestGate();
async function loadSpaceCategories() {
    const categoryResult = await request("/api/v1/spaces/categories", "GET", undefined, true);
    const categories = categoryResult.items;
    const categorySelect = document.querySelector("#space-category");
    categorySelect.replaceChildren(new Option("Selecciona categoría", ""));
    for (const category of categories)
        categorySelect.add(new Option(category.name, category.code));
    categorySelect.addEventListener("change", () => void action(async () => { await loadAttributeProfile(categorySelect.value); }));
    if (categorySelect.value)
        await loadAttributeProfile(categorySelect.value);
}
async function loadAttributeProfile(category, version) {
    const requestID = profileRequestGate.begin();
    currentProfile = null;
    const root = document.querySelector("#space-attributes");
    root.replaceChildren();
    if (!category)
        return false;
    const versionPath = version === undefined ? "" : `/${version}`;
    const profile = await request(`/api/v1/spaces/categories/${encodeURIComponent(category)}/attributes${versionPath}`, "GET", undefined, true);
    const selectedCategory = document.querySelector("#space-category").value;
    if (!profileRequestGate.accepts(requestID, category, selectedCategory) || profile.category_code !== category || profile.schema_version !== (version ?? profile.schema_version))
        return false;
    currentProfile = profile;
    for (const definition of [...profile.attributes].sort((a, b) => a.order - b.order)) {
        const label = document.createElement("label");
        label.textContent = `${definition.label}${definition.unit ? ` (${definition.unit})` : ""}`;
        let control;
        if (definition.type === "boolean") {
            const select = document.createElement("select");
            select.add(new Option("No declarar", ""));
            select.add(new Option("Sí", "true"));
            select.add(new Option("No", "false"));
            control = select;
        }
        else if (definition.type === "enum" || definition.type === "enum_list") {
            const select = document.createElement("select");
            select.add(new Option("No declarar", ""));
            if (definition.type === "enum_list")
                select.multiple = true;
            for (const option of definition.options ?? [])
                select.add(new Option(option, option));
            control = select;
        }
        else {
            const input = document.createElement("input");
            input.type = "number";
            input.step = definition.type === "integer" ? "1" : String(definition.step ?? "any");
            if (definition.minimum !== undefined)
                input.min = String(definition.minimum);
            if (definition.maximum !== undefined)
                input.max = String(definition.maximum);
            control = input;
        }
        control.name = `attribute:${definition.code}`;
        if (definition.description)
            control.title = definition.description;
        label.append(control);
        root.append(label);
    }
    return true;
}
function spaceInput(data) {
    const selectedCategory = String(data.get("category_code") ?? "");
    if (!currentProfile || !profileMatchesSelection(currentProfile.category_code, selectedCategory))
        throw new Error("Espera a que cargue el perfil de la categoría seleccionada.");
    const attributes = {};
    for (const definition of currentProfile?.attributes ?? []) {
        const key = `attribute:${definition.code}`, raw = data.getAll(key);
        if (definition.type === "enum_list") {
            const values = raw.map(String).filter(Boolean);
            if (values.length)
                attributes[definition.code] = values;
            continue;
        }
        const value = String(raw[0] ?? "");
        if (value === "")
            continue;
        if (definition.type === "boolean")
            attributes[definition.code] = value === "true";
        else if (definition.type === "integer")
            attributes[definition.code] = Number.parseInt(value, 10);
        else if (definition.type === "number")
            attributes[definition.code] = Number(value);
        else
            attributes[definition.code] = value;
    }
    return { title: data.get("title"), description: data.get("description"), area_m2: Number(data.get("area_m2")), category_code: selectedCategory, capacity: Number(data.get("capacity")), usage_rules: data.get("usage_rules"), rate_unit: data.get("rate_unit"), base_price_clp: Number(data.get("base_price_clp")), address: data.get("address"), attribute_schema_version: currentProfile.schema_version, attributes };
}
async function loadSpaces() {
    const result = await request("/api/v1/spaces", "GET", undefined, true);
    const items = result.items;
    spacesList.replaceChildren();
    for (const item of items) {
        const li = document.createElement("li"), button = document.createElement("button");
        button.type = "button";
        button.textContent = `${String(item.title)} · ${String(item.state)}`;
        button.addEventListener("click", () => void action(async () => {
            const draft = await request(`/api/v1/spaces/${encodeURIComponent(String(item.id))}`, "GET", undefined, true);
            currentDraftID = String(draft.id);
            spaceForm.querySelector('[name="draft_id"]').value = currentDraftID;
            for (const key of ["title", "description", "area_m2", "category_code", "capacity", "usage_rules", "rate_unit", "base_price_clp", "address"]) {
                const field = spaceForm.elements.namedItem(key);
                field.value = String(draft[key] ?? "");
            }
            const profileLoaded = await loadAttributeProfile(String(draft.category_code), Number(draft.attribute_schema_version));
            if (!profileLoaded)
                throw new Error("No se pudo cargar el perfil guardado del borrador.");
            for (const definition of currentProfile?.attributes ?? []) {
                const control = spaceForm.elements.namedItem(`attribute:${definition.code}`);
                const value = draft.attributes?.[definition.code];
                if (!control || value === undefined)
                    continue;
                if (definition.type === "enum_list" && control instanceof HTMLSelectElement) {
                    const selected = new Set(value);
                    for (const option of control.options)
                        option.selected = selected.has(option.value);
                }
                else
                    control.value = definition.type === "boolean" ? (value ? "true" : "false") : String(value);
            }
            document.querySelector("#space-save").textContent = "Guardar cambios";
            document.querySelector("#space-cancel").hidden = false;
            spacesOutput.textContent = JSON.stringify(draft, null, 2);
        }));
        li.append(button);
        spacesList.append(li);
    }
    spacesOutput.textContent = JSON.stringify(result, null, 2);
}
document.querySelector("#spaces-load").addEventListener("click", () => void action(loadSpaces));
document.querySelector("#space-cancel").addEventListener("click", () => { spaceForm.reset(); currentDraftID = ""; document.querySelector("#space-save").textContent = "Crear borrador"; document.querySelector("#space-cancel").hidden = true; });
form("space-form", async (data, element) => {
    const id = String(data.get("draft_id") ?? "");
    const draft = await request(id ? `/api/v1/spaces/${encodeURIComponent(id)}` : "/api/v1/spaces", id ? "PUT" : "POST", spaceInput(data), true);
    spacesOutput.textContent = JSON.stringify(draft, null, 2);
    element.reset();
    currentDraftID = "";
    document.querySelector("#space-save").textContent = "Crear borrador";
    document.querySelector("#space-cancel").hidden = true;
    resultElement.textContent = id ? "Borrador guardado." : "Borrador privado creado.";
    await loadSpaces();
});
const calendarSpace = document.querySelector("#calendar-space");
const calendarOutput = document.querySelector("#calendar-output");
const calendarBlocks = document.querySelector("#calendar-blocks");
const calendarRequestState = new CalendarRequestState();
const calendarZoneInput = document.querySelector('#calendar-config-form [name="time_zone"]');
const calendarQueryForm = document.querySelector("#calendar-query-form");
const calendarBlockForm = document.querySelector("#calendar-block-form");
const priceRateForm = document.querySelector("#price-rate-form");
const priceRateOutput = document.querySelector("#price-rate-output");
const priceSimulationOutput = document.querySelector("#price-simulation-output");
function pricePath(suffix) { if (!calendarSpace.value)
    throw new Error("Selecciona uno de tus borradores."); return `/api/v1/spaces/${encodeURIComponent(calendarSpace.value)}/${suffix}`; }
document.querySelector("#price-load").addEventListener("click", () => void action(async () => {
    const spaceID = calendarSpace.value, selection = calendarRequestState.snapshot();
    const [rate, history] = await Promise.all([request(pricePath("tariff"), "GET", undefined, true), request(pricePath("tariffs"), "GET", undefined, true)]);
    if (!calendarRequestState.accepts(selection, spaceID, calendarSpace.value))
        return;
    priceRateForm.querySelector('[name="rate_unit"]').value = String(rate.rate_unit);
    priceRateForm.querySelector('[name="base_price"]').value = String(rate.base_price);
    priceRateOutput.textContent = JSON.stringify({ current: rate, history: (history.items ?? []), private: true }, null, 2);
}));
form("price-rate-form", async (data) => {
    const spaceID = calendarSpace.value, selection = calendarRequestState.snapshot();
    const rate = await request(pricePath("tariff"), "PUT", { rate_unit: data.get("rate_unit"), base_price: Number(data.get("base_price")) }, true);
    if (!calendarRequestState.accepts(selection, spaceID, calendarSpace.value))
        return;
    const history = await request(pricePath("tariffs"), "GET", undefined, true);
    if (!calendarRequestState.accepts(selection, spaceID, calendarSpace.value))
        return;
    priceRateOutput.textContent = JSON.stringify({ current: rate, history: (history.items ?? []), private: true }, null, 2);
    resultElement.textContent = "Tarifa actualizada como nueva versión; las simulaciones guardadas conservan su snapshot.";
});
form("price-simulation-form", async (data) => {
    const spaceID = calendarSpace.value, selection = calendarRequestState.snapshot();
    const zone = calendarRequestState.zoneFor(spaceID, calendarZoneInput.value);
    const simulation = await request(pricePath("price-simulations"), "POST", { start_at: localTimeAsUTC(String(data.get("start_at")), zone), end_at: localTimeAsUTC(String(data.get("end_at")), zone) }, true);
    if (!calendarRequestState.accepts(selection, spaceID, calendarSpace.value))
        return;
    priceSimulationOutput.textContent = JSON.stringify({ ...simulation, label: "SIMULACIÓN PRIVADA" }, null, 2);
    resultElement.textContent = "Simulación privada guardada con su versión de tarifa. No se modificó la ocupación.";
});
form("price-simulation-load-form", async (data) => {
    const spaceID = calendarSpace.value, selection = calendarRequestState.snapshot();
    const simulation = await request(pricePath(`price-simulations/${encodeURIComponent(String(data.get("id")))}`), "GET", undefined, true);
    if (!calendarRequestState.accepts(selection, spaceID, calendarSpace.value))
        return;
    priceSimulationOutput.textContent = JSON.stringify({ ...simulation, label: "SIMULACIÓN PRIVADA · SNAPSHOT ORIGINAL" }, null, 2);
});
function refreshCalendarControls() {
    const dirty = !calendarSpace.value || (() => {
        try {
            calendarRequestState.zoneFor(calendarSpace.value, calendarZoneInput.value);
            return false;
        }
        catch {
            return true;
        }
    })();
    calendarQueryForm.querySelectorAll("button").forEach(button => button.disabled = dirty);
    calendarBlockForm.querySelectorAll("button").forEach(button => button.disabled = dirty);
    document.querySelector("#price-simulation-form").querySelectorAll("button").forEach(button => button.disabled = dirty);
}
async function loadCalendarSpaces() {
    const result = await request("/api/v1/spaces", "GET", undefined, true);
    const items = result.items;
    const selected = calendarSpace.value;
    calendarSpace.replaceChildren(new Option("Selecciona un borrador", ""));
    for (const item of items)
        calendarSpace.add(new Option(String(item.title), String(item.id)));
    if ([...calendarSpace.options].some(option => option.value === selected))
        calendarSpace.value = selected;
}
function calendarPath(spaceID, suffix = "") {
    if (!spaceID)
        throw new Error("Selecciona uno de tus borradores.");
    return `/api/v1/spaces/${encodeURIComponent(spaceID)}/availability${suffix}`;
}
function localTimeAsUTC(value, zone) {
    if (!value || !zone)
        throw new Error("Selecciona fechas y configura una zona horaria IANA.");
    const normalized = value.length === 16 ? `${value}:00` : value;
    const target = Date.parse(`${normalized}Z`);
    if (!Number.isFinite(target))
        throw new Error("La fecha local no es válida.");
    const formatter = new Intl.DateTimeFormat("en-CA", { timeZone: zone, year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hourCycle: "h23" });
    const fieldsAt = (instant) => Object.fromEntries(formatter.formatToParts(new Date(instant)).filter(part => part.type !== "literal").map(part => [part.type, Number(part.value)]));
    let guess = target;
    for (let attempt = 0; attempt < 4; attempt++) {
        const fields = fieldsAt(guess);
        const represented = Date.UTC(fields.year, fields.month - 1, fields.day, fields.hour, fields.minute, fields.second);
        const delta = target - represented;
        if (delta === 0)
            return new Date(guess).toISOString();
        guess += delta;
    }
    throw new Error("La hora local no existe en esa zona por un cambio horario. Elige otra hora.");
}
function renderCalendarBlocks(items, spaceID, selection) {
    if (!calendarRequestState.accepts(selection, spaceID, calendarSpace.value))
        return;
    calendarBlocks.replaceChildren();
    for (const block of items) {
        const li = document.createElement("li"), summary = document.createElement("span");
        const zone = String(block.time_zone), formatter = new Intl.DateTimeFormat("es-CL", { timeZone: zone, dateStyle: "medium", timeStyle: "short" });
        summary.textContent = `${formatter.format(new Date(String(block.start_at)))} – ${formatter.format(new Date(String(block.end_at)))} (${zone}): ${String(block.reason)} `;
        const remove = document.createElement("button");
        remove.type = "button";
        remove.textContent = "Quitar bloqueo";
        remove.addEventListener("click", () => void action(async () => {
            const capturedSpaceID = spaceID;
            const capturedSelection = calendarRequestState.snapshot();
            await request(`${calendarPath(capturedSpaceID, "/blocks")}/${encodeURIComponent(String(block.id))}`, "DELETE", undefined, true);
            if (!calendarRequestState.accepts(capturedSelection, capturedSpaceID, calendarSpace.value))
                return;
            calendarOutput.textContent = "Bloqueo retirado.";
            calendarQueryForm.requestSubmit();
        }));
        li.append(summary, remove);
        calendarBlocks.append(li);
    }
}
document.querySelector("#calendar-spaces-load").addEventListener("click", () => void action(loadCalendarSpaces));
calendarSpace.addEventListener("change", () => void action(async () => {
    const spaceID = calendarSpace.value;
    const selection = calendarRequestState.beginSelection();
    calendarZoneInput.value = "";
    calendarBlocks.replaceChildren();
    calendarOutput.textContent = "";
    refreshCalendarControls();
    if (!spaceID)
        return;
    const result = await request(calendarPath(spaceID, "/timezone"), "GET", undefined, true);
    if (!calendarRequestState.confirm(spaceID, String(result.time_zone), selection, calendarSpace.value))
        return;
    calendarZoneInput.value = String(result.time_zone);
    calendarOutput.textContent = JSON.stringify(result, null, 2);
    refreshCalendarControls();
}));
form("calendar-config-form", async (data) => {
    const spaceID = calendarSpace.value, selection = calendarRequestState.snapshot();
    const result = await request(calendarPath(spaceID), "PUT", { time_zone: data.get("time_zone") }, true);
    if (!calendarRequestState.confirm(spaceID, String(result.time_zone), selection, calendarSpace.value))
        return;
    calendarOutput.textContent = JSON.stringify(result, null, 2);
    calendarZoneInput.value = String(result.time_zone);
    refreshCalendarControls();
});
form("calendar-query-form", async (data) => {
    const spaceID = calendarSpace.value, selection = calendarRequestState.snapshot();
    const zone = calendarRequestState.zoneFor(spaceID, calendarZoneInput.value);
    const query = new URLSearchParams({ from: localTimeAsUTC(String(data.get("from")), zone), to: localTimeAsUTC(String(data.get("to")), zone) });
    const availability = await request(`${calendarPath(spaceID)}?${query}`, "GET", undefined, true);
    if (!calendarRequestState.accepts(selection, spaceID, calendarSpace.value))
        return;
    const blocks = await request(`${calendarPath(spaceID, "/blocks")}?${query}`, "GET", undefined, true);
    if (!calendarRequestState.accepts(selection, spaceID, calendarSpace.value))
        return;
    renderCalendarBlocks((blocks.items ?? []), spaceID, selection);
    calendarOutput.textContent = JSON.stringify({ availability, blocks }, null, 2);
});
form("calendar-block-form", async (data, element) => {
    const spaceID = calendarSpace.value, selection = calendarRequestState.snapshot();
    const zone = calendarRequestState.zoneFor(spaceID, calendarZoneInput.value);
    const result = await request(calendarPath(spaceID, "/blocks"), "POST", { start_at: localTimeAsUTC(String(data.get("start_at")), zone), end_at: localTimeAsUTC(String(data.get("end_at")), zone), reason: data.get("reason") }, true);
    if (!calendarRequestState.accepts(selection, spaceID, calendarSpace.value))
        return;
    calendarOutput.textContent = JSON.stringify(result, null, 2);
    element.reset();
});
calendarZoneInput.addEventListener("input", refreshCalendarControls);
refreshCalendarControls();
async function initialize() {
    try {
        const config = await (await fetch("/config.json", { cache: "no-store" })).json();
        const readyURL = new URL(config.apiReadyURL);
        if (readyURL.protocol !== "http:" && readyURL.protocol !== "https:")
            throw new Error("URL no admitida");
        apiBase = readyURL.origin;
        const ready = await fetch(readyURL, { cache: "no-store", mode: "cors" });
        if (!ready.ok)
            throw new Error("API no disponible");
        const terms = await request("terms");
        const items = terms.items;
        termIDs = items.filter(item => item.type === "terminos").map(item => item.id);
        document.querySelector("#terms-version").textContent = items.map(item => `${item.type}: ${item.code}`).join(" · ");
        statusElement.textContent = "API y PostgreSQL listos.";
    }
    catch {
        statusElement.textContent = "No se pudo comprobar la API local; revisa el entorno y recarga.";
    }
}
void initialize();
