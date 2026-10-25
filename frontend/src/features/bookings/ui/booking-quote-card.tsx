"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/features/auth/ui/use-auth";
import { useBookings } from "./use-bookings";
import { formatBookingClp } from "./format";
import { generateIdempotencyKey } from "../domain/booking-rules";
import type { RateUnit } from "../domain/types";
import { Button } from "@/components/workspace/ui/button";
import {
  Calendar,
  Clock,
  ShieldCheck,
  Info,
  AlertCircle,
  CheckCircle2,
  Loader2,
} from "lucide-react";

interface BookingQuoteCardProps {
  spaceId?: string;
  spaceTitle?: string;
  defaultRateUnit?: RateUnit;
  defaultUnitPriceClp?: number;
  onReservationCreated?: (reservationId: string) => void;
}

export function BookingQuoteCard({
  spaceId: initialSpaceId = "",
  spaceTitle,
  defaultRateUnit = "hora",
  defaultUnitPriceClp,
  onReservationCreated,
}: BookingQuoteCardProps) {
  const router = useRouter();
  const { accessToken } = useAuth();
  const {
    quote,
    actionLoading,
    error,
    clearError,
    clearQuote,
    requestQuote,
    createReservation,
  } = useBookings();

  const [spaceId, setSpaceId] = useState(initialSpaceId);
  const [rateUnit, setRateUnit] = useState<RateUnit>(defaultRateUnit);
  const [durationUnits, setDurationUnits] = useState<number>(1);
  const [startDate, setStartDate] = useState<string>(() => {
    const d = new Date();
    d.setDate(d.getDate() + 1);
    d.setHours(9, 0, 0, 0);
    return d.toISOString().slice(0, 16);
  });
  const [localError, setLocalError] = useState<string | null>(null);
  const [createdReservationId, setCreatedReservationId] = useState<
    string | null
  >(null);
  const [, startTransition] = useTransition();

  // Calculate end_at based on start_at, rateUnit, and durationUnits
  const calculateEndAt = (
    startIso: string,
    unit: RateUnit,
    units: number,
  ): string => {
    const date = new Date(startIso);
    if (isNaN(date.getTime())) return "";
    if (unit === "hora") {
      date.setHours(date.getHours() + units);
    } else if (unit === "dia") {
      date.setDate(date.getDate() + units);
    } else if (unit === "mes") {
      date.setMonth(date.getMonth() + units);
    }
    return date.toISOString();
  };

  const handleRequestQuote = async (e: React.FormEvent) => {
    e.preventDefault();
    setLocalError(null);
    clearError();

    if (!accessToken) {
      setLocalError("Debes iniciar sesión para cotizar una reserva.");
      return;
    }
    const targetSpaceId = spaceId.trim();
    if (!targetSpaceId) {
      setLocalError("Identificador de espacio requerido.");
      return;
    }

    const startAtIso = new Date(startDate).toISOString();
    const endAtIso = calculateEndAt(startDate, rateUnit, durationUnits);

    try {
      await requestQuote(accessToken, {
        space_id: targetSpaceId,
        start_at: startAtIso,
        end_at: endAtIso,
      });
    } catch (err) {
      setLocalError(
        err instanceof Error ? err.message : "Error al calcular cotización.",
      );
    }
  };

  const handleConfirmReservation = async () => {
    if (!quote || !accessToken) return;
    setLocalError(null);
    clearError();

    try {
      const idempotencyKey = generateIdempotencyKey();
      const reservation = await createReservation(
        accessToken,
        quote.id,
        idempotencyKey,
      );
      setCreatedReservationId(reservation.id);
      clearQuote();
      if (onReservationCreated) {
        onReservationCreated(reservation.id);
      } else {
        startTransition(() => {
          router.push(`/reservas/${encodeURIComponent(reservation.id)}`);
        });
      }
    } catch (err) {
      setLocalError(
        err instanceof Error ? err.message : "Error al confirmar la reserva.",
      );
    }
  };

  return (
    <div className="rounded-2xl border border-border bg-card p-6 shadow-sm sm:p-8">
      <div className="border-b border-border pb-4">
        <div className="flex items-center justify-between gap-2">
          <h1 className="text-xl font-semibold tracking-tight text-card-foreground">
            {spaceTitle
              ? `Cotizar: ${spaceTitle}`
              : "Cotización Oficial de Reserva"}
          </h1>
          <span
            className="inline-flex items-center gap-1 rounded-full bg-amber-500/10 px-2.5 py-0.5 text-xs font-medium text-amber-800 dark:text-amber-300"
            title="Entorno de ensayo seguro. No se realizan cobros reales."
          >
            Ensayo
          </span>
        </div>
        <p className="mt-1 text-sm text-muted-foreground">
          Precios transparentes en pesos chilenos (CLP) con desglose vinculante.
        </p>
      </div>

      {(localError || error) && (
        <div
          role="alert"
          className="mt-4 flex items-start gap-3 rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-sm text-red-800 dark:text-red-300"
        >
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <div>
            <p className="font-medium">Atención</p>
            <p>{localError || error}</p>
          </div>
        </div>
      )}

      {createdReservationId && (
        <div
          role="status"
          className="mt-4 flex items-start gap-3 rounded-lg border border-emerald-500/20 bg-emerald-500/10 p-4 text-sm text-emerald-700 dark:text-emerald-400"
        >
          <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0" />
          <div>
            <p className="font-medium">¡Reserva creada exitosamente!</p>
            <p className="mt-0.5">
              Identificador de reserva: {createdReservationId}
            </p>
          </div>
        </div>
      )}

      <form onSubmit={handleRequestQuote} className="mt-6 space-y-5">
        {!initialSpaceId && (
          <div>
            <label
              htmlFor="quote-space-id"
              className="block text-sm font-medium text-foreground"
            >
              ID del Espacio
            </label>
            <input
              id="quote-space-id"
              type="text"
              value={spaceId}
              onChange={(e) => setSpaceId(e.target.value)}
              placeholder="uuid-del-espacio"
              required
              className="mt-1.5 w-full rounded-lg border border-input bg-background px-3.5 py-2 text-sm shadow-sm focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
        )}

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label
              htmlFor="quote-rate-unit"
              className="block text-sm font-medium text-foreground"
            >
              Modalidad de Arriendo
            </label>
            <select
              id="quote-rate-unit"
              value={rateUnit}
              onChange={(e) => {
                const unit = e.target.value as RateUnit;
                setRateUnit(unit);
                setDurationUnits(1);
              }}
              className="mt-1.5 w-full rounded-lg border border-input bg-background px-3.5 py-2 text-sm shadow-sm focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
            >
              <option value="hora">Por Hora</option>
              <option value="dia">Por Día</option>
              <option value="mes">Por Mes</option>
            </select>
          </div>

          <div>
            <label
              htmlFor="quote-units"
              className="block text-sm font-medium text-foreground"
            >
              Duración (
              {rateUnit === "hora"
                ? "horas"
                : rateUnit === "dia"
                  ? "días"
                  : "meses"}
              )
            </label>
            <input
              id="quote-units"
              type="number"
              min="1"
              max={rateUnit === "hora" ? 24 : rateUnit === "dia" ? 30 : 12}
              value={durationUnits}
              onChange={(e) =>
                setDurationUnits(Math.max(1, parseInt(e.target.value, 10) || 1))
              }
              required
              className="mt-1.5 w-full rounded-lg border border-input bg-background px-3.5 py-2 text-sm shadow-sm focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
        </div>

        <div>
          <label
            htmlFor="quote-start-at"
            className="block text-sm font-medium text-foreground"
          >
            Fecha y Hora de Inicio
          </label>
          <div className="relative mt-1.5">
            <input
              id="quote-start-at"
              type="datetime-local"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              required
              className="w-full rounded-lg border border-input bg-background px-3.5 py-2 text-sm shadow-sm focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
        </div>

        {defaultUnitPriceClp ? (
          <p className="text-xs text-muted-foreground">
            Tarifa referencial: {formatBookingClp(defaultUnitPriceClp)} CLP /{" "}
            {rateUnit}
          </p>
        ) : null}

        <Button
          type="submit"
          variant="secondary"
          className="w-full"
          disabled={actionLoading || !accessToken}
        >
          {actionLoading ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Calculando cotización...
            </>
          ) : (
            "Obtener Cotización Vinculante"
          )}
        </Button>
      </form>

      {/* Quote Breakdown */}
      {quote && (
        <div className="mt-8 space-y-4 rounded-xl border border-primary/20 bg-primary/5 p-5">
          <div className="flex items-center justify-between">
            <h2 className="font-medium text-foreground flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4 text-primary" />
              Cotización Calculada
            </h2>
            <span className="text-xs text-muted-foreground">
              Válida hasta{" "}
              {new Date(quote.expires_at).toLocaleTimeString([], {
                hour: "2-digit",
                minute: "2-digit",
              })}
            </span>
          </div>

          <div className="divide-y divide-border/60 text-sm">
            <div className="flex justify-between py-2">
              <span className="text-muted-foreground">
                Tarifa base ({quote.units} {quote.rate_unit}
                {quote.units > 1 ? "s" : ""} a{" "}
                {formatBookingClp(quote.unit_price_clp)} CLP)
              </span>
              <span className="font-medium text-foreground">
                {formatBookingClp(quote.subtotal_clp)} CLP
              </span>
            </div>

            {quote.guarantee_expected_clp > 0 && (
              <div className="flex justify-between py-2">
                <span className="text-muted-foreground flex items-center gap-1">
                  <ShieldCheck className="h-3.5 w-3.5 text-muted-foreground" />
                  Garantía de respaldo (reembolsable)
                </span>
                <span className="font-medium text-foreground">
                  {formatBookingClp(quote.guarantee_expected_clp)} CLP
                </span>
              </div>
            )}

            <div className="flex justify-between py-3 text-base font-semibold text-foreground">
              <span>Total Arriendo</span>
              <span>{formatBookingClp(quote.subtotal_clp)} CLP</span>
            </div>
          </div>

          <div className="rounded-lg bg-background/80 p-3 text-xs text-muted-foreground space-y-1.5">
            <div className="flex items-center gap-1.5">
              <Calendar className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              <span>
                Inicio: {new Date(quote.start_at).toLocaleString("es-CL")}
              </span>
            </div>
            <div className="flex items-center gap-1.5">
              <Clock className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              <span>
                Término: {new Date(quote.end_at).toLocaleString("es-CL")}
              </span>
            </div>
            <div className="flex items-center gap-1.5">
              <Info className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              <span>
                Política de cancelación: {quote.cancellation_policy_version}
              </span>
            </div>
          </div>

          <div className="pt-2">
            <Button
              type="button"
              onClick={handleConfirmReservation}
              disabled={actionLoading}
              className="w-full text-base py-5"
            >
              {actionLoading ? (
                <>
                  <Loader2 className="mr-2 h-5 w-5 animate-spin" />
                  Reservando...
                </>
              ) : (
                `Confirmar y Reservar (${formatBookingClp(quote.subtotal_clp)} CLP)`
              )}
            </Button>
            <p className="mt-2 text-center text-xs text-muted-foreground">
              Al reservar, se enviará una solicitud formal con clave de
              idempotencia única.
            </p>
          </div>
        </div>
      )}
    </div>
  );
}
