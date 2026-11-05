"use client";

import { useEffect, useState } from "react";
import type { AccessToken } from "@/features/auth/application/auth.port";
import { Button } from "@/components/workspace/ui/button";
import { listingReturnsFacade } from "../host-listings-runtime";
import type { ListingReturnsSnapshot } from "../domain/listing-returns";

interface Props {
  readonly spaceId: string;
  readonly token: AccessToken;
  readonly sessionScope: string;
  readonly snapshot: ListingReturnsSnapshot;
  onBusy(busy: boolean): void;
}

export function ListingReturnsPanel({
  spaceId,
  token,
  sessionScope,
  snapshot,
  onBusy,
}: Props) {
  const [confirmed, setConfirmed] = useState<Record<string, boolean>>({});
  useEffect(() => {
    onBusy(snapshot.uncertain || snapshot.pendingReservationId !== null);
  }, [onBusy, snapshot.uncertain, snapshot.pendingReservationId]);
  const refresh = () => listingReturnsFacade.load(token, spaceId, sessionScope);
  const confirm = async (reservationId: string) => {
    if (!confirmed[reservationId]) return;
    const success = await listingReturnsFacade.confirm(
      token,
      spaceId,
      sessionScope,
      reservationId,
    );
    if (success)
      setConfirmed((current) => ({ ...current, [reservationId]: false }));
  };
  return (
    <section
      aria-labelledby="listing-returns-title"
      className="rounded-xl border border-border bg-card p-5 space-y-4"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 id="listing-returns-title" className="text-lg font-semibold">
            Entregas y desocupación
          </h2>
          <p className="text-sm text-muted-foreground">
            Confirma la vacancia después del check-out. Esto no republica el
            espacio ni modifica contratos, historial o finanzas.
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          onClick={refresh}
          disabled={
            snapshot.loading ||
            (snapshot.pendingReservationId !== null && !snapshot.uncertain)
          }
        >
          Actualizar entregas
        </Button>
      </div>
      {snapshot.error && (
        <p role="alert" className="text-sm text-red-800">
          {snapshot.error}
        </p>
      )}
      {snapshot.loading && (
        <p role="status" className="text-sm text-muted-foreground">
          Cargando entregas…
        </p>
      )}
      {!snapshot.loading && !snapshot.error && snapshot.items.length === 0 && (
        <p className="text-sm text-muted-foreground">
          No hay entregas finalizadas con check-out registrado.
        </p>
      )}
      <ul className="divide-y divide-border">
        {snapshot.items.map((item) => (
          <li
            key={item.reservation_id}
            className="py-4 first:pt-0 last:pb-0 space-y-3"
          >
            <p className="text-sm">
              <span className="font-medium">Arriendo de ensayo</span> ·{" "}
              {new Date(item.start_at).toLocaleDateString("es-CL")} –{" "}
              {new Date(item.end_at).toLocaleDateString("es-CL")}
            </p>
            <p className="text-xs text-muted-foreground">
              Referencia de arriendo:{" "}
              <code className="break-all">{item.reservation_id}</code>
            </p>
            <p className="text-sm">
              {item.vacancy_confirmed
                ? "Desocupación confirmada"
                : "Check-out registrado"}
            </p>
            {!item.vacancy_confirmed && (
              <div className="flex flex-wrap items-center gap-3">
                <label className="flex items-start gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={Boolean(confirmed[item.reservation_id])}
                    onChange={(event) =>
                      setConfirmed((current) => ({
                        ...current,
                        [item.reservation_id]: event.target.checked,
                      }))
                    }
                    disabled={
                      snapshot.uncertain ||
                      snapshot.pendingReservationId !== null
                    }
                  />
                  <span>
                    Confirmo explícitamente que el espacio está desocupado
                  </span>
                </label>
                <Button
                  type="button"
                  onClick={() => void confirm(item.reservation_id)}
                  disabled={
                    !confirmed[item.reservation_id] ||
                    snapshot.uncertain ||
                    snapshot.pendingReservationId !== null
                  }
                >
                  {snapshot.pendingReservationId === item.reservation_id
                    ? "Confirmando…"
                    : "Confirmar desocupación"}
                </Button>
              </div>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
