export function apiErrorMessage(body, status) {
    const code = body.error?.code ?? "unknown";
    const message = body.error?.message ?? "Error de API";
    if (status === 403 && code === "restricted_account") {
        return `Acceso restringido: ${message} (HTTP 403).`;
    }
    return `${message} (HTTP ${status}, ${code})`;
}
