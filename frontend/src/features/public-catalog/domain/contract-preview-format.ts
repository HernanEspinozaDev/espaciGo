export function formatRentalCLP(value: number | string): string {
  const digits =
    typeof value === "number"
      ? Number.isSafeInteger(value) && value >= 0
        ? String(value)
        : null
      : /^\d+$/.test(value)
        ? value
        : null;

  if (digits === null) return "Monto no disponible";
  const normalized = digits.replace(/^0+(?=\d)/, "");
  const grouped = normalized.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  return `$${grouped} CLP`;
}
