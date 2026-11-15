export function localDateISO(value: Date, timeZone: string): string {
  const parts = new Intl.DateTimeFormat("en", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    timeZone,
  }).formatToParts(value);
  const year = parts.find((part) => part.type === "year")?.value;
  const month = parts.find((part) => part.type === "month")?.value;
  const day = parts.find((part) => part.type === "day")?.value;

  if (!year || !month || !day) {
    throw new RangeError("No fue posible obtener la fecha civil local");
  }
  return `${year.padStart(4, "0")}-${month}-${day}`;
}

export function addCivilDays(date: string, days: number): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || !Number.isSafeInteger(days)) {
    throw new RangeError("La fecha civil o el desplazamiento no es válido");
  }
  const value = new Date(`${date}T00:00:00.000Z`);
  if (
    !Number.isFinite(value.getTime()) ||
    value.toISOString().slice(0, 10) !== date
  ) {
    throw new RangeError("La fecha civil no existe");
  }
  value.setUTCDate(value.getUTCDate() + days);
  if (!Number.isFinite(value.getTime())) {
    throw new RangeError("El desplazamiento excede el rango de fecha válido");
  }
  return value.toISOString().slice(0, 10);
}
