"use client";

import { useId, useMemo, useState } from "react";
import type { ScheduleSlot } from "../domain/types";
import {
  applyWeeklyPreset,
  toggleWeeklyHour,
  weeklyHourCoverage,
  type WeeklyPreset,
} from "../domain/weekly-schedule-slots";

export interface WeeklyScheduleSlotsProps {
  slots: readonly ScheduleSlot[];
  onChange?: (slots: ScheduleSlot[]) => void;
  disabled?: boolean;
  readOnly?: boolean;
  invalid?: boolean;
  errorId?: string;
  timeZone: string;
}
const DAYS = [
  { id: 1, label: "Lunes" },
  { id: 2, label: "Martes" },
  { id: 3, label: "Miércoles" },
  { id: 4, label: "Jueves" },
  { id: 5, label: "Viernes" },
  { id: 6, label: "Sábado" },
  { id: 7, label: "Domingo" },
];
const PRESETS: Array<{ id: WeeklyPreset; label: string }> = [
  { id: "morning", label: "Mañana 09:00–13:00" },
  { id: "afternoon", label: "Tarde 14:00–18:00" },
  { id: "full-day", label: "Jornada 09:00–18:00" },
  { id: "24h", label: "24 horas 00:00–24:00" },
];

export function WeeklyScheduleSlots({
  slots,
  onChange,
  disabled = false,
  readOnly = false,
  invalid = false,
  errorId,
  timeZone,
}: WeeklyScheduleSlotsProps) {
  const titleId = useId();
  const [day, setDay] = useState(1);
  const [error, setError] = useState("");
  const locked = disabled || readOnly || !onChange;
  const daySlots = useMemo(
    () => slots.filter((slot) => slot.day_of_week === day),
    [slots, day],
  );
  const selected = (hour: number) =>
    daySlots.some((slot) => {
      const [sh, sm] = slot.start_time.split(":").map(Number);
      const [eh, em] = slot.end_time.split(":").map(Number);
      return hour * 60 < eh * 60 + em && (hour + 1) * 60 > sh * 60 + sm;
    });
  const commit = (result: { slots: ScheduleSlot[]; error: string | null }) => {
    setError(result.error ?? "");
    if (!result.error) onChange?.(result.slots);
  };
  return (
    <section aria-labelledby={titleId} className="space-y-4">
      <div>
        <h3 id={titleId} className="text-lg font-semibold">
          Horario semanal habitual
        </h3>
        <p className="text-sm text-muted-foreground">
          Este horario es orientativo y no indica disponibilidad para fechas
          específicas. Zona horaria: {timeZone}
        </p>
      </div>
      <div
        role="group"
        aria-label="Día de la semana"
        className="flex flex-wrap gap-2"
      >
        {DAYS.map((item) => (
          <button
            key={item.id}
            type="button"
            aria-pressed={day === item.id}
            disabled={disabled}
            onClick={() => setDay(item.id)}
            className="min-h-11 min-w-11 rounded-full border px-3 py-2 aria-pressed:bg-primary aria-pressed:text-primary-foreground"
          >
            {item.label}
          </button>
        ))}
      </div>
      {!locked && (
        <div aria-label="Presets de horario" className="flex flex-wrap gap-2">
          {PRESETS.map((preset) => (
            <button
              key={preset.id}
              type="button"
              disabled={disabled}
              onClick={() => commit(applyWeeklyPreset(slots, day, preset.id))}
              className="min-h-11 min-w-11 rounded-md border px-3 py-2"
            >
              {preset.label}
            </button>
          ))}
        </div>
      )}
      <div className="grid grid-cols-3 gap-2 sm:grid-cols-6">
        {Array.from({ length: 24 }, (_, hour) => (
          <button
            key={hour}
            type="button"
            aria-pressed={
              weeklyHourCoverage(daySlots, day, hour) === "partial"
                ? "mixed"
                : weeklyHourCoverage(daySlots, day, hour) === "full"
            }
            disabled={locked}
            onClick={() =>
              commit(toggleWeeklyHour(slots, day, hour, !selected(hour)))
            }
            className="min-h-11 min-w-11 rounded-md border px-2 py-2 text-sm aria-pressed:bg-primary aria-pressed:text-primary-foreground"
          >
            {String(hour).padStart(2, "0")}:00–
            {String(hour + 1).padStart(2, "0")}:00
            {weeklyHourCoverage(daySlots, day, hour) === "partial" && (
              <>
                {" "}
                <span className="mt-1 block rounded bg-primary/10 px-1 text-xs font-medium text-primary">
                  Parcial
                </span>
                <span className="sr-only">
                  {" "}
                  Tramos:{" "}
                  {daySlots
                    .filter(
                      (slot) =>
                        weeklyHourCoverage([slot], day, hour) !== "none",
                    )
                    .map((slot) => `${slot.start_time}–${slot.end_time}`)
                    .join(", ")}
                </span>
              </>
            )}
          </button>
        ))}
      </div>
      {daySlots.map((slot, index) => (
        <div key={index} className="grid grid-cols-2 gap-3">
          <label className="text-sm">
            Inicio
            <input
              aria-label={`Tramo ${index + 1}: hora de inicio`}
              aria-invalid={invalid || undefined}
              aria-describedby={invalid ? errorId : undefined}
              type="time"
              step={60}
              value={slot.start_time}
              disabled={locked}
              onChange={(event) => {
                if (!onChange) return;
                const next = [...slots];
                next[slots.indexOf(slot)] = {
                  ...slot,
                  start_time: event.target.value,
                };
                onChange(next);
              }}
              className="mt-1 block w-full rounded-md border p-2"
            />
          </label>
          <label className="text-sm">
            Cierre (incluye 24:00)
            <input
              aria-label={`Tramo ${index + 1}: hora de cierre (24:00 permitido)`}
              aria-invalid={invalid || undefined}
              aria-describedby={invalid ? errorId : undefined}
              type="text"
              inputMode="numeric"
              pattern="(?:[01][0-9]|2[0-3]):[0-5][0-9]|24:00"
              value={slot.end_time}
              disabled={locked}
              onChange={(event) => {
                if (!onChange) return;
                const next = [...slots];
                next[slots.indexOf(slot)] = {
                  ...slot,
                  end_time: event.target.value,
                };
                onChange(next);
              }}
              className="mt-1 block w-full rounded-md border p-2"
            />
          </label>
        </div>
      ))}
      <div
        role="alert"
        aria-live="polite"
        className="min-h-5 text-sm text-red-800"
      >
        {error}
      </div>
    </section>
  );
}
