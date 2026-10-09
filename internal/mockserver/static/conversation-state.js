const writableStates = new Set([
    "pendiente_de_pago",
    "pagada",
    "aprobada_host",
    "firma_parcial",
    "lista_para_checkin",
]);
export function canSendConversation(actorID, reservation) {
    return Boolean(reservation && actorID &&
        (reservation.host_id === actorID || reservation.renter_id === actorID) &&
        writableStates.has(reservation.state));
}
