"use strict";
const statusElement = document.querySelector("#api-status");
async function showReadiness() {
    if (!statusElement) {
        return;
    }
    try {
        const configResponse = await fetch("/config.json", { cache: "no-store" });
        if (!configResponse.ok) {
            throw new Error("configuration unavailable");
        }
        const config = (await configResponse.json());
        const readyURL = new URL(config.apiReadyURL);
        if (readyURL.protocol !== "http:" && readyURL.protocol !== "https:") {
            throw new Error("unsupported API URL");
        }
        const response = await fetch(readyURL, {
            headers: { Accept: "application/json" },
            mode: "cors",
            cache: "no-store",
        });
        const readiness = (await response.json());
        statusElement.textContent = response.ok && readiness.status === "ready"
            ? "API y PostgreSQL listos."
            : `API o PostgreSQL no disponible (HTTP ${response.status}).`;
    }
    catch {
        statusElement.textContent = "No se pudo comprobar la API local.";
    }
}
void showReadiness();
