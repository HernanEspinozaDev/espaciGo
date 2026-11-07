"use client";

import { publicWeeklyScheduleSlots } from "@/features/public-catalog/domain/weekly-schedule";
import type { components } from "@/lib/api/generated/openapi";
import { WeeklyScheduleSlots } from "@/features/host-listings/ui/weekly-schedule-slots";

type PublicWeeklyScheduleDto = components["schemas"]["PublicWeeklySchedule"];

export function PublicWeeklySchedule({
  schedule,
}: {
  schedule?: PublicWeeklyScheduleDto;
}) {
  if (!schedule) {
    return (
      <section aria-labelledby="public-weekly-schedule-title">
        <h2 id="public-weekly-schedule-title" className="text-lg font-medium">
          Horarios de la publicación
        </h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Horario no informado.
        </p>
      </section>
    );
  }

  return (
    <section
      aria-labelledby="public-weekly-schedule-title"
      className="space-y-3"
    >
      <h2 id="public-weekly-schedule-title" className="text-lg font-medium">
        Horarios de la publicación
      </h2>
      {!schedule.enabled ? (
        <p className="text-sm font-medium text-amber-800">Horario inactivo.</p>
      ) : null}
      <WeeklyScheduleSlots
        slots={publicWeeklyScheduleSlots(schedule)}
        timeZone={schedule.time_zone}
        readOnly
      />
      <p className="text-sm text-muted-foreground">
        Horario habitual orientativo; no confirma disponibilidad para fechas
        específicas ni permite reservar.
      </p>
    </section>
  );
}
