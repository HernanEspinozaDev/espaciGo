"use client";

import { useEffect, useId, useRef, useState } from "react";
import Link from "next/link";
import {
  MonthCalendar,
  type RentalDemoCalendarDay,
} from "@/features/rental-demo/ui/month-calendar";
import { addCivilDays } from "../domain/local-date";
import { publicAvailabilityNotice } from "../domain/availability-notice";
import { publicAvailabilityExceptionMessage } from "../domain/availability-error";
import { publicSpaceLoginHref } from "../domain/availability-return";
import type { RentalReturnSelection } from "../domain/availability-return";
import type { PublicRentalAvailability } from "../domain/rental-availability";
import { publicRentalAvailability } from "../infrastructure/rental-availability-runtime";
import { publicRentalQuote } from "../infrastructure/rental-quote-runtime";
import { formatRentalCLP } from "../domain/contract-preview-format";
import type { RentalQuote } from "../domain/rental-quote";
import { useAuth } from "@/features/auth/ui/use-auth";
import { PublicRentalContractPreview } from "./public-rental-contract-preview";

type Interval = PublicRentalAvailability["items"][number];

interface PublicSpaceRentalAvailabilityProps {
  spaceId: string;
  rateUnit: "hora" | "dia" | "mes";
  basePriceCLP: number | string | null;
  today: string;
  initialSelection?: RentalReturnSelection;
}

