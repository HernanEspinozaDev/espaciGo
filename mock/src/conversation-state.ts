export interface ConversationReservation {
  host_id: string;
  renter_id: string;
  state: string;
}

const writableStates = new Set([
  "pendiente_de_pago",
  "pagada",
  "aprobada_host",
  "firma_parcial",
  "lista_para_checkin",
]);

export function canSendConversation(actorID: string, reservation: ConversationReservation | null): boolean {
  return Boolean(
    reservation && actorID &&
    (reservation.host_id === actorID || reservation.renter_id === actorID) &&
    writableStates.has(reservation.state)
  );
}
