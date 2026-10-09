export function damageClaimRequestIsCurrent(context, current) {
    return Boolean(context.token) && context.requestRevision === current.requestRevision && context.selectionRevision === current.selectionRevision && context.selectionID === current.selectionID && context.accountID === current.accountID && context.token === current.token && context.generation === current.generation;
}
export async function finishDamageClaimAfterReload(context, reload, current, onSuccess) {
    await reload();
    if (!damageClaimRequestIsCurrent(context, current()))
        return false;
    onSuccess();
    return true;
}
