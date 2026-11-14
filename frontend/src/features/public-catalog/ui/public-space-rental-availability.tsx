"use client";

import { useEffect, useId, useRef, useState } from "react";
import Link from "next/link";
import { MonthCalendar, type RentalDemoCalendarDay } from "@/features/rental-demo/ui/month-calendar";

interface Interval {
  start_at: string;
  end_at: string;
}

interface PublicSpaceRentalAvailabilityProps {
  spaceId: string;
  rateUnit: "hora" | "dia" | "mes";
  basePriceCLP: number | string | null;
}

export function PublicSpaceRentalAvailability({
  spaceId,
  rateUnit,
}: PublicSpaceRentalAvailabilityProps) {
  const headingId = useId();
  const today = new Date().toISOString().slice(0, 10);
  const [currentMonth, setCurrentMonth] = useState(today.slice(0, 7));
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const [duration, setDuration] = useState<number>(1);
  const [intervals, setIntervals] = useState<Interval[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedInterval, setSelectedInterval] = useState<Interval | null>(null);
  const generation = useRef(0);

  const durations = rateUnit === "hora" ? [1, 2, 4] : rateUnit === "dia" ? [1, 2, 3] : [1];

  useEffect(() => {
    if (!selectedDate) {
      return;
    }
    const currentOp = ++generation.current;
    const controller = new AbortController();

    fetch(
      `/api/backend/spaces/${encodeURIComponent(spaceId)}/rental-availability?date=${encodeURIComponent(selectedDate)}&duration=${duration}`,
      { method: "GET", headers: { Accept: "application/json" }, signal: controller.signal },
    )
      .then(async (res) => {
        if (generation.current !== currentOp) return;
        if (!res.ok) {
          if (res.status === 404) {
            setError("Publicación no encontrada o no disponible para arriendo.");
          } else {
            setError("No se pudo consultar la disponibilidad en este momento.");
          }
          setIntervals([]);
          return;
        }
        const json = await res.json();
        if (generation.current !== currentOp) return;
        const items: Interval[] = json?.data?.items ?? [];
        setIntervals(items);
        setError(null);
      })
      .catch(() => {
        if (controller.signal.aborted || generation.current !== currentOp) return;
        setError("Error de conexión al consultar disponibilidad.");
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
  if (selectedDate) {
    days.push({
      date: selectedDate,
      status: intervals.length > 0 ? "available" : "unavailable",
      intervals,
    });
  }

  const formatTime = (iso: string) => {
    try {
      const d = new Date(iso);
      return d.toLocaleTimeString("es-CL", { hour: "2-digit", minute: "2-digit", timeZone: "UTC" });
    } catch {
      return iso;
    }
  };

  const returnUrl = `/espacios/${encodeURIComponent(spaceId)}`;
  const loginUrl = `/acceso?returnTo=${encodeURIComponent(returnUrl)}`;

  return (
    <section aria-labelledby={headingId} className="rounded-2xl border border-border p-6 sm:p-8 space-y-6">
      <header className="space-y-1">
        <h2 id={headingId} className="text-xl font-semibold tracking-tight text-foreground">
          Disponibilidad para arriendo
        </h2>
        <p className="text-sm text-muted-foreground leading-relaxed">
          Consulta los intervalos disponibles en tiempo real. La consulta es meramente informativa, no congela ni reserva el horario.
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
                onClick={() => setDuration(d)}
                className={`min-h-11 px-4 py-2 rounded-md border text-sm font-medium transition-colors ${
                  duration === d
                    ? "border-primary bg-primary text-primary-foreground"
                    : "border-border bg-background hover:bg-muted text-foreground"
                }`}
              >
                {d} {rateUnit === "hora" ? (d === 1 ? "hora" : "horas") : (d === 1 ? "día" : "días")}
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
          onSelect={(date) => setSelectedDate(date)}
          onMonthChange={(m) => setCurrentMonth(m)}
          today={today}
          disabled={loading}
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
              No hay intervalos disponibles para la duración seleccionada en esta fecha.
            </p>
          ) : (
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4">
              {intervals.map((it) => {
                const selected = selectedInterval?.start_at === it.start_at;
                return (
                  <button
                    key={it.start_at}
                    type="button"
                    onClick={() => setSelectedInterval(it)}
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
              <p className="text-sm font-medium text-foreground">
                Para continuar, inicia sesión o crea una cuenta
              </p>
              <p className="text-xs text-muted-foreground leading-relaxed">
                Al iniciar sesión volverás directamente a esta publicación con tu intervalo seleccionado para revisar la cotización y verificar tu identidad antes de ver el borrador de contrato.
              </p>
              <Link
                href={loginUrl}
                className="inline-flex min-h-11 items-center justify-center rounded-lg bg-primary px-6 py-2 text-sm font-medium text-primary-foreground shadow hover:bg-primary/90"
              >
                Iniciar sesión o registrarse
              </Link>
            </div>
          ) : null}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground text-center py-2">
          Selecciona una fecha en el calendario para consultar horarios disponibles.
        </p>
      )}
    </section>
  );
}
