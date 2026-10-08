export function capturePrivacyExportContext(accountID, sessionToken, generation) {
    if (!accountID || !sessionToken)
        return null;
    return { accountID, sessionToken, generation };
}
export function privacyExportSessionMatches(context, accountID, sessionToken, generation) {
    return Boolean(accountID && sessionToken)
        && context.accountID === accountID
        && context.sessionToken === sessionToken
        && context.generation === generation;
}
export async function deliverPrivacyExportIfCurrent(context, load, current, deliver) {
    let data;
    try {
        data = await load();
    }
    catch (error) {
        if (!current(context))
            return false;
        throw error;
    }
    if (!current(context))
        return false;
    deliver(data);
    return true;
}
