export function contractResponseIsCurrent(start, current) {
    return start.revision === current.revision && start.reservationID === current.reservationID && start.accountID === current.accountID && start.token === current.token && start.generation === current.generation;
}
export function contractActions(state) {
    const own = state.signatures.find(signature => signature.signer_id === state.accountID);
    const beforeStart = state.now < state.reservationStart;
    const cancellableReservation = ["aprobada_host", "firma_parcial", "lista_para_checkin"].includes(state.reservationState);
    const actionable = cancellableReservation && beforeStart && Boolean(own && own.state === "pendiente") && !["firmado", "anulado"].includes(state.contractState ?? "");
    return {
        canOpen: ["aprobada_host", "firma_parcial", "lista_para_checkin"].includes(state.reservationState),
        canSign: actionable,
        canReject: actionable,
        canDownload: state.contractState === "firmado",
    };
}
