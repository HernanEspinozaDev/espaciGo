import type { components } from "@/lib/api/generated/openapi";
import type { ScheduleSlot } from "@/features/host-listings/domain/types";

type PublicWeeklySchedule = components["schemas"]["PublicWeeklySchedule"];

/** Convert the public ISO weekday DTO to the editor's display-only slot shape. */
export function publicWeeklyScheduleSlots(
  schedule: PublicWeeklySchedule,
): ScheduleSlot[] {
  return schedule.days.flatMap(({ weekday, periods }) =>
    periods.map(({ open, close }) => ({
      day_of_week: weekday,
      start_time: open,
      end_time: close,
    })),
  );
}
