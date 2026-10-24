export function parseRfc3339(value: string): Date {
  if (
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(
      value,
    )
  ) {
    throw new TypeError("Timestamp must be RFC 3339 with an explicit offset");
  }
  const timestamp = new Date(value);
  if (!Number.isFinite(timestamp.getTime())) {
    throw new TypeError("Timestamp is invalid");
  }
  return timestamp;
}

export function assertIanaTimeZone(timeZone: string): string {
  try {
    new Intl.DateTimeFormat("en-US", { timeZone }).format(0);
  } catch {
    throw new RangeError("Time zone is not a supported IANA identifier");
  }
  return timeZone;
}

export function formatInTimeZone(value: string, timeZone: string): string {
  const timestamp = parseRfc3339(value);
  return new Intl.DateTimeFormat("es-CL", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: assertIanaTimeZone(timeZone),
  }).format(timestamp);
}
