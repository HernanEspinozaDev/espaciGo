export interface InboxReservationState {
  host_id: string;
  renter_id: string;
  state: string;
  start_at: string;
  pay_expires_at: string;
  host_expires_at?: string | null;
}

export interface InboxActions {
  isRenter: boolean;
  isHost: boolean;
  canPay: boolean;
  canCancel: boolean;
  canDecide: boolean;
  awaitsHostDecision: boolean;
}

export function inboxActions(actorID: string, reservation: InboxReservationState, now = Date.now()): InboxActions {
  const isRenter = Boolean(actorID && reservation.renter_id === actorID);
  const isHost = Boolean(actorID && reservation.host_id === actorID);
  const awaitsHostDecision = reservation.state === "pagada";
  const paymentPending = reservation.state === "pendiente_de_pago" && Date.parse(reservation.pay_expires_at) > now;
  const preStart = Date.parse(reservation.start_at) > now;
  const refundableState = reservation.state === "pagada" || reservation.state === "aprobada_host";
  const hostDeadlineActive = Boolean(reservation.host_expires_at && Date.parse(reservation.host_expires_at) > now);
  return {
    isRenter,
    isHost,
    canPay: isRenter && paymentPending,
    canCancel: isRenter && (paymentPending || (refundableState && preStart)),
    canDecide: isHost && awaitsHostDecision && hostDeadlineActive,
    awaitsHostDecision: isHost && awaitsHostDecision,
  };
}
