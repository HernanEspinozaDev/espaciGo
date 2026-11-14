export interface MonthDay {
  date: string;
  weekday: number;
  inMonth: boolean;
}

function parseMonth(month: string): { year: number; monthIndex: number } {
  const match = /^(\d{4})-(0[1-9]|1[0-2])$/.exec(month);
  if (!match) throw new RangeError("Mes inválido; se esperaba YYYY-MM");
  const year = Number(match[1]);
  if (year < 1 || year > 9999) throw new RangeError("Año fuera de rango");
  return { year, monthIndex: Number(match[2]) - 1 };
}

function utcCivilDate(year: number, monthIndex: number, day: number): Date {
  const date = new Date(0);
  date.setUTCHours(0, 0, 0, 0);
  date.setUTCFullYear(year, monthIndex, day);
  return date;
}

function isoDate(date: Date): string {
  const year = String(date.getUTCFullYear()).padStart(4, "0");
  const month = String(date.getUTCMonth() + 1).padStart(2, "0");
  const day = String(date.getUTCDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function monthDays(month: string): MonthDay[] {
  const { year, monthIndex } = parseMonth(month);
  const first = utcCivilDate(year, monthIndex, 1);
  const mondayOffset = (first.getUTCDay() + 6) % 7;
  const count = utcCivilDate(year, monthIndex + 1, 0).getUTCDate();
  const length = Math.ceil((mondayOffset + count) / 7) * 7;
  return Array.from({ length }, (_, index) => {
    const date = utcCivilDate(year, monthIndex, index - mondayOffset + 1);
    return { date: isoDate(date), weekday: index % 7, inMonth: date.getUTCMonth() === monthIndex && date.getUTCFullYear() === year };
  });
}

export function shiftMonth(month: string, delta: number): string | null {
  const { year, monthIndex } = parseMonth(month);
  if (!Number.isInteger(delta)) throw new RangeError("El desplazamiento debe ser entero");
  const ordinal = year * 12 + monthIndex + delta;
  if (ordinal < 12 || ordinal > 9999 * 12 + 11) return null;
  const nextYear = Math.floor(ordinal / 12);
  const nextMonth = (ordinal % 12) + 1;
  return `${String(nextYear).padStart(4, "0")}-${String(nextMonth).padStart(2, "0")}`;
}

export function localDateLabel(date: string): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) throw new RangeError("Fecha inválida");
  const [year, month, day] = date.split("-").map(Number);
  const civil = utcCivilDate(year, month - 1, day);
  if (isoDate(civil) !== date) throw new RangeError("Fecha inválida");
  return new Intl.DateTimeFormat("es", { weekday: "long", day: "numeric", month: "long", year: "numeric", timeZone: "UTC" }).format(civil);
}
