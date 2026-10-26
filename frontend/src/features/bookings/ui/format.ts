import { clpAmount, formatClp } from "@/lib/api/clp";

export function formatBookingClp(
  value: number | string | null | undefined,
): string {
  if (value === null || value === undefined) return "$0";
  try {
    const num =
      typeof value === "string" ? parseInt(value, 10) : Math.round(value);
    if (isNaN(num)) return "$0";
    return formatClp(clpAmount(num));
  } catch {
    return `$${value}`;
  }
}
