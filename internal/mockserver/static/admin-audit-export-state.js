export async function consumeAdminAuditExportResponse(response, state, context, currentResult, currentSession, onUnauthorized, onForbidden) {
    if (!response.ok) {
        let code = "unknown";
        try {
            const body = await response.json();
            code = body.error?.code ?? code;
        }
        catch { /* response body may be unavailable */ }
        if (response.status === 401) {
            if (!currentSession(context))
                return { kind: "stale" };
            onUnauthorized();
            return { kind: "unauthorized", code };
        }
        if (!currentResult(context))
            return { kind: "stale" };
        if (response.status === 403) {
            await onForbidden();
            return { kind: "forbidden", code };
        }
        return { kind: "failed", status: response.status, code };
    }
    const blob = await response.blob();
    if (!currentResult(context))
        return { kind: "stale" };
    return { kind: "download", blob };
}
