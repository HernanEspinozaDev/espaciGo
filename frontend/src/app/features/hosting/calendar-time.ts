export type LocalTimeDisambiguation = 'earlier' | 'later';

/** Convert a wall time into RFC3339 using the selected IANA zone, rejecting DST gaps. */
export function zonedLocalToRfc3339(
  local: string,
  timeZone: string,
  disambiguation: LocalTimeDisambiguation = 'earlier',
): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(local);
  if (!match) throw new Error('Ingresa una fecha y hora local válidas.');
  const target = match.slice(1).map(Number);
  const [year, month, day, hour, minute] = target;
  const naiveUtc = Date.UTC(year, month - 1, day, hour, minute);
  const formatter = new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  });
  const partsAt = (instant: number) => {
    const parts = formatter.formatToParts(new Date(instant));
    const values = Object.fromEntries(parts.map((part) => [part.type, part.value]));
    return [
      Number(values['year']),
      Number(values['month']),
      Number(values['day']),
      Number(values['hour']),
      Number(values['minute']),
    ];
  };
  const offsets = new Set<number>();
  for (let delta = -36; delta <= 36; delta += 3) {
    const sample = naiveUtc + delta * 60 * 60 * 1000;
    const sampleParts = partsAt(sample);
    const sampleAsUtc = Date.UTC(
      sampleParts[0],
      sampleParts[1] - 1,
      sampleParts[2],
      sampleParts[3],
      sampleParts[4],
    );
    offsets.add(sampleAsUtc - sample);
  }
  const candidates = [...offsets]
    .map((offset) => ({ instant: naiveUtc - offset, offset }))
    .filter(({ instant }) => partsAt(instant).every((part, index) => part === target[index]))
    .sort((a, b) => a.instant - b.instant);
  if (!candidates.length) {
    throw new Error('Esa hora local no existe por el cambio de horario de la zona elegida.');
  }
  const selected = disambiguation === 'later' ? candidates[candidates.length - 1] : candidates[0];
  const offsetMinutes = selected.offset / 60_000;
  const sign = offsetMinutes >= 0 ? '+' : '-';
  const absolute = Math.abs(offsetMinutes);
  const offset = `${sign}${String(Math.floor(absolute / 60)).padStart(2, '0')}:${String(absolute % 60).padStart(2, '0')}`;
  return `${local}:00${offset}`;
}

export function zonedIsoLabel(value: string, timeZone: string): string {
  return new Intl.DateTimeFormat('es-CL', {
    dateStyle: 'medium',
    timeStyle: 'short',
    timeZone,
    timeZoneName: 'short',
  }).format(new Date(value));
}
