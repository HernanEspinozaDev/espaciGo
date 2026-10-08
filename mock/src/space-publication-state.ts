export interface PublicationAction {
  label: "Publicar" | "Ocultar";
  nextState: "activa" | "oculta";
}

export function publicationAction(state: string, roles: readonly string[]): PublicationAction | null {
  if (!roles.includes("arrendador")) return null;
  if (state === "borrador" || state === "oculta") return { label: "Publicar", nextState: "activa" };
  if (state === "activa") return { label: "Ocultar", nextState: "oculta" };
  return null;
}
