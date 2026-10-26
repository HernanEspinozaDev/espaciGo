import { clpAmount, formatClp } from "@/lib/api/clp";
import type { PublicSpaceCategory } from "./types";

export function formatPublicClp(value: number | string | undefined | null) {
  if (value === undefined || value === null) return null;
  try {
    return formatClp(clpAmount(value));
  } catch {
    return null;
  }
}

export function categoryLabel(category: PublicSpaceCategory): string {
  return category.code === "inmuebles"
    ? "Arriendos residenciales"
    : category.name;
}

export function rateUnitLabel(unit: "hora" | "dia" | "mes") {
  return unit === "hora" ? "hora" : unit === "dia" ? "día" : "mes";
}

export function safePublicText(value: string): string {
  return value
    .replace(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/gi, "[contacto omitido]")
    .replace(
      /(?:\+?56[\s.-]?)?9[\s.-]?\d{4}[\s.-]?\d{4}\b/g,
      "[contacto omitido]",
    )
    .replace(
      /\b(?:calle|av(?:enida)?|pasaje|camino|ruta)\s+[^,;\n]{1,70}?\s+\d{1,6}\b/gi,
      "[dirección omitida]",
    );
}
