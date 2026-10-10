export async function consumeAdminAuditExportResponse(response, state, context, current, onUnauthorized, onForbidden) {
    if (!response.ok) {
        let code = "unknown";
        try {
            const body = await response.json();
            code = body.error?.code ?? code;
        }
        catch { /* response body may be unavailable */ }
        if (!current(context))
            return { kind: "stale" };
        if (response.status === 401) {
            onUnauthorized();
            return { kind: "unauthorized", code };
        }
        if (response.status === 403) {
            await onForbidden();
            return { kind: "forbidden", code };
        }
        return { kind: "failed", status: response.status, code };
    }
    const blob = await response.blob();
    if (!current(context))
        return { kind: "stale" };
    return { kind: "download", blob };
}
