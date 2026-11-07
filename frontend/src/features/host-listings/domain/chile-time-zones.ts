export const CHILE_TIME_ZONE_OPTIONS = [
  {
    value: "America/Santiago",
    label: "Chile continental",
  },
  {
    value: "America/Coyhaique",
    label: "Región de Aysén",
  },
  {
    value: "America/Punta_Arenas",
    label: "Región de Magallanes",
  },
  {
    value: "Pacific/Easter",
    label: "Rapa Nui / Isla de Pascua",
  },
] as const;

export function timeZoneOptionsForCurrentValue(
  current: string | null | undefined,
) {
  if (
    !current ||
    CHILE_TIME_ZONE_OPTIONS.some((option) => option.value === current)
  ) {
    return CHILE_TIME_ZONE_OPTIONS;
  }

  return [
    ...CHILE_TIME_ZONE_OPTIONS,
    { value: current, label: "Zona horaria guardada previamente" },
  ];
}
