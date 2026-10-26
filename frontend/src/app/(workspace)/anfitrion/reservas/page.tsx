"use client";

import { ReservationsList } from "@/features/bookings/ui/reservations-list";

export default function HostReservasPage() {
  return (
    <ReservationsList
      mode="host"
      title="Gestión de Reservas de Anfitrión"
      description="Supervisa solicitudes, aprueba arriendos y administra las operaciones de entrega y recepción."
    />
  );
}
