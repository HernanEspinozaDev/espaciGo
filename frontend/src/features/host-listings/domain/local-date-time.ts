const WALL_TIME_PATTERN = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/;

function getParts(formatter: Intl.DateTimeFormat, value: Date) {
  const parts = formatter.formatToParts(value);
  const number = (type: Intl.DateTimeFormatPartTypes) =>
    Number(parts.find((part) => part.type === type)?.value);
  return {
    year: number("year"),
    month: number("month"),
    day: number("day"),
    hour: number("hour"),
    minute: number("minute"),
  };
}

function getOffsetMinutes(formatter: Intl.DateTimeFormat, value: Date): number {
  const offset = formatter
    .formatToParts(value)
    .find((part) => part.type === "timeZoneName")?.value;
  if (offset === "GMT") return 0;
  const match = /^GMT([+-])(\d{2}):(\d{2})$/.exec(offset ?? "");
  if (!match) throw new RangeError("No se pudo resolver la zona horaria.");
  const minutes = Number(match[2]) * 60 + Number(match[3]);
  return match[1] === "+" ? minutes : -minutes;
}

export function localDateTimeToUtcISOString(
  localDateTime: string,
  timeZone: string,
): string {
  const match = WALL_TIME_PATTERN.exec(localDateTime);
  if (!match) throw new RangeError("Ingresa una fecha y hora válidas.");

  const [year, month, day, hour, minute] = match.slice(1).map(Number);
  const wallTime = new Date(0);
  wallTime.setUTCFullYear(year, month - 1, day);
  wallTime.setUTCHours(hour, minute, 0, 0);
  const wallEpoch = wallTime.getTime();
  const expected = { year, month, day, hour, minute };
  const normalized = {
    year: wallTime.getUTCFullYear(),
    month: wallTime.getUTCMonth() + 1,
    day: wallTime.getUTCDate(),
    hour: wallTime.getUTCHours(),
    minute: wallTime.getUTCMinutes(),
  };
  if (
    Object.keys(expected).some(
      (key) =>
        expected[key as keyof typeof expected] !==
        normalized[key as keyof typeof normalized],
    )
  ) {
    throw new RangeError("Ingresa una fecha y hora válidas.");
  }

  const formatter = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
    timeZoneName: "longOffset",
  });
  const offsets = new Set<number>();
  for (let hours = -24; hours <= 24; hours += 1) {
    offsets.add(
      getOffsetMinutes(formatter, new Date(wallEpoch + hours * 3600000)),
    );
  }

  const candidates = [...offsets]
    .map((offset) => wallEpoch - offset * 60000)
    .filter((candidate) => {
      const actual = getParts(formatter, new Date(candidate));
      return Object.keys(expected).every(
        (key) =>
          expected[key as keyof typeof expected] ===
          actual[key as keyof typeof expected],
      );
    });

  if (candidates.length !== 1) {
    throw new RangeError(
      "La hora local es inexistente o ambigua en la zona horaria del espacio.",
    );
  }
  return new Date(candidates[0]).toISOString().replace(".000Z", "Z");
}
