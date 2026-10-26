export type SortKey = "newest" | "price-asc" | "price-desc" | "name";

export const SORT_OPTIONS: { value: SortKey; label: string }[] = [
  { value: "newest", label: "Destacados" },
  { value: "price-asc", label: "Menor tarifa" },
  { value: "price-desc", label: "Mayor tarifa" },
  { value: "name", label: "Nombre" },
];

export function getSortFromParams(value: string | undefined): SortKey {
  return SORT_OPTIONS.some((option) => option.value === value)
    ? (value as SortKey)
    : "newest";
}
