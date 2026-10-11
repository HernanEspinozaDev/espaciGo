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
export async function finishDamageClaimAfterEvidence(context, createEvidence, reload, current, onSuccess) {
    await createEvidence();
    if (!damageClaimRequestIsCurrent(context, current()))
        return false;
    return finishDamageClaimAfterReload(context, reload, current, onSuccess);
}
// A defense may already have been persisted while PNG creation failed, or the
// PNG may have committed while its HTTP response was lost. Read the operations
// list first so a recovery action never resends the defense text or duplicates
// a confirmed attachment.
export async function createOrRecoverDamageDefenseEvidence(context, hasConfirmedEvidence, createEvidence, reload, current, onSuccess) {
    const confirmed = await hasConfirmedEvidence();
    if (!damageClaimRequestIsCurrent(context, current()))
        return false;
    if (!confirmed) {
        await createEvidence();
        if (!damageClaimRequestIsCurrent(context, current()))
            return false;
    }
    await reload();
    if (!damageClaimRequestIsCurrent(context, current()))
        return false;
    onSuccess();
    return true;
}
