export function captureAdminLocalContext(account, token, generation, revision) { return { account, token, generation, revision }; }
export function captureAdminLocalSession(context) { return { account: context.account, token: context.token, generation: context.generation }; }
export function adminLocalSessionMatches(context, current) { return Boolean(context.token && context.account === current.account && context.token === current.token && context.generation === current.generation); }
export function adminLocalContextMatches(context, current) { return Boolean(context.token && current.isAdmin && context.account === current.account && context.token === current.token && context.generation === current.generation && context.revision === current.revision); }
export function isAdminLocalForbiddenError(error) { return error instanceof Error && /\(HTTP 403(?:,|\))/.test(error.message); }
export async function handleAdminLocalForbidden(context, current, clearView, refreshSession, applySession) {
    if (!adminLocalSessionMatches(context, current()))
        return "stale";
    clearView("Acceso administrativo denegado; se están actualizando los permisos de esta sesión.");
    const currentSession = current();
    applySession({ roles: currentSession.roles.filter(role => role !== "administrador"), restrictedMode: currentSession.restrictedMode });
    if (!adminLocalSessionMatches(context, current()))
        return "stale";
    let snapshot;
    try {
        snapshot = await refreshSession(context.token);
    }
    catch {
        if (!adminLocalSessionMatches(context, current()))
            return "stale";
        clearView("No se pudieron actualizar los permisos; la sesión continúa disponible para operaciones permitidas.");
        return "refreshed";
    }
    if (!adminLocalSessionMatches(context, current()))
        return "stale";
    applySession(snapshot);
    if (!snapshot.roles.includes("administrador") || snapshot.restrictedMode) {
        clearView("El acceso administrativo ya no está disponible para esta sesión.");
    }
    return "refreshed";
}
