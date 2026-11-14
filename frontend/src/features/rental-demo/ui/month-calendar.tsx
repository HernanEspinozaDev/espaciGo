"use client";

import { useId } from "react";
import { localDateLabel, monthDays, shiftMonth } from "../domain/calendar";

export interface RentalDemoCalendarDay {
  date: string;
  status: "available" | "partial" | "unavailable" | "outside_horizon";
  intervals: readonly { start_at: string; end_at: string }[];
}

export interface MonthCalendarProps {
  month: string;
  days: readonly RentalDemoCalendarDay[];
  selectedDate: string | null;
  onSelect: (date: string) => void;
  onMonthChange: (month: string) => void;
  today: string;
  disabled?: boolean;
}

const WEEKDAYS = ["Lunes", "Martes", "Miércoles", "Jueves", "Viernes", "Sábado", "Domingo"];
const STATUS_TEXT: Record<RentalDemoCalendarDay["status"], string> = {
  available: "Disponible",
  partial: "Parcialmente disponible",
  unavailable: "No disponible",
  outside_horizon: "Fuera del horizonte de consulta",
};

function monthLabel(month: string): string {
  const date = new Date(`${month}-01T00:00:00Z`);
  return new Intl.DateTimeFormat("es", { month: "long", year: "numeric", timeZone: "UTC" }).format(date);
}

export function MonthCalendar({ month, days, selectedDate, onSelect, onMonthChange, today, disabled = false }: MonthCalendarProps) {
  const titleId = useId();
  const byDate = new Map(days.map((day) => [day.date, day]));
  const grid = monthDays(month);
  const move = (delta: number) => {
    const next = shiftMonth(month, delta);
    if (next) onMonthChange(next);
  };

  return (
    <section aria-labelledby={titleId} className="w-full min-w-0 space-y-4">
      <header className="flex min-w-0 items-center justify-between gap-2">
        <button type="button" aria-label="Mes anterior" disabled={disabled || !shiftMonth(month, -1)} onClick={() => move(-1)} className="min-h-11 min-w-11 shrink-0 rounded-md border px-3">‹</button>
        <h2 id={titleId} className="min-w-0 truncate text-center text-lg font-semibold capitalize">{monthLabel(month)}</h2>
        <button type="button" aria-label="Mes siguiente" disabled={disabled || !shiftMonth(month, 1)} onClick={() => move(1)} className="min-h-11 min-w-11 shrink-0 rounded-md border px-3">›</button>
      </header>
      <div className="flex justify-center">
        <button type="button" disabled={disabled} onClick={() => onMonthChange(today.slice(0, 7))} className="min-h-11 rounded-md border px-4">Hoy</button>
      </div>
      <div role="grid" aria-label={`Calendario de ${monthLabel(month)}`} className="grid grid-cols-7 gap-1">
        {WEEKDAYS.map((weekday) => <div role="columnheader" key={weekday} className="min-w-0 py-2 text-center text-xs font-medium sm:text-sm">{weekday}</div>)}
        {grid.map(({ date, inMonth }) => {
          const day = byDate.get(date);
          const selected = selectedDate === date;
          const unavailable = !day || day.status === "outside_horizon";
          const label = day ? STATUS_TEXT[day.status] : "Sin información";
          return <div role="gridcell" key={date} className="min-w-0">
            {inMonth ? <button type="button" aria-label={`${localDateLabel(date)}. ${label}${selected ? ". Seleccionado" : ""}`} aria-pressed={selected} disabled={disabled || unavailable} onClick={() => onSelect(date)} className={`flex min-h-11 w-full min-w-0 flex-col items-center justify-center rounded-md border px-0.5 text-xs sm:text-sm ${selected ? "border-primary bg-primary text-primary-foreground" : day?.status === "available" ? "border-emerald-600 bg-emerald-50" : day?.status === "partial" ? "border-amber-600 bg-amber-50" : "border-muted bg-muted/30"}`}>
              <span>{Number(date.slice(-2))}</span><span className="max-w-full truncate text-[9px] leading-tight sm:text-[10px]">{label}</span>
            </button> : <span aria-hidden="true" className="block min-h-11" />}
          </div>;
        })}
      </div>
      <ul aria-label="Leyenda de disponibilidad" className="flex flex-wrap gap-x-4 gap-y-2 text-xs sm:text-sm">
        {Object.entries(STATUS_TEXT).filter(([status]) => status !== "outside_horizon").map(([status, label]) => <li key={status}>{label}</li>)}
        <li>Seleccionado</li><li>Sin información</li>
      </ul>
    </section>
  );
}
