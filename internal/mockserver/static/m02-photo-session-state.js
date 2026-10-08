export function captureM02PhotoSession(token, account, generation) {
    return token && account ? { token, account, generation } : null;
}
export function m02PhotoSessionMatches(captured, token, account, generation) {
    return Boolean(token && account)
        && captured.token === token
        && captured.account === account
        && captured.generation === generation;
}
/** Delivers an asynchronous photo result only while the exact requesting session is current. */
export async function deliverM02PhotoIfCurrent(captured, load, current, deliver) {
    let result;
    try {
        result = await load();
    }
    catch (error) {
        if (!current(captured))
            return false;
        throw error;
    }
    return applyM02PhotoIfCurrent(captured, current, () => deliver(result));
}
export function applyM02PhotoIfCurrent(captured, current, apply) {
    if (!current(captured))
        return false;
    apply();
    return true;
}
