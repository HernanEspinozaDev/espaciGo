import type { ScheduleSlot } from "./types";
import { normalizeDayOfWeek, validateScheduleSlots } from "./validation";
const timeToMinutes = (time: string) => {
  const [hours, minutes] = time.split(":").map(Number);
  return hours * 60 + minutes;
};

export type WeeklyPreset = "morning" | "afternoon" | "full-day" | "24h";
export interface SlotEditResult {
  slots: ScheduleSlot[];
  error: string | null;
}
export type WeeklyHourCoverage = "none" | "partial" | "full";

export function weeklyHourCoverage(
  slots: readonly ScheduleSlot[],
  day: number,
  hour: number,
): WeeklyHourCoverage {
  let isoDay: number;
  try {
    isoDay = normalizeDayOfWeek(day);
  } catch {
    return "none";
  }
  if (!Number.isInteger(hour) || hour < 0 || hour > 23) return "none";
  const start = hour * 60;
  const end = start + 60;
  const intervals: Array<[number, number]> = [];
  for (const slot of slots) {
    let slotDay: number;
    try {
      slotDay = normalizeDayOfWeek(slot.day_of_week);
    } catch {
      continue;
    }
    if (slotDay !== isoDay) continue;
    const [sh, sm] = slot.start_time.split(":").map(Number);
    const [eh, em] = slot.end_time.split(":").map(Number);
    const clippedStart = Math.max(start, sh * 60 + sm);
    const clippedEnd = Math.min(end, eh * 60 + em);
    if (clippedStart < clippedEnd) intervals.push([clippedStart, clippedEnd]);
  }
  intervals.sort((a, b) => a[0] - b[0]);
  let coveredUntil = start;
  let hasCoverage = false;
  for (const [from, to] of intervals) {
    hasCoverage = true;
    if (from > coveredUntil) break;
    coveredUntil = Math.max(coveredUntil, to);
    if (coveredUntil >= end) return "full";
  }
  return hasCoverage ? "partial" : "none";
}

const LIMIT_MESSAGE = "Cada día admite como máximo dos tramos horarios.";
const formatTime = (minutes: number) =>
  minutes === 1440
    ? "24:00"
    : `${String(Math.floor(minutes / 60)).padStart(2, "0")}:${String(minutes % 60).padStart(2, "0")}`;

function validate(slots: ScheduleSlot[]): SlotEditResult {
  const result = validateScheduleSlots(slots);
  return result.valid
    ? { slots, error: null }
    : { slots: [], error: result.errors[0] ?? "Horario inválido." };
}

export function toggleWeeklyHour(
  current: readonly ScheduleSlot[],
  day: number,
  hour: number,
  selected: boolean,
): SlotEditResult {
  let isoDay: number;
  try {
    isoDay = normalizeDayOfWeek(day);
  } catch {
    return { slots: [...current], error: "Día de semana inválido." };
  }
  if (!Number.isInteger(hour) || hour < 0 || hour > 23)
    return { slots: [...current], error: "Hora inválida." };

  const untouched = current.filter((slot) => {
    try {
      return normalizeDayOfWeek(slot.day_of_week) !== isoDay;
    } catch {
      return true;
    }
  });
  const target = current.filter((slot) => {
    try {
      return normalizeDayOfWeek(slot.day_of_week) === isoDay;
    } catch {
      return false;
    }
  });
  const currentValidation = validateScheduleSlots(current);
  if (!currentValidation.valid) {
    return {
      slots: [...current],
      error: currentValidation.errors[0] ?? "Horario inválido.",
    };
  }
  const minute = hour * 60;
  const intervals: Array<[number, number]> = [];
  for (const slot of target) {
    const start = timeToMinutes(slot.start_time),
      end = timeToMinutes(slot.end_time);
    if (selected) intervals.push([start, end]);
    else {
      if (start < minute) intervals.push([start, Math.min(end, minute)]);
      if (end > minute + 60)
        intervals.push([Math.max(start, minute + 60), end]);
    }
  }
  if (selected) intervals.push([minute, minute + 60]);
  intervals.sort((a, b) => a[0] - b[0]);
  const merged: Array<[number, number]> = [];
  for (const item of intervals) {
    const last = merged.at(-1);
    if (last && item[0] <= last[1]) last[1] = Math.max(last[1], item[1]);
    else merged.push([...item]);
  }
  const updated = [
    ...untouched,
    ...merged.map(([start, end]) => ({
      day_of_week: isoDay,
      start_time: formatTime(start),
      end_time: formatTime(end),
    })),
  ];
  if (merged.length > 2) return { slots: [...current], error: LIMIT_MESSAGE };
  const checked = validate(updated);
  return checked.error
    ? { slots: [...current], error: checked.error }
    : checked;
}

export function applyWeeklyPreset(
  current: readonly ScheduleSlot[],
  day: number,
  preset: WeeklyPreset,
): SlotEditResult {
  let isoDay: number;
  try {
    isoDay = normalizeDayOfWeek(day);
  } catch {
    return { slots: [...current], error: "Día de semana inválido." };
  }
  const interval: Record<WeeklyPreset, [string, string]> = {
    morning: ["09:00", "13:00"],
    afternoon: ["14:00", "18:00"],
    "full-day": ["09:00", "18:00"],
    "24h": ["00:00", "24:00"],
  };
  const next = [
    ...current.filter((slot) => {
      try {
        return normalizeDayOfWeek(slot.day_of_week) !== isoDay;
      } catch {
        return true;
      }
    }),
    {
      day_of_week: isoDay,
      start_time: interval[preset][0],
      end_time: interval[preset][1],
    },
  ];
  const checked = validate(next);
  return checked.error
    ? { slots: [...current], error: checked.error }
    : checked;
}
