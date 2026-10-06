export function inboxActions(actorID, reservation, now = Date.now()) {
    const isRenter = Boolean(actorID && reservation.renter_id === actorID);
    const isHost = Boolean(actorID && reservation.host_id === actorID);
    const awaitsHostDecision = reservation.state === "pagada";
    const paymentPending = reservation.state === "pendiente_de_pago" && Date.parse(reservation.pay_expires_at) > now;
    const hostDeadlineActive = Boolean(reservation.host_expires_at && Date.parse(reservation.host_expires_at) > now);
    return {
        isRenter,
        isHost,
        canPay: isRenter && paymentPending,
        canCancel: isRenter && paymentPending,
        canDecide: isHost && awaitsHostDecision && hostDeadlineActive,
        awaitsHostDecision: isHost && awaitsHostDecision,
    };
}