export function PublicSpaceRentalAvailability({
  spaceId,
  rateUnit,
  today,
  initialSelection,
}: PublicSpaceRentalAvailabilityProps) {
  const headingId = useId();
  const [currentMonth, setCurrentMonth] = useState(
    initialSelection?.date.slice(0, 7) ?? today.slice(0, 7),
  );
  const [selectedDate, setSelectedDate] = useState<string | null>(
    initialSelection?.date ?? null,
  );
  const [duration, setDuration] = useState<number>(
    initialSelection?.duration ?? 1,
  );
  const [intervals, setIntervals] = useState<Interval[]>([]);
  const [loading, setLoading] = useState(Boolean(initialSelection));
  const [error, setError] = useState<string | null>(null);
  const [selectedInterval, setSelectedInterval] = useState<Interval | null>(
    null,
  );
  const { snapshot: authSnapshot, checkSession } = useAuth();
  const [quote, setQuote] = useState<RentalQuote | null>(null);
  const [quoteScope, setQuoteScope] = useState<string | null>(null);
  const [quotePending, setQuotePending] = useState(false);
  const [quoteError, setQuoteError] = useState<string | null>(null);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [blockedAttempt, setBlockedAttempt] = useState<string | null>(null);
  const quoteController = useRef<AbortController | null>(null);
  const pendingRentalSelection = useRef(initialSelection);
  const generation = useRef(0);
  const selectionKey = `${selectedDate ?? ""}|${duration}|${selectedInterval?.start_at ?? ""}|${selectedInterval?.end_at ?? ""}`;
  const sessionKey =
    authSnapshot.status === "authenticated"
      ? JSON.stringify([authSnapshot.accountId, authSnapshot.expiresAt])
      : authSnapshot.status;
  const visibleQuote = quoteScope === sessionKey ? quote : null;
  const previousSession = useRef(sessionKey);

  useEffect(() => {
    if (previousSession.current !== sessionKey) {
      quoteController.current?.abort();
      quoteController.current = null;
      setQuote(null);
      setQuoteScope(sessionKey);
      setQuotePending(false);
      setQuoteError(null);
      setPreviewOpen(false);
      setBlockedAttempt(null);
      previousSession.current = sessionKey;
    }
  }, [sessionKey]);

  useEffect(() => () => quoteController.current?.abort(), []);

  const clearQuoteForSelection = () => {
    quoteController.current?.abort();
    quoteController.current = null;
    setQuote(null);
    setQuoteScope(sessionKey);
    setQuoteError(null);
    setPreviewOpen(false);
    setBlockedAttempt(null);
  };

  const requestQuote = async () => {
    if (
      !selectedInterval ||
      authSnapshot.status !== "authenticated" ||
      quotePending ||
      quoteController.current !== null ||
      blockedAttempt === selectionKey
    )
      return;
    const controller = new AbortController();
    quoteController.current = controller;
    setQuotePending(true);
    setQuoteError(null);
    const requestKey = selectionKey;
    const requestSessionKey = sessionKey;
    setQuoteScope(requestSessionKey);
    try {
      const result = await publicRentalQuote.createQuote(
        {
          space_id: spaceId,
          start_at: selectedInterval.start_at,
          end_at: selectedInterval.end_at,
        },
        controller.signal,
      );
      if (controller.signal.aborted) return;
      if (result.ok) setQuote(result.value);
      else {
        const messages = {
          unauthenticated:
            "Tu sesión ya no está disponible. Inicia sesión nuevamente.",
          forbidden:
            "Tu cuenta no tiene acceso para solicitar esta cotización.",
          not_found: "La publicación o el intervalo ya no está disponible.",
          conflict: "La disponibilidad cambió. Selecciona otro intervalo.",
          invalid:
            "La selección no es válida. Actualiza fecha, duración o intervalo.",
          unavailable:
            "El servicio no está disponible temporalmente. Cambia la selección antes de intentar otra vez.",
          other:
            "No se pudo confirmar el resultado de la solicitud. Cambia fecha, duración o intervalo antes de volver a intentarlo.",
        } as const;
        setQuoteError(messages[result.error.kind]);
        setBlockedAttempt(requestKey);
        if (result.error.kind === "unauthenticated")
          void checkSession().catch(() => false);
      }
    } catch {
      if (!controller.signal.aborted) {
        setQuoteError(
          "No se pudo confirmar el resultado de la solicitud. Cambia fecha, duración o intervalo antes de volver a intentarlo.",
        );
        setBlockedAttempt(requestKey);
      }
    } finally {
      if (!controller.signal.aborted) setQuotePending(false);
      if (quoteController.current === controller)
        quoteController.current = null;
    }
  };

  const durations =
    rateUnit === "hora" ? [1, 2, 4] : rateUnit === "dia" ? [1, 2, 3] : [1];

  const beginAvailabilityQuery = () => {
    setLoading(true);
    setError(null);
    setIntervals([]);
    setSelectedInterval(null);
  };

  useEffect(() => {
    if (!selectedDate) {
      return;
    }
    const currentOp = ++generation.current;
    const controller = new AbortController();

    publicRentalAvailability
      .getAvailability(spaceId, selectedDate, duration, controller.signal)
      .then((result) => {
        if (generation.current !== currentOp) return;
        if (!result.ok) {
          setError(
            result.error.kind === "not-found"
              ? "Publicación no encontrada o no disponible para arriendo."
              : result.error.kind === "invalid"
                ? "La fecha o duración seleccionada no es válida."
                : "No se pudo consultar la disponibilidad en este momento.",
          );
          setIntervals([]);
          return;
        }
        setIntervals(result.value.items);
        const pending = pendingRentalSelection.current;
        if (pending?.date === selectedDate && pending.duration === duration) {
          const matchingInterval = result.value.items.find(
            (item) =>
              item.start_at === pending.startAt &&
              item.end_at === pending.endAt,
          );
          if (matchingInterval) setSelectedInterval(matchingInterval);
          pendingRentalSelection.current = undefined;
        }
      })
      .catch(() => {
        if (controller.signal.aborted || generation.current !== currentOp)
          return;
        setError(publicAvailabilityExceptionMessage());
        setIntervals([]);
      })
      .finally(() => {
        if (generation.current === currentOp) {
          setLoading(false);
        }
      });

    return () => {
      controller.abort();
    };
  }, [spaceId, selectedDate, duration]);

  // Days for calendar projection
  const days: RentalDemoCalendarDay[] = [];
  // If a date is selected and has intervals, mark it as available
  if (selectedDate && !loading && !error) {
    days.push({
      date: selectedDate,
      status: intervals.length > 0 ? "available" : "unavailable",
      intervals,
    });
  }

  const formatTime = (iso: string) => {
    try {
      const d = new Date(iso);
      return d.toLocaleTimeString("es-CL", {
        hour: "2-digit",
        minute: "2-digit",
        timeZone: "UTC",
      });
    } catch {
      return iso;
    }
  };

  const loginUrl = publicSpaceLoginHref(
    spaceId,
    selectedDate && selectedInterval
      ? {
          date: selectedDate,
          duration,
          startAt: selectedInterval.start_at,
          endAt: selectedInterval.end_at,
        }
      : undefined,
  );

  return (
    <section
      aria-labelledby={headingId}
      className="rounded-2xl border border-border p-6 sm:p-8 space-y-6"
    >
      <header className="space-y-1">
        <h2
          id={headingId}
          className="text-xl font-semibold tracking-tight text-foreground"
        >
          Disponibilidad para arriendo
        </h2>
        <p className="text-sm text-muted-foreground leading-relaxed">
          {publicAvailabilityNotice}
        </p>
      </header>

      {durations.length > 1 ? (
        <fieldset className="space-y-2">
          <legend className="text-sm font-medium text-foreground">
            Duración ({rateUnit === "hora" ? "horas" : "días"}):
          </legend>
          <div className="flex flex-wrap gap-2">
            {durations.map((d) => (
              <button
                key={d}
                type="button"
                disabled={quotePending}
                onClick={() => {
                  if (duration === d || quotePending) return;
                  pendingRentalSelection.current = undefined;
                  clearQuoteForSelection();
                  beginAvailabilityQuery();
                  setDuration(d);
                }}
                className={`min-h-11 px-4 py-2 rounded-md border text-sm font-medium transition-colors ${
                  duration === d
                    ? "border-primary bg-primary text-primary-foreground"
                    : "border-border bg-background hover:bg-muted text-foreground"
                }`}
              >
                {d}{" "}
                {rateUnit === "hora"
                  ? d === 1
                    ? "hora"
                    : "horas"
                  : d === 1
                    ? "día"
                    : "días"}
              </button>
            ))}
          </div>
        </fieldset>
      ) : null}

      <div className="pt-2">
        <MonthCalendar
          month={currentMonth}
          days={days}
          selectedDate={selectedDate}
          onSelect={(date) => {
            if (selectedDate === date || quotePending) return;
            pendingRentalSelection.current = undefined;
            clearQuoteForSelection();
            beginAvailabilityQuery();
            setSelectedDate(date);
          }}
          onMonthChange={(m) => setCurrentMonth(m)}
          today={today}
          disabled={loading || quotePending}
          allowUnqueriedDates
          minDate={today}
          maxDate={addCivilDays(today, 90)}
        />
      </div>

      {selectedDate ? (
        <div className="space-y-4 rounded-xl border border-border bg-muted/20 p-4 sm:p-6">
          <h3 className="text-base font-medium text-foreground">
            Horarios para el {selectedDate}:
          </h3>

          {loading ? (
            <p className="text-sm text-muted-foreground animate-pulse">
              Consultando disponibilidad en servidor...
            </p>
          ) : error ? (
            <p className="text-sm text-destructive">{error}</p>
          ) : intervals.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No hay intervalos disponibles para la duración seleccionada en
              esta fecha.
            </p>
          ) : (
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4">
              {intervals.map((it) => {
                const selected = selectedInterval?.start_at === it.start_at;
                return (
                  <button
                    key={it.start_at}
                    type="button"
                    disabled={quotePending}
                    onClick={() => {
                      if (quotePending) return;
                      if (selectedInterval?.start_at !== it.start_at)
                        clearQuoteForSelection();
                      setSelectedInterval(it);
                    }}
                    className={`min-h-11 rounded-lg border p-2 text-center text-xs sm:text-sm font-medium transition-colors ${
                      selected
                        ? "border-primary bg-primary text-primary-foreground"
                        : "border-border bg-background hover:border-primary/50 text-foreground"
                    }`}
                  >
                    {formatTime(it.start_at)} – {formatTime(it.end_at)}
                  </button>
                );
              })}
            </div>
          )}

          {selectedInterval ? (
            <div className="mt-6 space-y-3 rounded-lg border border-primary/20 bg-primary/5 p-4">
              {authSnapshot.status === "authenticated" ? (
                <>
                  {!visibleQuote && (
                    <button
                      type="button"
                      onClick={requestQuote}
                      disabled={quotePending || blockedAttempt === selectionKey}
                      className="inline-flex min-h-11 items-center justify-center rounded-lg bg-primary px-6 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
                    >
                      {quotePending
                        ? "Solicitando cotización..."
                        : "Solicitar cotización"}
                    </button>
                  )}
                  {quotePending && <p role="status">Guardando cotización...</p>}
                  {quoteError && (
                    <p role="alert" className="text-sm text-destructive">
                      {quoteError}
                    </p>
                  )}
                  {blockedAttempt === selectionKey && (
                    <p className="text-xs text-muted-foreground">
                      Resultado no confirmado; cambia fecha, duración o
                      intervalo antes de otra solicitud.
                    </p>
                  )}
                  {visibleQuote && (
                    <div className="space-y-2" role="status">
                      <p className="font-medium">Cotización creada</p>
                      <p>{visibleQuote.space_title}</p>
                      <p>{formatRentalCLP(visibleQuote.subtotal_clp)}</p>
                      <p>
                        Válida hasta{" "}
                        {new Intl.DateTimeFormat("es-CL", {
                          dateStyle: "medium",
                          timeStyle: "short",
                          timeZone: visibleQuote.time_zone,
                        }).format(new Date(visibleQuote.expires_at))}{" "}
                        ({visibleQuote.time_zone}).
                      </p>
                      <p className="text-xs text-muted-foreground">
                        Cotización no vinculante; no constituye reserva, pago ni
                        firma.
                      </p>
                      <button
                        type="button"
                        onClick={() => setPreviewOpen(true)}
                        className="inline-flex min-h-11 items-center justify-center rounded-lg border px-5 py-2"
                      >
                        Vista previa del contrato
                      </button>
                    </div>
                  )}
                </>
              ) : authSnapshot.status === "checking" ? (
                <p role="status" className="text-sm text-muted-foreground">
                  Verificando sesión...
                </p>
              ) : (
                <>
                  <p className="text-sm font-medium text-foreground">
                    Para continuar, inicia sesión o crea una cuenta
                  </p>
                  <p className="text-xs text-muted-foreground leading-relaxed">
                    Al iniciar sesión volverás a esta publicación. La
                    disponibilidad puede cambiar después de esta consulta; el
                    horario no queda congelado ni reservado.
                  </p>
                  <Link
                    href={loginUrl}
                    className="inline-flex min-h-11 items-center justify-center rounded-lg bg-primary px-6 py-2 text-sm font-medium text-primary-foreground shadow hover:bg-primary/90"
                  >
                    Iniciar sesión o registrarse
                  </Link>
                </>
              )}
            </div>
          ) : null}
          {previewOpen && visibleQuote ? (
            <PublicRentalContractPreview
              quoteId={visibleQuote.id}
              onClose={() => setPreviewOpen(false)}
            />
          ) : null}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground text-center py-2">
          Selecciona una fecha en el calendario para consultar horarios
          disponibles.
        </p>
      )}
    </section>
  );
}
