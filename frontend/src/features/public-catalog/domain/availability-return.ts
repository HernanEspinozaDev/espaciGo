export interface RentalReturnSelection {
  date: string;
  duration: number;
  startAt: string;
  endAt: string;
}

const selectionKeys = [
  "rental_date",
  "rental_duration",
  "rental_start_at",
  "rental_end_at",
] as const;
const civilDate = /^\d{4}-\d{2}-\d{2}$/;
const timestamp =
  /^(\d{4}-\d{2}-\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(Z|[+-]\d{2}:\d{2})$/;

function isCivilDate(value: string): boolean {
  if (!civilDate.test(value)) return false;
  const parsed = new Date(`${value}T00:00:00.000Z`);
  return (
    Number.isFinite(parsed.getTime()) &&
    parsed.toISOString().slice(0, 10) === value
  );
}

function timestampMilliseconds(value: string): number | null {
  const parts = timestamp.exec(value);
  if (!parts || !isCivilDate(parts[1] ?? "")) return null;
  const hour = Number(parts[2]);
  const minute = Number(parts[3]);
  const second = Number(parts[4]);
  const offset = parts[5] ?? "";
  if (hour > 23 || minute > 59 || second > 59) return null;
  if (offset !== "Z") {
    const offsetHour = Number(offset.slice(1, 3));
    const offsetMinute = Number(offset.slice(4, 6));
    if (offsetHour > 23 || offsetMinute > 59) return null;
  }
  const milliseconds = Date.parse(value);
  return Number.isFinite(milliseconds) ? milliseconds : null;
}

export function parseRentalReturnSelection(
  search: string,
): RentalReturnSelection | null {
  const raw = search.startsWith("?") ? search.slice(1) : search;
  if (!raw || raw.length > 1024) return null;
  const params = new URLSearchParams(raw);
  if (
    params.size !== selectionKeys.length ||
    selectionKeys.some((key) => params.getAll(key).length !== 1) ||
    [...params.keys()].some(
      (key) => !selectionKeys.includes(key as (typeof selectionKeys)[number]),
    )
  )
    return null;

  const date = params.get("rental_date") ?? "";
  const rawDuration = params.get("rental_duration") ?? "";
  const startAt = params.get("rental_start_at") ?? "";
  const endAt = params.get("rental_end_at") ?? "";
  const start = timestampMilliseconds(startAt);
  const end = timestampMilliseconds(endAt);
  if (
    !isCivilDate(date) ||
    !/^[1-4]$/.test(rawDuration) ||
    start === null ||
    end === null ||
    end <= start
  )
    return null;
  return { date, duration: Number(rawDuration), startAt, endAt };
}

export function serializeRentalReturnSelection(
  selection: RentalReturnSelection,
): string {
  const params = new URLSearchParams();
  params.set("rental_date", selection.date);
  params.set("rental_duration", String(selection.duration));
  params.set("rental_start_at", selection.startAt);
  params.set("rental_end_at", selection.endAt);
  return params.toString();
}

export function publicSpaceLoginHref(
  spaceId: string,
  selection?: RentalReturnSelection,
): string {
  const serialized = selection ? serializeRentalReturnSelection(selection) : "";
  const query =
    serialized && parseRentalReturnSelection(serialized)
      ? `?${serialized}`
      : "";
  const returnPath = `/espacios/${encodeURIComponent(spaceId)}${query}`;
  return `/acceso?return_to=${encodeURIComponent(returnPath)}`;
}
